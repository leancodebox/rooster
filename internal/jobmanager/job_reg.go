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

func (m *Manager) Closed() bool {
	m.closingLock.RLock()
	defer m.closingLock.RUnlock()
	return m.closing
}

func (m *Manager) StartClose() {
	m.closingLock.Lock()
	defer m.closingLock.Unlock()
	m.closing = true
}

func Closed() bool {
	if DefaultManager != nil {
		return DefaultManager.Closed()
	}
	return false
}

func StartClose() {
	if DefaultManager != nil {
		DefaultManager.StartClose()
	}
}

var DefaultManager *Manager

type Manager struct {
	config         JobConfig
	cron           *cron.Cron
	closing        bool
	closingLock    sync.RWMutex
	configLock     sync.Mutex
	taskStatusLock sync.Mutex
	startTime      time.Time
}

func NewManager(fileData []byte) (*Manager, error) {
	var config JobConfig
	err := json.Unmarshal(fileData, &config)
	if err != nil {
		return nil, err
	}

	m := &Manager{
		config:    config,
		cron:      cron.New(),
		startTime: time.Now(),
	}
	return m, nil
}

func (m *Manager) Start() {
	for _, job := range m.config.GetResidentTask() {
		m.ConfigInit(job)

		if job.Run {
			m.StartResidentJob(job)
		}

		slog.Info(fmt.Sprintf("%v 加入常驻任务", job.UUID+job.JobName))
	}

	go m.scheduleV2(m.config.GetScheduledTask())
}

func RegV2(fileData []byte) {
	mgr, err := NewManager(fileData)
	if err != nil {
		slog.Info(err.Error())
		return
	}
	DefaultManager = mgr
	DefaultManager.Start()
}

func (m *Manager) scheduleV2(jobList []*Job) {
	for _, job := range jobList {
		m.ConfigInit(job)
		if !job.Run {
			continue
		}
		entityId, err := m.cron.AddFunc(job.Spec, func(job *Job) func() {
			return func() {
				m.execAction(job)
			}
		}(job))
		if err != nil {
			slog.Error(err.Error())
		} else {
			job.entityId = entityId
			slog.Info(fmt.Sprintf("%v 加入任务", job.GetJobName()))
		}
	}
	m.cron.Start()
}

func (m *Manager) ConfigInit(itself *Job) {
	needFlush := false
	defer func() {
		if needFlush {
			m.flushConfig()
		}
	}()
	if itself.UUID == "" {
		itself.UUID = generateUUID()
		needFlush = true
	}
	itself.confLock = &sync.Mutex{}
	itself.runOnceLock = &sync.Mutex{}
	def := m.config.Config.DefaultOptions
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

	// Initialize runtime log path
	if path, err := ResolveLogPath(itself.JobName, itself.Options); err == nil {
		itself.runtimeLogPath = path
	}
}

func (itself *Job) GetJobName() string {
	return fmt.Sprintf("%v:%v", itself.UUID, itself.JobName)
}

