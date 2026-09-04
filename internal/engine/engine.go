package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/leancodebox/rooster/internal/domain"
	"github.com/leancodebox/rooster/internal/runner"
	"github.com/leancodebox/rooster/internal/store"
	"github.com/robfig/cron/v3"
)

var ErrAlreadyRunning = errors.New("task already has an active execution")

type RuntimeState struct {
	State       string `json:"state"`
	ExecutionID string `json:"executionId,omitempty"`
	PID         int    `json:"pid,omitempty"`
}

type TaskView struct {
	domain.Task
	Runtime RuntimeState `json:"runtime"`
}

type activeExecution struct {
	taskID   string
	handle   *runner.Handle
	mu       sync.Mutex
	finished bool
}

type supervisor struct{ cancel context.CancelFunc }

type Engine struct {
	store  store.Store
	runner *runner.Runner
	logDir string
	ctx    context.Context
	cancel context.CancelFunc

	mu          sync.RWMutex
	scheduleMu  sync.Mutex
	active      map[string]*activeExecution
	byTask      map[string]map[string]struct{}
	runtime     map[string]RuntimeState
	supervisors map[string]supervisor
	scheduler   *cron.Cron
	wg          sync.WaitGroup
	closing     bool
}

func New(st store.Store, processRunner *runner.Runner, logDir string) *Engine {
	ctx, cancel := context.WithCancel(context.Background())
	return &Engine{store: st, runner: processRunner, logDir: logDir, ctx: ctx, cancel: cancel, active: map[string]*activeExecution{}, byTask: map[string]map[string]struct{}{}, runtime: map[string]RuntimeState{}, supervisors: map[string]supervisor{}}
}

func (e *Engine) Start(ctx context.Context) error {
	if err := e.store.MarkInterruptedExecutions(ctx); err != nil {
		return err
	}
	if err := e.reloadSchedules(ctx); err != nil {
		return err
	}
	tasks, err := e.store.ListTasks(ctx)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if task.Enabled && task.Kind == domain.TaskKindResident {
			e.startSupervisor(task)
		}
	}
	return nil
}

