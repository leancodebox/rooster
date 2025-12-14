package jobmanager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"gopkg.in/natefinch/lumberjack.v2"
)

type dualWriter struct {
	file *lumberjack.Logger
}

func (w *dualWriter) Write(p []byte) (n int, err error) {
	_, _ = os.Stdout.Write(p)
	return w.file.Write(p)
}

func (w *dualWriter) Close() error {
	return w.file.Close()
}

var startTime = time.Now()

var signClose = false

func Closed() bool {
	return signClose
}
func StartClose() {
	signClose = true
}

type RunStatus int

const (
	Stop RunStatus = iota
	Running
)

var jobConfigV2 JobConfig

func RegV2(fileData []byte) {
	err := json.Unmarshal(fileData, &jobConfigV2)
	if err != nil {
		slog.Info(err.Error())
		return
	}

	for _, job := range jobConfigV2.GetResidentTask() {
		job.ConfigInit()
		job.JobInit()

		slog.Info(fmt.Sprintf("%v 加入常驻任务", job.UUID+job.JobName))
	}

	go scheduleV2(jobConfigV2.GetScheduledTask())
}

func scheduleV2(jobList []*Job) {
	for _, job := range jobList {
		job.ConfigInit()
		if job.Run == false {
			continue
		}
		entityId, err := c.AddFunc(job.Spec, func(job *Job) func() {
			return func() {
				execAction(job)
			}
		}(job))
		if err != nil {
			slog.Error(err.Error())
		} else {
			job.entityId = entityId
			slog.Info(fmt.Sprintf("%v 加入任务", job.GetJobName()))
		}
	}
	c.Start()
}

func (itself *Job) ConfigInit() {
	needFlush := false
	defer func() {
		if needFlush == true {
			flushConfig()
		}
	}()
	if itself.UUID == "" {
		itself.UUID = generateUUID()
		needFlush = true
	}
	itself.confLock = &sync.Mutex{}
	itself.runOnceLock = &sync.Mutex{}
	def := jobConfigV2.Config.DefaultOptions
	if itself.Options.OutputType == 0 {
		if def.OutputType == 0 {
			itself.Options.OutputType = OutputTypeFile
		} else {
			itself.Options.OutputType = def.OutputType
		}
	}
	if itself.Options.OutputPath == "" {
		if def.OutputPath != "" {
			itself.Options.OutputPath = def.OutputPath
		} else if ld, err := getLogDir(); err == nil {
			itself.Options.OutputPath = ld
		}
	}
	if itself.Options.MaxFailures == 0 {
		itself.Options.MaxFailures = def.MaxFailures
	}
	if itself.Options.MinRunSeconds == 0 {
		itself.Options.MinRunSeconds = def.MinRunSeconds
	}
	if itself.Options.ShellPath == "" {
		itself.Options.ShellPath = def.ShellPath
	}
}

func (itself *Job) GetJobName() string {
	return fmt.Sprintf("%v:%v", itself.UUID, itself.JobName)
}

