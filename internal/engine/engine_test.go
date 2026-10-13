//go:build !windows

package engine

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/leancodebox/rooster/internal/domain"
	"github.com/leancodebox/rooster/internal/runner"
	storesqlite "github.com/leancodebox/rooster/internal/store/sqlite"
)

func TestManualExecutionCanBeStoppedAndRecorded(t *testing.T) {
	dir := t.TempDir()
	st, err := storesqlite.Open(filepath.Join(dir, "rooster.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	eng := New(st, runner.New(runner.NewEnvironmentProvider()), filepath.Join(dir, "logs"))
	if err := eng.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	task, err := eng.CreateTask(context.Background(), domain.Task{Name: "worker", Kind: domain.TaskKindResident, CommandMode: domain.CommandModeShell, Command: "sleep 30", OverlapPolicy: domain.OverlapSkip, RestartPolicy: domain.RestartNever})
	if err != nil {
		t.Fatal(err)
	}
	execution, err := eng.Run(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.StopExecution(execution.ID); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		items, err := eng.ListExecutions(context.Background(), task.ID, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) > 0 && items[0].Status == domain.ExecutionCanceled {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	items, err := eng.ListExecutions(context.Background(), task.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Status != domain.ExecutionCanceled {
		t.Fatalf("unexpected executions: %#v", items)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := eng.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestRejectsInvalidCronBeforePersistence(t *testing.T) {
	dir := t.TempDir()
	st, err := storesqlite.Open(filepath.Join(dir, "rooster.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	eng := New(st, runner.New(runner.NewEnvironmentProvider()), filepath.Join(dir, "logs"))
	_, err = eng.CreateTask(context.Background(), domain.Task{Name: "bad schedule", Kind: domain.TaskKindScheduled, CommandMode: domain.CommandModeShell, Command: "echo hi", Schedule: "not cron", OverlapPolicy: domain.OverlapSkip, RestartPolicy: domain.RestartNever})
	if err == nil {
		t.Fatal("expected cron validation error")
	}
	tasks, err := st.ListTasks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Fatalf("invalid task was persisted: %#v", tasks)
	}
}

func TestStoppingResidentDisablesAutomaticRestart(t *testing.T) {
	dir := t.TempDir()
	st, err := storesqlite.Open(filepath.Join(dir, "rooster.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	eng := New(st, runner.New(runner.NewEnvironmentProvider()), filepath.Join(dir, "logs"))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := eng.Close(ctx); err != nil {
			t.Error(err)
		}
	}()
	task, err := eng.CreateTask(context.Background(), domain.Task{Name: "worker", Kind: domain.TaskKindResident, Enabled: true, CommandMode: domain.CommandModeShell, Command: "sleep 30", OverlapPolicy: domain.OverlapSkip, RestartPolicy: domain.RestartAlways})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	var executionID string
	for time.Now().Before(deadline) {
		views, err := eng.ListTasks(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(views) == 1 && views[0].Runtime.ExecutionID != "" {
			executionID = views[0].Runtime.ExecutionID
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if executionID == "" {
		t.Fatal("resident task did not start")
	}
	if err := eng.StopExecution(executionID); err != nil {
		t.Fatal(err)
	}
	saved, err := st.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Enabled {
		t.Fatal("manually stopped resident remains enabled")
	}
	time.Sleep(1500 * time.Millisecond)
	items, err := eng.ListExecutions(context.Background(), task.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Status != domain.ExecutionCanceled {
		t.Fatalf("expected one canceled execution without restart: %#v", items)
	}
}

func TestConcurrentRunsRespectSkipOverlap(t *testing.T) {
	dir := t.TempDir()
	st, err := storesqlite.Open(filepath.Join(dir, "rooster.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	eng := New(st, runner.New(runner.NewEnvironmentProvider()), filepath.Join(dir, "logs"))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := eng.Close(ctx); err != nil {
			t.Error(err)
		}
	}()
	task, err := eng.CreateTask(context.Background(), domain.Task{Name: "worker", Kind: domain.TaskKindResident, CommandMode: domain.CommandModeShell, Command: "sleep 30", OverlapPolicy: domain.OverlapSkip, RestartPolicy: domain.RestartNever})
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 12)
	var workers sync.WaitGroup
	for range 12 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			_, err := eng.Run(context.Background(), task.ID)
			results <- err
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrAlreadyRunning) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful concurrent starts = %d, want 1", successes)
	}
}

func TestStartupFailureRetriesAndDisablesTask(t *testing.T) {
	for _, command := range []string{"missing", "short"} {
		t.Run(command, func(t *testing.T) { testFailureRetriesAndDisablesTask(t, command) })
	}
}

func testFailureRetriesAndDisablesTask(t *testing.T, command string) {
	dir := t.TempDir()
	st, err := storesqlite.Open(filepath.Join(dir, "rooster.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	eng := New(st, runner.New(runner.NewEnvironmentProvider()), filepath.Join(dir, "logs"))
	defer eng.Close(context.Background())
	task := domain.Task{Name: "missing", Kind: domain.TaskKindResident, Enabled: true, CommandMode: domain.CommandModeExec, Command: filepath.Join(dir, "does-not-exist"), OverlapPolicy: domain.OverlapSkip, RestartPolicy: domain.RestartOnFailure, MaxRetries: 2}
	if command == "short" {
		task.CommandMode = domain.CommandModeShell
		task.Command = "exit 1"
	}
	_, err = eng.CreateTask(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		views, err := eng.ListTasks(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(views) == 1 && views[0].Runtime.State == "failed" {
			if views[0].Enabled || views[0].Runtime.RetryCount != 2 || views[0].Runtime.Error == "" {
				t.Fatalf("unexpected exhausted state: %#v", views[0])
			}
			items, err := eng.ListExecutions(context.Background(), views[0].ID, 10)
			if err != nil || len(items) != 2 {
				t.Fatalf("expected two attempts: %v, %v", items, err)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("startup failure did not reach bounded retry exhaustion")
}

func TestTimeoutReleasesConcurrencySlot(t *testing.T) {
	dir := t.TempDir()
	st, err := storesqlite.Open(filepath.Join(dir, "rooster.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	eng := New(st, runner.New(runner.NewEnvironmentProvider()), filepath.Join(dir, "logs"), Limits{MaxConcurrent: 1, ExecutionTimeout: 300 * time.Millisecond})
	defer eng.Close(context.Background())
	task, err := eng.CreateTask(context.Background(), domain.Task{Name: "timeout", Kind: domain.TaskKindScheduled, Schedule: "0 * * * *", CommandMode: domain.CommandModeShell, Command: "sleep 30", OverlapPolicy: domain.OverlapParallel, RestartPolicy: domain.RestartNever})
	if err != nil {
		t.Fatal(err)
	}
	first, err := eng.Run(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.Run(context.Background(), task.ID); !errors.Is(err, ErrConcurrencyLimit) {
		t.Fatalf("expected concurrency rejection, got %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		execution, err := eng.GetExecution(context.Background(), first.ID)
		if err != nil {
			t.Fatal(err)
		}
		if execution.Status == domain.ExecutionFailed {
			if execution.Error != context.DeadlineExceeded.Error() {
				t.Fatalf("timeout reason = %q", execution.Error)
			}
			eng.mu.RLock()
			slots := eng.slots
			eng.mu.RUnlock()
			if slots == 0 {
				if _, err := eng.Run(context.Background(), task.ID); err != nil {
					t.Fatal(err)
				}
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out execution did not release its slot")
}

func TestPerTaskCapacityDoesNotBlockOtherTasks(t *testing.T) {
	dir := t.TempDir()
	st, err := storesqlite.Open(filepath.Join(dir, "rooster.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	eng := New(st, runner.New(runner.NewEnvironmentProvider()), filepath.Join(dir, "logs"), Limits{MaxConcurrent: 3, MaxPerTask: 1})
	defer eng.Close(context.Background())
	create := func(name string) domain.Task {
		t.Helper()
		task, err := eng.CreateTask(context.Background(), domain.Task{Name: name, Kind: domain.TaskKindScheduled, Schedule: "0 * * * *", CommandMode: domain.CommandModeShell, Command: "sleep 30", OverlapPolicy: domain.OverlapParallel, RestartPolicy: domain.RestartNever})
		if err != nil {
			t.Fatal(err)
		}
		return task
	}
	first, second := create("first"), create("second")
	if _, err := eng.Run(context.Background(), first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.Run(context.Background(), first.ID); !errors.Is(err, ErrConcurrencyLimit) {
		t.Fatalf("per-task limit returned %v", err)
	}
	if _, err := eng.Run(context.Background(), second.ID); err != nil {
		t.Fatalf("another task was incorrectly blocked: %v", err)
	}
}

func TestRapidResidentDisableEnableKeepsNewSupervisor(t *testing.T) {
	dir := t.TempDir()
	st, err := storesqlite.Open(filepath.Join(dir, "rooster.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	eng := New(st, runner.New(runner.NewEnvironmentProvider()), filepath.Join(dir, "logs"))
	defer eng.Close(context.Background())
	task, err := eng.CreateTask(context.Background(), domain.Task{Name: "rapid-toggle", Kind: domain.TaskKindResident, Enabled: true, CommandMode: domain.CommandModeShell, Command: "sleep 30", OverlapPolicy: domain.OverlapSkip, RestartPolicy: domain.RestartAlways})
	if err != nil {
		t.Fatal(err)
	}
	previous := ""
	for range 5 {
		deadline := time.Now().Add(5 * time.Second)
		started := false
		for time.Now().Before(deadline) {
			views, err := eng.ListTasks(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(views) == 1 && views[0].Enabled && views[0].Runtime.ExecutionID != "" && views[0].Runtime.ExecutionID != previous {
				previous = views[0].Runtime.ExecutionID
				started = true
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if !started {
			t.Fatal("replacement supervisor did not start")
		}
		if err := eng.SetEnabled(context.Background(), task.ID, false); err != nil {
			t.Fatal(err)
		}
		if err := eng.SetEnabled(context.Background(), task.ID, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := eng.SetEnabled(context.Background(), task.ID, false); err != nil {
		t.Fatal(err)
	}
}
