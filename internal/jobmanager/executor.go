package jobmanager

import (
	"context"
	"log/slog"
	"time"
)

// ExecutionResult holds the result of a single job execution
type ExecutionResult struct {
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	ExitCode  int
	Error     error
}

// JobExecutor handles the execution of jobs
type JobExecutor struct {
	logManager *LogManager
}

// NewJobExecutor creates a new instance
func NewJobExecutor() *JobExecutor {
	return &JobExecutor{
		logManager: NewLogManager(),
	}
}

// Execute handles the full execution lifecycle of a single run
func (e *JobExecutor) Execute(ctx context.Context, job *Job) ExecutionResult {
	result := ExecutionResult{
		StartTime: time.Now(),
	}

	// 1. Setup Logger
	writer, fullLogPath, err := e.logManager.SetupLogger(job.JobName, job.Options)
	if err != nil {
		slog.Error("SetupLogger failed", "err", err)
	} else {
		job.runtimeLogPath = fullLogPath
		defer func() {
			_ = writer.Close()
		}()
	}

	// 2. Build Command
	// buildCmdWithCtx is defined in cmd_build_*.go
	cmd := buildCmdWithCtx(ctx, job)

	// Configure Graceful Shutdown (Go 1.20+)
	// When context is canceled, we try to kill the process group gracefully first.
	// If it doesn't exit within WaitDelay, the runtime will send a force kill (to the parent process).
	cmd.Cancel = func() error {
		return KillProcessGroup(cmd)
	}
	cmd.WaitDelay = 1 * time.Second

	if writer != nil {
		cmd.Stdout = writer
		cmd.Stderr = writer
	}

	// 3. Update Status (Start)
	job.LastStart = result.StartTime
	job.status = Running

	// 4. Run
	err = cmd.Run()

	// 5. Update Status (End)
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Error = err

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	} else {
		result.ExitCode = -1
	}

	job.LastExit = result.EndTime
	job.LastDuration = result.Duration
	job.LastExitCode = result.ExitCode
	job.status = Stop

	return result
}
