package jobmanager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/leancodebox/rooster/resource"
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

		if !itself.Run || Closed() {
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
			var maxDelay = 16
			calculatedDelay := maxDelay
			if consecutiveFailures < 10 {
				calculatedDelay = 1 << uint(consecutiveFailures)
			}
			// 确保延迟不超过最大限制
			actualDelay := time.Duration(min(calculatedDelay, maxDelay))
			time.Sleep(actualDelay * time.Second)
		}
	}
}

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
	if itself.cmd != nil && itself.cmd.Process != nil {
		// 先发送终止信号
		err := itself.cmd.Process.Signal(os.Interrupt)
		if err != nil {
			slog.Info("发送终止信号失败:", "err", err)
		}

		// 等待5秒让进程优雅退出
		done := make(chan error, 1)
		go func() {
			done <- itself.cmd.Wait()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		select {
		case <-ctx.Done():
			// 超时后强制终止
			err = itself.cmd.Process.Kill()
			if err != nil {
				slog.Info("强制终止进程失败:", "err", err)
				return
			}
		case err := <-done:
			if err != nil {
				slog.Info("进程退出错误:", "err", err)
			}
		}
		itself.cmd = nil
	}
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

func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		slog.Error("获取家目录失败", "err", err)
		homeDir = "tmp"
	}
	configDir := path.Join(homeDir, ".roosterTaskConfig")
	slog.Info("当前目录", "homeDir", homeDir)
	if _, err = os.Stat(configDir); os.IsNotExist(err) {
		err = os.Mkdir(configDir, os.ModePerm)
		if err != nil {
			return "", err
		}
	}
	jobConfigPath := path.Join(configDir, "jobConfig.json")
	return jobConfigPath, nil
}

func RegByUserConfig() error {
	jobConfigPath, err := getConfigPath()
	if err != nil {
		return err
	}
	fileData, err := os.ReadFile(jobConfigPath)
	if err != nil {
		fileData = resource.GetJobConfigDefault()
		if len(fileData) == 0 {
			return err
		}
		_ = os.WriteFile(jobConfigPath, fileData, 0644)
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

var c = cron.New()

func buildCmd(job *Job) *exec.Cmd {
    var shell string
    var args []string
    if runtime.GOOS == "windows" {
        shell = "cmd.exe"
        args = append([]string{"/C", job.BinPath}, job.Params...)
    } else {
        shell = os.Getenv("SHELL")
        if shell == "" {
            shell = "/bin/bash"
        }
        fullCommand := job.BinPath
        if len(job.Params) > 0 {
            fullCommand += " " + strings.Join(job.Params, " ")
        }
        args = []string{"-l", "-c", fullCommand}
    }
    cmd := exec.Command(shell, args...)
    HideWindows(cmd)
    cmd.Env = os.Environ()
    cmd.Dir = job.Dir
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    return cmd
}

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
    cmd := buildCmd(&job)
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
        cmd.Stdout = logFile
        cmd.Stderr = logFile
    }
    cmdErr := cmd.Run()
    if cmdErr != nil {
        slog.Info(cmdErr.Error())
    }
}

// JobInit 初始化并执行
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
