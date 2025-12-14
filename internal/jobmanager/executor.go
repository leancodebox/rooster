package jobmanager

import (
	"context"
	"log/slog"
	"time"
)

// ExecutionResult 保存单次任务执行的结果
type ExecutionResult struct {
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	ExitCode  int
	Error     error
}

// JobExecutor 处理任务的执行逻辑
type JobExecutor struct {
	logManager *LogManager
}

// NewJobExecutor 创建一个新的执行器实例
func NewJobExecutor() *JobExecutor {
	return &JobExecutor{
		logManager: NewLogManager(),
	}
}

// Execute 处理单次运行的完整生命周期
func (e *JobExecutor) Execute(ctx context.Context, job *Job, onStart func(int)) ExecutionResult {
	result := ExecutionResult{
		StartTime: time.Now(),
	}

	// 1. 设置日志
	writer, fullLogPath, err := e.logManager.SetupLogger(job.JobName, job.Options)
	if err != nil {
		slog.Error("SetupLogger failed", "err", err)
	} else {
		job.runtimeLogPath = fullLogPath
		defer func() {
			_ = writer.Close()
		}()
	}

	// 2. 构建命令
	// buildCmdWithCtx 定义在 cmd_build_*.go
	cmd := buildCmdWithCtx(ctx, job)

	// 配置优雅退出 (Go 1.20+)
	// 当上下文被取消时，尝试先优雅终止进程组。
	// 如果在 WaitDelay 时间内未退出，运行时将强制杀死进程（及其父进程）。
	cmd.Cancel = func() error {
		return KillProcessGroup(cmd)
	}
	cmd.WaitDelay = 1 * time.Second

	if writer != nil {
		cmd.Stdout = writer
		cmd.Stderr = writer
	}

	// 3. 更新状态（开始）
	job.LastStart = result.StartTime
	job.status = Running

	// 4. 运行
	if err := cmd.Start(); err != nil {
		// 启动失败
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		result.Error = err
		result.ExitCode = -1 // 无法获取具体退出码

		job.LastExit = result.EndTime
		job.LastDuration = result.Duration
		job.LastExitCode = result.ExitCode
		job.status = Stop
		return result
	}

	// 启动成功，记录PID
	if onStart != nil && cmd.Process != nil {
		onStart(cmd.Process.Pid)
	}

	// 等待结束
	err = cmd.Wait()

	// 5. 更新状态（结束）
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
