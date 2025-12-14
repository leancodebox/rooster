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
	"runtime"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

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
		if job.Run {
			job.JobInit()
		}

		slog.Info(fmt.Sprintf("%v 加入常驻任务", job.UUID+job.JobName))
	}

	go scheduleV2(jobConfigV2.GetScheduledTask())
}

func scheduleV2(jobList []*Job) {
	for _, job := range jobList {
		job.ConfigInit()
		if !job.Run {
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
		if needFlush {
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
	}()

	executor := NewJobExecutor()

	counter := 1
	consecutiveFailures := 0
	for {
		if !itself.Run {
			slog.Info(fmt.Sprintf("%v : no Run ", itself.JobName))
			return
		}
		unitStartTime := time.Now()
		counter += 1

		// Prepare context for cancellation (Phase 3 transition)
		// For now we use Background but support cancellation via itself.cancel
		ctx, cancel := context.WithCancel(context.Background())
		itself.cancel = cancel

		// Execute the job
		result := executor.Execute(ctx, itself)

		// Cleanup context
		itself.cancel = nil
		cancel()

		// Logic for retry/backoff based on result
		cmdErr := result.Error

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

func (itself *Job) StopJob() {
	itself.confLock.Lock()
	defer itself.confLock.Unlock()
	itself.Run = false

	if itself.cancel != nil {
		itself.cancel()
	}
}

var start time.Time

func init() {
	start = time.Now()
}

func GetRunTime() time.Duration {
	return time.Since(start)
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

func GetLogDir() (string, error) {
	return getLogDir()
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
	defer func() {
		job.cancel = nil
		cancel()
	}()

	executor := NewJobExecutor()
	result := executor.Execute(ctx, job)

	if result.Error != nil {
		slog.Info(result.Error.Error())
	}
}

// JobInit 初始化并执行常驻任务守护
func (itself *Job) JobInit() error {
	itself.confLock.Lock()
	defer itself.confLock.Unlock()
	// 如果已经在运行中，则不重复启动
	if itself.status == Running {
		return errors.New("程序运行中")
	}
	itself.Run = true
	go itself.jobGuard()
	return nil
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
		JobSpec: JobSpec{
			UUID:    "",
			JobName: "echo-loop",
			Type:    JobTypeResident,
			Run:     true,
			BinPath: shellLoop,
			Dir:     "",
			Spec:    "",
			Options: RunOptions{OutputType: OutputTypeFile, OutputPath: logDir, MaxFailures: 5},
		},
	}
	scheduled := &Job{
		JobSpec: JobSpec{
			UUID:    "",
			JobName: "tick",
			Type:    JobTypeScheduled,
			Run:     true,
			BinPath: tick,
			Dir:     "",
			Spec:    "* * * * *",
			Options: RunOptions{OutputType: OutputTypeFile, OutputPath: logDir, MaxFailures: 5},
		},
	}
	return JobConfig{
		TaskList: []*Job{resident, scheduled},
		Config: BaseConfig{Dashboard: struct {
			Port int `json:"port"`
		}{Port: 9090}, DefaultOptions: RunOptions{OutputType: OutputTypeFile, OutputPath: logDir, MaxFailures: 5}},
	}
}
