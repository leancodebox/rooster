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

	"github.com/leancodebox/rooster/roosterSay"
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
func StartOpen() {
	signClose = false
}

type RunStatus int

const (
	Stop RunStatus = iota
	Running
)

var jobConfigV2 JobConfigV2

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
			err := flushConfig()
			if err != nil {
				slog.Error("flushConfig", "err", err)
			}
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
	if itself.Options.OutputType == OutputTypeFile && itself.Options.OutputPath != "" {
		err := os.MkdirAll(itself.Options.OutputPath, os.ModePerm)
		if err != nil {
			slog.Info(err.Error())
		}
		logFile, err := os.OpenFile(filepath.Join(itself.Options.OutputPath, itself.JobName+"_log.txt"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			slog.Info(err.Error())
		}
		defer logFile.Close()
		out, errw := buildWriters(itself.UUID, logFile)
		itself.cmd.Stdout = out
		itself.cmd.Stderr = errw
	} else {
		out, errw := buildWriters(itself.UUID, nil)
		itself.cmd.Stdout = out
		itself.cmd.Stderr = errw
	}
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
			slog.Info(cmdErr.Error())
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
			roosterSay.Send(msg)
			break
		}

		failLimit := maxConsecutiveFailures
		if itself.Options.MaxFailures > 0 && itself.Options.MaxFailures > failLimit {
			failLimit = itself.Options.MaxFailures
		}
		if consecutiveFailures >= failLimit {
			msg := itself.JobName + "程序连续3次启动失败，停止重启"
			slog.Info(msg)
			roosterSay.Send(msg)
			break
		} else {
			msg := itself.JobName + "程序终止尝试重新运行"
			roosterSay.Send(msg)
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

	if len(updateStatus) == 1 && updateStatus[0] == true {
		itself.Run = false
	}
	if itself.cancel != nil {
		itself.cancel()
	}
	if itself.cmd != nil && itself.cmd.Process != nil {
		_ = itself.cmd.Process.Signal(os.Interrupt)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		<-ctx.Done()
		_ = KillProcessGroup(itself.cmd)
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
		var tmp JobConfigV2
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

func flushConfig() error {
	jobConfigPath, err := getConfigPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(jobConfigV2, "", "  ")
	if err != nil {
		return err
	}
	err = os.WriteFile(jobConfigPath, data, 0644)
	if err != nil {
		return err
	}
	return nil
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
	if job.Options.OutputType == OutputTypeFile && job.Options.OutputPath != "" {
		err := os.MkdirAll(job.Options.OutputPath, os.ModePerm)
		if err != nil {
			slog.Info(err.Error())
		}
		logFile, err := os.OpenFile(filepath.Join(job.Options.OutputPath, job.JobName+"_log.txt"),
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			slog.Info(err.Error())
		}
		defer logFile.Close()
		out, errw := buildWriters(job.UUID, logFile)
		cmd.Stdout = out
		cmd.Stderr = errw
	} else {
		out, errw := buildWriters(job.UUID, nil)
		cmd.Stdout = out
		cmd.Stderr = errw
	}
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
func generateDefaultJobConfig() JobConfigV2 {
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
	return JobConfigV2{
		TaskList: []*Job{resident, scheduled},
		Config: BaseConfig{Dashboard: struct {
			Port int `json:"port"`
		}{Port: 9090}, DefaultOptions: RunOptions{OutputType: OutputTypeFile, OutputPath: logDir, MaxFailures: 5}},
	}
}