// runResidentJobLoop 常驻任务的守护循环
func (m *Manager) runResidentJobLoop(job *Job) {
	defer func() {
		if err := recover(); err != nil {
			slog.Error("jobGuard", "err", err)
		}
	}()

	executor := NewJobExecutor()

	counter := 1
	consecutiveFailures := 0
	for {
		if !job.Run {
			slog.Info(fmt.Sprintf("%v : no Run ", job.JobName))
			return
		}
		unitStartTime := time.Now()
		counter += 1

		// 准备取消上下文 (Phase 3 迁移)
		// 目前使用 Background，但支持通过 job.cancel 进行取消
		ctx, cancel := context.WithCancel(context.Background())
		job.cancel = cancel

		// 执行任务
		result := executor.Execute(ctx, job, func(pid int) {
			job.Pid = pid
			m.flushConfig()
		})

		// 清理上下文
		job.cancel = nil
		cancel()

		// 基于结果的重试/退避逻辑
		cmdErr := result.Error

		executionTime := time.Since(unitStartTime)
		if cmdErr != nil {
			slog.Info(cmdErr.Error(), "jobName", job.JobName)
		}
		threshold := maxExecutionTime
		if job.Options.MinRunSeconds > 0 {
			threshold = time.Duration(job.Options.MinRunSeconds) * time.Second
		}
		if executionTime <= threshold {
			consecutiveFailures += 1
		} else {
			consecutiveFailures = 0
		}

		if !job.Run || m.Closed() {
			msg := job.JobName + " 溜了溜了"
			slog.Info(msg)
			break
		}

		failLimit := maxConsecutiveFailures
		if job.Options.MaxFailures > 0 && job.Options.MaxFailures > failLimit {
			failLimit = job.Options.MaxFailures
		}
		if consecutiveFailures >= failLimit {
			msg := job.JobName + "程序连续3次启动失败，停止重启"
			slog.Info(msg)
			job.Run = false
			m.flushConfig()
			break
		} else {
			msg := job.JobName + "程序终止尝试重新运行"
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

// ForceRunJob 强制运行任务
func (m *Manager) ForceRunJob(job *Job) error {
	job.Run = true
	return m.StartResidentJob(job)
}

func ForceRunJob(job *Job) error {
	if DefaultManager != nil {
		return DefaultManager.ForceRunJob(job)
	}
	return errors.New("manager not initialized")
}

// StopJob 停止任务
func (m *Manager) StopJob(job *Job) {
	job.confLock.Lock()
	defer job.confLock.Unlock()
	job.Run = false

	if job.cancel != nil {
		job.cancel()
	}
}

func StopJob(job *Job) {
	if DefaultManager != nil {
		DefaultManager.StopJob(job)
	}
}

func (m *Manager) GetRunTime() time.Duration {
	return time.Since(m.startTime)
}

func GetRunTime() time.Duration {
	if DefaultManager != nil {
		return DefaultManager.GetRunTime()
	}
	return 0
}

func (m *Manager) GetStartTime() time.Time {
	return m.startTime
}

func GetStartTime() time.Time {
	if DefaultManager != nil {
		return DefaultManager.GetStartTime()
	}
	return time.Time{}
}

var userHomeDirFn = os.UserHomeDir

func getConfigPath() (string, error) {
	homeDir, err := userHomeDirFn()
	if err != nil {
		slog.Error("获取家目录失败", "err", err)
		homeDir = "tmp"
	}
	if devPath := getDevHomeDir(); devPath != "" {
		homeDir = devPath
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
	if devPath := getDevHomeDir(); devPath != "" {
		homeDir = devPath
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
	}
	var tmp JobConfig
	if json.Unmarshal(fileData, &tmp) != nil {
		def := generateDefaultJobConfig()
		b, _ := json.MarshalIndent(def, "", "  ")
		_ = os.WriteFile(jobConfigPath, b, 0644)
		RegV2(b)
		return nil
	}
	RegV2(fileData)
	return nil
}

func (m *Manager) flushConfig() {
	jobConfigPath, err := getConfigPath()
	slog.Error("flushConfigErr", "err", err)
	if err != nil {
		slog.Error("flushConfigErr", "err", err)
		return
	}
	m.configLock.Lock()
	defer m.configLock.Unlock()
	data, err := json.MarshalIndent(m.config, "", "  ")
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

// buildCmd / buildCmdWithCtx 定义在特定平台的代码文件中，以获得更好的测试覆盖率

// RunScheduledJob 运行一次定时任务（手动触发或定时触发）
func (m *Manager) RunScheduledJob(job *Job) error {
	if job.runOnceLock.TryLock() {
		go func(j *Job) {
			defer j.runOnceLock.Unlock()
			m.execAction(j)
		}(job)
		return nil
	}
	return errors.New("上次手动运行尚未结束")
}

func RunScheduledJob(job *Job) error {
	if DefaultManager != nil {
		return DefaultManager.RunScheduledJob(job)
	}
	return errors.New("manager not initialized")
}

// execAction 执行一次性任务并记录观测值
func (m *Manager) execAction(job *Job) {
	ctx, cancel := context.WithCancel(context.Background())
	job.cancel = cancel
	defer func() {
		job.cancel = nil
		cancel()
	}()

	executor := NewJobExecutor()
	result := executor.Execute(ctx, job, nil)

	if result.Error != nil {
		slog.Info(result.Error.Error())
	}
}

// StartResidentJob 初始化并执行常驻任务守护
func (m *Manager) StartResidentJob(job *Job) error {
	job.confLock.Lock()
	defer job.confLock.Unlock()
	// 如果已经在运行中，则不重复启动
	if job.RunningLoop {
		return errors.New("程序运行中")
	}
	job.Run = true
	job.RunningLoop = true
	go func() {
		defer func() {
			job.confLock.Lock()
			job.RunningLoop = false
			job.confLock.Unlock()
		}()
		m.runResidentJobLoop(job)
	}()
	return nil
}

func StartResidentJob(job *Job) error {
	if DefaultManager != nil {
		return DefaultManager.StartResidentJob(job)
	}
	return errors.New("manager not initialized")
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
