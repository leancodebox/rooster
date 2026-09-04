package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/leancodebox/rooster/internal/domain"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Result struct {
	StartedAt  time.Time
	FinishedAt time.Time
	ExitCode   int
	Err        error
}

type Handle struct {
	PID    int
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
	mu     sync.RWMutex
	result Result
}

func (h *Handle) Stop()                 { h.once.Do(h.cancel) }
func (h *Handle) Done() <-chan struct{} { return h.done }
func (h *Handle) Result() Result        { h.mu.RLock(); defer h.mu.RUnlock(); return h.result }
func (h *Handle) Wait() Result          { <-h.done; return h.Result() }

type Runner struct {
	environment *EnvironmentProvider
	stopGrace   time.Duration
}

func New(environment *EnvironmentProvider) *Runner {
	return &Runner{environment: environment, stopGrace: 3 * time.Second}
}

func (r *Runner) Start(parent context.Context, task domain.Task, logPath string) (*Handle, error) {
	if err := task.Validate(); err != nil {
		return nil, err
	}
	if task.WorkingDir != "" {
		info, err := os.Stat(task.WorkingDir)
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("working directory is unavailable: %s", task.WorkingDir)
		}
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	logFile := &lumberjack.Logger{Filename: logPath, MaxSize: 20, MaxBackups: 3, MaxAge: 14, Compress: true}
	cmd, err := buildCommand(task)
	if err != nil {
		logFile.Close()
		return nil, err
	}
	cmd.Dir = task.WorkingDir
	cmd.Env = r.environment.Resolve(task.Environment)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	configureProcess(cmd)
	started := time.Now().UTC()
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("start command: %w", err)
	}
	ctx, cancel := context.WithCancel(parent)
	h := &Handle{PID: cmd.Process.Pid, cancel: cancel, done: make(chan struct{})}
	waitDone := make(chan Result, 1)
	go func() {
		err := cmd.Wait()
		result := Result{StartedAt: started, FinishedAt: time.Now().UTC(), ExitCode: -1, Err: err}
		if cmd.ProcessState != nil {
			result.ExitCode = cmd.ProcessState.ExitCode()
		}
		_ = logFile.Close()
		waitDone <- result
		close(waitDone)
	}()
	go func() {
		var result Result
		select {
		case result = <-waitDone:
		case <-ctx.Done():
			_ = terminateProcessTree(cmd)
			select {
			case result = <-waitDone:
			case <-time.After(r.stopGrace):
				_ = killProcessTree(cmd)
				result = <-waitDone
			}
			result.Err = context.Canceled
		}
		cancel()
		h.mu.Lock()
		h.result = result
		h.mu.Unlock()
		close(h.done)
	}()
	return h, nil
}

func buildCommand(task domain.Task) (*exec.Cmd, error) {
	if task.CommandMode == domain.CommandModeExec {
		return exec.Command(task.Command, task.Arguments...), nil
	}
	if runtime.GOOS == "windows" {
		return exec.Command("cmd.exe", "/C", task.Command), nil
	}
	shell := task.Shell
	if shell == "" {
		shell = defaultShell()
	}
	if shell == "" {
		return nil, errors.New("no shell available")
	}
	return exec.Command(shell, "-lc", task.Command), nil
}