func (itself *Job) jobGuard() {
	defer func() {
		if err := recover(); err != nil {
			slog.Error("jobGuard", "err", err)
		}
		itself.cmd = nil
	}()

	// 确定日志路径
	logDir := itself.Options.OutputPath
	useDefault := false
	if logDir != "" {
		if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
			slog.Error("用户设置的日志路径无效，使用默认路径", "path", logDir, "err", err)
			useDefault = true
		}
	} else {
		useDefault = true
	}

	if useDefault {
		if defDir, err := getLogDir(); err == nil {
			logDir = defDir
			// 更新配置中的路径，以便 UI 展示正确位置（注意：这可能会在 flushConfig 时保存到文件）
			itself.Options.OutputPath = logDir
		}
	}

	// 确保最终路径存在
	_ = os.MkdirAll(logDir, os.ModePerm)

	// 创建 logger
	lLogger := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, itself.JobName+"_log.txt"),
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     28,   // days
		Compress:   true, // disabled by default
	}

	// 使用 dualWriter 同时输出到 stdout 和文件
	writer := &dualWriter{file: lLogger}

	defer func() {
		_ = writer.Close()
	}()

	itself.cmd.Stdout = writer
	itself.cmd.Stderr = writer
	counter := 1
	consecutiveFailures := 0
	for {
		if !itself.Run {
			slog.Info(fmt.Sprintf("%v : no Run ", itself.JobName))
			return
		}
		unitStartTime := time.Now()
		counter += 1
		cmdErr := itself.cmd.Start()
		if cmdErr == nil {
			itself.LastStart = unitStartTime
			itself.status = Running
			cmdErr = itself.cmd.Wait()
			itself.LastExit = time.Now()
			itself.LastDuration = time.Since(unitStartTime)
			if itself.cmd.ProcessState != nil {
				itself.LastExitCode = itself.cmd.ProcessState.ExitCode()
			}
			itself.status = Stop
		}

		executionTime := time.Since(unitStartTime)
		if cmdErr != nil {
			slog.Info(cmdErr.Error(), "jobName", itself.JobName)
		}
		threshold := maxExecutionTime
		if itself.Options.MinRunSeconds > 0 {
			threshold = time.Duration(itself.Options.MinRunSeconds) * time.Second
		}
		if executionTime <= threshold {
			consecutiveFailures += 1
		} else {
			consecutiveFailures = 0
		}

		if !itself.Run || Closed() {
			msg := itself.JobName + " 溜了溜了"
			slog.Info(msg)
			break
		}

		failLimit := maxConsecutiveFailures
		if itself.Options.MaxFailures > 0 && itself.Options.MaxFailures > failLimit {
			failLimit = itself.Options.MaxFailures
		}
		if consecutiveFailures >= failLimit {
			msg := itself.JobName + "程序连续3次启动失败，停止重启"
			slog.Info(msg)
			break
		} else {
			msg := itself.JobName + "程序终止尝试重新运行"
			slog.Info(msg)
			var maxDelay = 16
			calculatedDelay := 1 << uint(consecutiveFailures)
			if calculatedDelay > maxDelay {
				calculatedDelay = maxDelay
			}
			jitter := rand.Intn(calculatedDelay/2 + 1)
			actualDelay := calculatedDelay + jitter
			if actualDelay > maxDelay {
				actualDelay = maxDelay
			}
			sleepFn(time.Duration(actualDelay) * time.Second)
		}
	}
}

var sleepFn = time.Sleep

func (itself *Job) ForceRunJob() error {
	itself.Run = true
	return itself.JobInit()
}

func (itself *Job) StopJob(updateStatus ...bool) {
	itself.confLock.Lock()
	defer itself.confLock.Unlock()
	defer func() {
		itself.cmd = nil
	}()

	if len(updateStatus) == 1 && updateStatus[0] == true {
		itself.Run = false
	}
	if itself.cancel != nil {
		itself.cancel()
	}
	if itself.cmd != nil && itself.cmd.Process != nil {
		_ = itself.cmd.Process.Signal(os.Interrupt)
		done := make(chan error, 1)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		// 等待进程退出
		go func() {
			_, err := itself.cmd.Process.Wait()
			done <- err
		}()
		select {
		case err := <-done:
			if err != nil {
				slog.Info(err.Error(), "jobName", itself.JobName)
			} else {
				slog.Info("waitEnd", "jobName", itself.JobName)
			}
			return
		case <-ctx.Done():
			slog.Info("KillProcessGroup", "jobName", itself.JobName)
			if err := KillProcessGroup(itself.cmd); err != nil {
				slog.Info(err.Error())
			}
		}
	}
	itself.status = Stop
}

var start time.Time

func init() {
	start = time.Now()
}

func GetRunTime() time.Duration {
	return time.Now().Sub(start)
}

func GetStartTime() time.Time {
	return start
}

var userHomeDirFn = os.UserHomeDir

func getConfigPath() (string, error) {
	homeDir, err := userHomeDirFn()
	if err != nil {
		slog.Error("获取家目录失败", "err", err)
		homeDir = "tmp"
	}
	configDir := path.Join(homeDir, ".roosterTaskConfig")
	slog.Info("当前目录", "homeDir", homeDir)
	if _, err = os.Stat(configDir); os.IsNotExist(err) {
		err = os.MkdirAll(configDir, os.ModePerm)
		if err != nil {
			return "", err
		}
	}
	jobConfigPath := path.Join(configDir, "jobConfig.json")
	return jobConfigPath, nil
}

func getLogDir() (string, error) {
	homeDir, err := userHomeDirFn()
	if err != nil {
		slog.Error("获取家目录失败", "err", err)
		homeDir = "tmp"
	}
	configDir := path.Join(homeDir, ".roosterTaskConfig")
	logDir := path.Join(configDir, "log")
	if _, err = os.Stat(logDir); os.IsNotExist(err) {
		if err = os.MkdirAll(logDir, os.ModePerm); err != nil {
			return "", err
		}
	}
	return logDir, nil
}

