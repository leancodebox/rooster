package jobmanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/leancodebox/rooster/resource"
	"github.com/leancodebox/rooster/roosterSay"
	"github.com/robfig/cron/v3"
)

var startTime = time.Now()

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

	for _, job := range jobConfigV2.ResidentTask {
		job.ConfigInit(1)
		job.JobInit()

		slog.Info(fmt.Sprintf("%v 加入常驻任务", job.UUID+job.JobName))
	}

	go scheduleV2(jobConfigV2.ScheduledTask)
}

func scheduleV2(jobList []*Job) {
	for _, job := range jobList {
		job.ConfigInit(2)
		if job.Run == false {
			continue
		}
		entityId, err := c.AddFunc(job.Spec, func(job *Job) func() {
			return func() {
				execAction(*job)
			}
		}(job))
		if err != nil {
			slog.Error(err.Error())
		} else {
			job.entityId = entityId
			job.status = Running
			slog.Info(fmt.Sprintf("%v 加入任务", job.GetJobName()))
		}
	}
	c.Run()
}

func (itself *Job) ConfigInit(taskType int) {
	needFlush := false
	defer func() {
		if needFlush == true {
			// 需要刷新配置
		}
	}()
	if itself.UUID == "" {
		itself.UUID = generateUUID()
		needFlush = true
	}
	if itself.Type != taskType {
		itself.Type = taskType
		needFlush = true
	}
	itself.confLock = &sync.Mutex{}
	itself.runOnceLock = &sync.Mutex{}
}

func (itself *Job) GetJobName() string {
	return fmt.Sprintf("%v:%v", itself.UUID, itself.JobName)
}

func (itself *Job) jobGuard() {
	defer func() {
		itself.cmd = nil
	}()
	if itself.Options.OutputType == OutputTypeFile && itself.Options.OutputPath != "" {
		err := os.MkdirAll(itself.Options.OutputPath, os.ModePerm)
		if err != nil {
			slog.Info(err.Error())
		}
		logFile, err := os.OpenFile(itself.Options.OutputPath+"/"+itself.JobName+"_log.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			slog.Info(err.Error())
		}
		defer logFile.Close()
		itself.cmd.Stdout = logFile
		itself.cmd.Stderr = logFile
	}
	counter := 1
	consecutiveFailures := 1
	for {
		if !itself.Run {
			slog.Info(fmt.Sprintf("%v : no Run ", itself.JobName))
			return
		}
		unitStartTime := time.Now()
		counter += 1
		cmdErr := itself.cmd.Start()
		if cmdErr == nil {
			itself.status = Running
			cmdErr = itself.cmd.Wait()
			itself.status = Stop
		}

		executionTime := time.Since(unitStartTime)
		if cmdErr != nil {
			slog.Info(cmdErr.Error())
		}
		if executionTime <= maxExecutionTime {
			consecutiveFailures += 1
		} else {
			consecutiveFailures = 1
		}

		if !itself.Run {
			msg := itself.JobName + " 溜了溜了"
			slog.Info(msg)
			roosterSay.Send(msg)
			break
		}

		if consecutiveFailures >= max(maxConsecutiveFailures, itself.Options.MaxFailures) {
			msg := itself.JobName + "程序连续3次启动失败，停止重启"
			slog.Info(msg)
			roosterSay.Send(msg)
			break
		} else {
			msg := itself.JobName + "程序终止尝试重新运行"
			roosterSay.Send(msg)
			slog.Info(msg)
		}
	}
}

func (itself *Job) ForceRunJob() error {
	itself.Run = true
	return itself.JobInit()
}

func (itself *Job) StopJob() {
	itself.confLock.Lock()
	defer itself.confLock.Unlock()

	itself.Run = false
	if itself.cmd != nil && itself.cmd.Process != nil {
		err := itself.cmd.Process.Kill()
		if err != nil {
			slog.Info(err.Error())
			return
		}
		itself.cmd = nil
	}
}

func RegByUserConfig() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		slog.Error("获取家目录失败", "err", err)
		return err
	}
	slog.Info("当前系统的家目录", "homeDir", homeDir)
	configDir := path.Join(homeDir, ".roosterTaskConfig")
	if _, err = os.Stat(configDir); os.IsNotExist(err) {
		err = os.Mkdir(configDir, os.ModePerm)
		if err != nil {
			return err
		}
	}
	jobConfigPath := path.Join(configDir, "jobConfig.json")
	if _, err = os.Stat(jobConfigPath); os.IsNotExist(err) {
		err = os.WriteFile(jobConfigPath, resource.GetJobConfigDefault(), 0644)
		if err != nil {
			slog.Error("无法写入文件", "err", err)
			return err
		}
	}
	fileData, err := os.ReadFile(jobConfigPath)
	if err != nil {
		return err
	}
	RegV2(fileData)
	return nil
}

var c = cron.New()

func (itself *Job) RunOnce() error {
	if itself.runOnceLock.TryLock() {
		go func(job Job) {
			defer itself.runOnceLock.Unlock()
			execAction(job)
		}(*itself)
		return nil
	}
	return errors.New("上次手动运行尚未结束")
}

func execAction(job Job) {
	cmd := exec.Command(job.BinPath, job.Params...)
	cmd.Dir = job.Dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	HideWindows(cmd)
	if job.Options.OutputType == OutputTypeFile && job.Options.OutputPath != "" {
		err := os.MkdirAll(job.Options.OutputPath, os.ModePerm)
		if err != nil {
			slog.Info(err.Error())
		}
		logFile, err := os.OpenFile(job.Options.OutputPath+"/"+job.JobName+"_log.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			slog.Info(err.Error())
		}
		defer logFile.Close()
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}
	cmdErr := cmd.Run()
	if cmdErr != nil {
		slog.Info(cmdErr.Error())
	}
}

func saveTask(job Job, jobType int) {
	if job.UUID == "" {
		job.UUID = generateUUID()
	}
	// add job
}

func generateUUID() string {
	var UUID uuid.UUID
	UUID, err := uuid.NewRandom()
	if err != nil {
		return time.Now().Format(time.UnixDate)
	}
	return UUID.String()
}