func (e *Engine) Close(ctx context.Context) error {
	e.scheduleMu.Lock()
	defer e.scheduleMu.Unlock()
	e.mu.Lock()
	if e.closing {
		e.mu.Unlock()
		return nil
	}
	e.closing = true
	if e.scheduler != nil {
		e.scheduler.Stop()
	}
	for _, s := range e.supervisors {
		s.cancel()
	}
	for _, a := range e.active {
		a.handle.Stop()
	}
	e.cancel()
	e.mu.Unlock()
	done := make(chan struct{})
	go func() { e.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (e *Engine) ListTasks(ctx context.Context) ([]TaskView, error) {
	tasks, err := e.store.ListTasks(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]TaskView, 0, len(tasks))
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, task := range tasks {
		state := e.runtime[task.ID]
		if state.State == "" {
			state = RuntimeState{State: "idle"}
		}
		view := TaskView{Task: task, Runtime: state}
		for id := range e.byTask[task.ID] {
			if a := e.active[id]; a != nil {
				view.Runtime = RuntimeState{State: "running", ExecutionID: id, PID: a.handle.PID}
				break
			}
		}
		views = append(views, view)
	}
	return views, nil
}

func (e *Engine) CreateTask(ctx context.Context, task domain.Task) (domain.Task, error) {
	task.ID = uuid.NewString()
	normalizeTask(&task)
	if err := validateTask(task); err != nil {
		return domain.Task{}, err
	}
	created, err := e.store.CreateTask(ctx, task)
	if err != nil {
		return domain.Task{}, err
	}
	if created.Enabled {
		if err := e.Reconcile(ctx, created.ID); err != nil {
			return domain.Task{}, err
		}
	}
	return created, nil
}

func (e *Engine) UpdateTask(ctx context.Context, task domain.Task) (domain.Task, error) {
	existing, err := e.store.GetTask(ctx, task.ID)
	if err != nil {
		return domain.Task{}, err
	}
	if existing.Enabled {
		return domain.Task{}, errors.New("disable the task before editing it")
	}
	e.mu.RLock()
	active := len(e.byTask[task.ID]) > 0
	e.mu.RUnlock()
	if active {
		return domain.Task{}, errors.New("stop the task before editing it")
	}
	normalizeTask(&task)
	if err := validateTask(task); err != nil {
		return domain.Task{}, err
	}
	updated, err := e.store.UpdateTask(ctx, task)
	if err != nil {
		return domain.Task{}, err
	}
	if err := e.Reconcile(ctx, task.ID); err != nil {
		return domain.Task{}, err
	}
	return updated, nil
}

func (e *Engine) DeleteTask(ctx context.Context, id string) error {
	existing, err := e.store.GetTask(ctx, id)
	if err != nil {
		return err
	}
	if existing.Enabled {
		return errors.New("disable the task before deleting it")
	}
	e.mu.RLock()
	active := len(e.byTask[id]) > 0
	e.mu.RUnlock()
	if active {
		return errors.New("stop the task before deleting it")
	}
	if err := e.store.DeleteTask(ctx, id); err != nil {
		return err
	}
	return e.reloadSchedules(ctx)
}

func (e *Engine) SetEnabled(ctx context.Context, id string, enabled bool) error {
	if enabled {
		task, err := e.store.GetTask(ctx, id)
		if err != nil {
			return err
		}
		if err := validateTask(task); err != nil {
			return err
		}
	}
	if err := e.store.SetTaskEnabled(ctx, id, enabled); err != nil {
		return err
	}
	return e.Reconcile(ctx, id)
}

func (e *Engine) Reconcile(ctx context.Context, id string) error {
	task, err := e.store.GetTask(ctx, id)
	if err != nil {
		return err
	}
	if task.Kind == domain.TaskKindScheduled {
		return e.reloadSchedules(ctx)
	}
	e.mu.Lock()
	current, ok := e.supervisors[id]
	if !task.Enabled && ok {
		delete(e.supervisors, id)
		e.runtime[id] = RuntimeState{State: "stopping"}
		current.cancel()
		for executionID := range e.byTask[id] {
			if a := e.active[executionID]; a != nil {
				a.handle.Stop()
			}
		}
		e.mu.Unlock()
		return nil
	}
	e.mu.Unlock()
	if task.Enabled && !ok {
		e.startSupervisor(task)
	}
	return nil
}

func (e *Engine) Run(ctx context.Context, id string) (domain.Execution, error) {
	task, err := e.store.GetTask(ctx, id)
	if err != nil {
		return domain.Execution{}, err
	}
	return e.dispatch(task, domain.TriggerManual)
}

func (e *Engine) StopExecution(id string) error {
	e.mu.RLock()
	a := e.active[id]
	e.mu.RUnlock()
	if a == nil {
		return store.ErrNotFound
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.finished {
		return store.ErrNotFound
	}
	execution, err := e.store.GetExecution(context.Background(), id)
	if err != nil {
		return err
	}
	execution.Status = domain.ExecutionStopping
	if err := e.store.UpdateExecution(context.Background(), execution); err != nil {
		return err
	}
	e.mu.Lock()
	e.runtime[a.taskID] = RuntimeState{State: "stopping", ExecutionID: id, PID: a.handle.PID}
	e.mu.Unlock()
	a.handle.Stop()
	return nil
}

func (e *Engine) ListExecutions(ctx context.Context, taskID string, limit int) ([]domain.Execution, error) {
	return e.store.ListExecutions(ctx, taskID, limit)
}

func (e *Engine) GetExecution(ctx context.Context, id string) (domain.Execution, error) {
	return e.store.GetExecution(ctx, id)
}

func (e *Engine) dispatch(task domain.Task, trigger domain.Trigger) (domain.Execution, error) {
	e.mu.RLock()
	count := len(e.byTask[task.ID])
	closing := e.closing
	e.mu.RUnlock()
	if closing {
		return domain.Execution{}, errors.New("engine is shutting down")
	}
	if count > 0 && task.OverlapPolicy != domain.OverlapParallel {
		return domain.Execution{}, ErrAlreadyRunning
	}
	id := uuid.NewString()
	logPath := filepath.Join(e.logDir, task.ID, id+".log")
	execution := domain.Execution{ID: id, TaskID: task.ID, Trigger: trigger, Status: domain.ExecutionStarting, LogPath: logPath, CreatedAt: time.Now().UTC()}
	created, err := e.store.CreateExecution(e.ctx, execution)
	if err != nil {
		return domain.Execution{}, err
	}
	handle, err := e.runner.Start(e.ctx, task, logPath)
	if err != nil {
		now := time.Now().UTC()
		created.Status = domain.ExecutionFailed
		created.FinishedAt = &now
		created.Error = err.Error()
		_ = e.store.UpdateExecution(context.Background(), created)
		return created, err
	}
	now := time.Now().UTC()
	created.Status = domain.ExecutionRunning
	created.StartedAt = &now
	created.PID = handle.PID
	if err := e.store.UpdateExecution(e.ctx, created); err != nil {
		handle.Stop()
		handle.Wait()
		return created, err
	}
	e.mu.Lock()
	active := &activeExecution{taskID: task.ID, handle: handle}
	e.active[id] = active
	if e.byTask[task.ID] == nil {
		e.byTask[task.ID] = map[string]struct{}{}
	}
	e.byTask[task.ID][id] = struct{}{}
	e.runtime[task.ID] = RuntimeState{State: "running", ExecutionID: id, PID: handle.PID}
	e.wg.Add(1)
	e.mu.Unlock()
	go e.observe(created, active)
	return created, nil
}

func (e *Engine) observe(execution domain.Execution, active *activeExecution) {
	defer e.wg.Done()
	result := active.handle.Wait()
	active.mu.Lock()
	defer active.mu.Unlock()
	active.finished = true
	execution.FinishedAt = &result.FinishedAt
	execution.ExitCode = &result.ExitCode
	if errors.Is(result.Err, context.Canceled) {
		execution.Status = domain.ExecutionCanceled
	} else if result.ExitCode == 0 {
		execution.Status = domain.ExecutionSucceeded
	} else {
		execution.Status = domain.ExecutionFailed
	}
	if result.Err != nil && !errors.Is(result.Err, context.Canceled) {
		execution.Error = result.Err.Error()
	}
	if err := e.store.UpdateExecution(context.Background(), execution); err != nil {
		slog.Error("update execution", "error", err)
	}
	e.mu.Lock()
	delete(e.active, execution.ID)
	delete(e.byTask[execution.TaskID], execution.ID)
	if len(e.byTask[execution.TaskID]) == 0 {
		delete(e.byTask, execution.TaskID)
		if current := e.runtime[execution.TaskID]; current.State != "backoff" && current.State != "failed" {
			e.runtime[execution.TaskID] = RuntimeState{State: "idle"}
		}
	}
	e.mu.Unlock()
}

func (e *Engine) startSupervisor(task domain.Task) {
	ctx, cancel := context.WithCancel(e.ctx)
	e.mu.Lock()
	if e.closing {
		e.mu.Unlock()
		cancel()
		return
	}
	if _, exists := e.supervisors[task.ID]; exists {
		e.mu.Unlock()
		cancel()
		return
	}
	e.supervisors[task.ID] = supervisor{cancel: cancel}
	e.wg.Add(1)
	e.mu.Unlock()
	go func() {
		defer e.wg.Done()
		defer func() {
			e.mu.Lock()
			delete(e.supervisors, task.ID)
			if e.runtime[task.ID].State != "failed" {
				e.runtime[task.ID] = RuntimeState{State: "idle"}
			}
			e.mu.Unlock()
		}()
		failures := 0
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			e.mu.Lock()
			e.runtime[task.ID] = RuntimeState{State: "starting"}
			e.mu.Unlock()
			execution, err := e.dispatch(task, domain.TriggerSupervisor)
			if err != nil {
				if errors.Is(err, ErrAlreadyRunning) {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				slog.Error("supervisor dispatch", "task", task.ID, "error", err)
				return
			}
			e.mu.RLock()
			active := e.active[execution.ID]
			e.mu.RUnlock()
			if active == nil {
				return
			}
			result := active.handle.Wait()
			duration := result.FinishedAt.Sub(result.StartedAt)
			failed := result.ExitCode != 0 || duration < time.Duration(task.MinRunSeconds)*time.Second
			if task.RestartPolicy == domain.RestartNever || task.RestartPolicy == domain.RestartOnFailure && !failed {
				e.mu.Lock()
				e.runtime[task.ID] = RuntimeState{State: "idle"}
				e.mu.Unlock()
				return
			}
			if failed {
				failures++
			} else {
				failures = 0
			}
			if task.MaxRetries > 0 && failures >= task.MaxRetries {
				_ = e.store.SetTaskEnabled(context.Background(), task.ID, false)
				e.mu.Lock()
				e.runtime[task.ID] = RuntimeState{State: "failed"}
				e.mu.Unlock()
				return
			}
			e.mu.Lock()
			e.runtime[task.ID] = RuntimeState{State: "backoff"}
			e.mu.Unlock()
			delay := time.Second * time.Duration(1<<min(failures, 4))
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()
}

func (e *Engine) reloadSchedules(ctx context.Context) error {
	e.scheduleMu.Lock()
	defer e.scheduleMu.Unlock()
	tasks, err := e.store.ListTasks(ctx)
	if err != nil {
		return err
	}
	c := cron.New()
	for _, task := range tasks {
		if !task.Enabled || task.Kind != domain.TaskKindScheduled {
			continue
		}
		task := task
		if _, err := c.AddFunc(task.Schedule, func() {
			if _, err := e.dispatch(task, domain.TriggerSchedule); err != nil && !errors.Is(err, ErrAlreadyRunning) {
				slog.Error("scheduled dispatch", "task", task.ID, "error", err)
			}
		}); err != nil {
			return fmt.Errorf("schedule %s: %w", task.Name, err)
		}
	}
	e.mu.Lock()
	if e.closing {
		e.mu.Unlock()
		return errors.New("engine is shutting down")
	}
	old := e.scheduler
	e.scheduler = c
	e.mu.Unlock()
	if old != nil {
		old.Stop()
	}
	c.Start()
	return nil
}

func normalizeTask(task *domain.Task) {
	if task.CommandMode == "" {
		task.CommandMode = domain.CommandModeShell
	}
	if task.OverlapPolicy == "" {
		task.OverlapPolicy = domain.OverlapSkip
	}
	if task.RestartPolicy == "" {
		task.RestartPolicy = domain.RestartOnFailure
	}
	if task.MaxRetries == 0 {
		task.MaxRetries = 3
	}
	if task.MinRunSeconds == 0 {
		task.MinRunSeconds = 10
	}
	if task.Arguments == nil {
		task.Arguments = []string{}
	}
	if task.Environment == nil {
		task.Environment = map[string]string{}
	}
}

func validateTask(task domain.Task) error {
	if err := task.Validate(); err != nil {
		return err
	}
	if task.Kind == domain.TaskKindScheduled {
		if _, err := cron.ParseStandard(task.Schedule); err != nil {
			return fmt.Errorf("invalid cron expression: %w", err)
		}
	}
	return nil
}