func RegByUserConfig() error {
	jobConfigPath, err := getConfigPath()
	if err != nil {
		return err
	}
	fileData, err := os.ReadFile(jobConfigPath)
	if err != nil || len(fileData) == 0 {
		def := generateDefaultJobConfig()
		b, _ := json.MarshalIndent(def, "", "  ")
		_ = os.WriteFile(jobConfigPath, b, 0644)
		RegV2(b)
		return nil
	} else {
		var tmp JobConfig
		if json.Unmarshal(fileData, &tmp) != nil {
			def := generateDefaultJobConfig()
			b, _ := json.MarshalIndent(def, "", "  ")
			_ = os.WriteFile(jobConfigPath, b, 0644)
			RegV2(b)
			return nil
		}
	}
	RegV2(fileData)
	return nil
}

func flushConfig() {
	jobConfigPath, err := getConfigPath()
	slog.Error("flushConfigErr", "err", err)
	if err != nil {
		slog.Error("flushConfigErr", "err", err)
		return
	}
	data, err := json.MarshalIndent(jobConfigV2, "", "  ")
	if err != nil {
		slog.Error("flushConfigErr", "err", err)
		return
	}
	err = os.WriteFile(jobConfigPath, data, 0644)
	if err != nil {
		slog.Error("flushConfigErr", "err", err)
		return
	}
}

// Cron 调度器实例
var c = cron.New()

// buildCmd / buildCmdWithCtx moved to platform-specific files for better test coverage

func (itself *Job) RunOnce() error {
	if itself.runOnceLock.TryLock() {
		go func(job *Job) {
			defer itself.runOnceLock.Unlock()
			execAction(job)
		}(itself)
		return nil
	}
	return errors.New("上次手动运行尚未结束")
}

// execAction 执行一次性任务并记录观测值
func execAction(job *Job) {
	ctx, cancel := context.WithCancel(context.Background())
	job.cancel = cancel
	cmd := buildCmdWithCtx(ctx, job)
	job.cmd = cmd
	job.LastStart = time.Now()
	job.status = Running

	// 确定日志路径
	logDir := job.Options.OutputPath
	useDefault := false
	if logDir != "" {
		if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
			slog.Error("用户设置的日志路径无效，使用默认路径", "path", logDir, "err", err)
			useDefault = true
		}
	} else {
		useDefault = true
	}

	if useDefault {
		if defDir, err := getLogDir(); err == nil {
			logDir = defDir
			job.Options.OutputPath = logDir
		}
	}
	_ = os.MkdirAll(logDir, os.ModePerm)

	// 创建 logger
	lLogger := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, job.JobName+"_log.txt"),
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     28,   // days
		Compress:   true, // disabled by default
	}

	writer := &dualWriter{file: lLogger}
	defer func() { _ = writer.Close() }()

	cmd.Stdout = writer
	cmd.Stderr = writer

	cmdErr := cmd.Run()
	if cmdErr != nil {
		slog.Info(cmdErr.Error())
	}
	job.LastExit = time.Now()
	job.LastDuration = job.LastExit.Sub(job.LastStart)
	if cmd.ProcessState != nil {
		job.LastExitCode = cmd.ProcessState.ExitCode()
	}
	job.status = Stop
	job.cancel = nil
	job.cmd = nil
}

// JobInit 初始化并执行常驻任务守护
func (itself *Job) JobInit() error {
	itself.confLock.Lock()
	defer itself.confLock.Unlock()
	if itself.cmd == nil {
		itself.cmd = buildCmd(itself)
		go itself.jobGuard()
		return nil
	}
	return errors.New("程序运行中")
}
func generateDefaultJobConfig() JobConfig {
	shellLoop := "while true; do echo 'rooster'; sleep 1; done"
	tick := "echo 'tick'"
	if runtime.GOOS == "windows" {
		shellLoop = "for /l %i in (1,0,2) do (echo rooster & timeout /t 1)"
		tick = "echo tick"
	}
	logDir, _ := getLogDir()
	resident := &Job{
		UUID:    "",
		JobName: "echo-loop",
		Type:    JobTypeResident,
		Run:     true,
		BinPath: shellLoop,
		Dir:     "",
		Spec:    "",
		Options: RunOptions{OutputType: OutputTypeFile, OutputPath: logDir, MaxFailures: 5},
	}
	scheduled := &Job{
		UUID:    "",
		JobName: "tick",
		Type:    JobTypeScheduled,
		Run:     true,
		BinPath: tick,
		Dir:     "",
		Spec:    "* * * * *",
		Options: RunOptions{OutputType: OutputTypeFile, OutputPath: logDir, MaxFailures: 5},
	}
	return JobConfig{
		TaskList: []*Job{resident, scheduled},
		Config: BaseConfig{Dashboard: struct {
			Port int `json:"port"`
		}{Port: 9090}, DefaultOptions: RunOptions{OutputType: OutputTypeFile, OutputPath: logDir, MaxFailures: 5}},
	}
}
