package jobmanager

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// JobStatusShow 对外展示的任务状态结构
type JobStatusShow struct {
	UUID    string     `json:"uuid"`
	JobName string     `json:"jobName"`
	Type    int        `json:"type"` // 运行模式 1 常驻 / 2 定时
	Run     bool       `json:"run"`
	BinPath string     `json:"binPath"`
	Dir     string     `json:"dir"`
	Spec    string     `json:"spec"`
	Options RunOptions `json:"options"` // 运行选项
	Link    string     `json:"link"`    // 快速跳转链接

	Status       RunStatus     `json:"status"`
	LastStart    time.Time     `json:"lastStart"`
	LastExit     time.Time     `json:"lastExit"`
	LastExitCode int           `json:"lastExitCode"`
	LastDuration time.Duration `json:"lastDuration"`

	// Log info
	HasLog      bool   `json:"hasLog"`
	RealLogPath string `json:"realLogPath"`
	LogSize     int64  `json:"size"`
	LogModTime  string `json:"modTime"`
}

// 类型转化
func job2jobStatus(job Job) JobStatusShow {
	js := JobStatusShow{
		UUID:         job.UUID,
		JobName:      job.JobName,
		Link:         job.Link,
		Type:         int(job.Type),
		Run:          job.Run,
		BinPath:      job.BinPath,
		Dir:          job.Dir,
		Spec:         job.Spec,
		Options:      job.Options,
		Status:       job.status,
		LastStart:    job.LastStart,
		LastExit:     job.LastExit,
		LastExitCode: job.LastExitCode,
		LastDuration: job.LastDuration,
	}

	// Populate log info
	// 1. Use runtime path if available
	if job.runtimeLogPath != "" {
		js.RealLogPath = job.runtimeLogPath
	}

	// 2. If no runtime path, try to find existing log file
	if js.RealLogPath == "" {
		defaultDir, _ := getLogDir()

		// Check user configured path
		if job.Options.OutputPath != "" {
			p := filepath.Join(job.Options.OutputPath, job.JobName+"_log.txt")
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				js.RealLogPath = p
			}
		}

		// Check default path if not found yet
		if js.RealLogPath == "" && defaultDir != "" {
			p := filepath.Join(defaultDir, job.JobName+"_log.txt")
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				js.RealLogPath = p
			}
		}

		// 3. If still not found, set to expected path
		if js.RealLogPath == "" {
			targetDir := job.Options.OutputPath
			if targetDir == "" {
				targetDir = defaultDir
			}
			if targetDir != "" {
				js.RealLogPath = filepath.Join(targetDir, job.JobName+"_log.txt")
			}
		}
	}

	// Fill details if path is set
	if js.RealLogPath != "" {
		if st, err := os.Stat(js.RealLogPath); err == nil && !st.IsDir() {
			js.HasLog = true
			js.LogSize = st.Size()
			js.LogModTime = st.ModTime().Format("2006-01-02 15:04:05")
		}
	}

	return js
}

func JobList() []JobStatusShow {
	var jobNameList []JobStatusShow
	for _, job := range jobConfigV2.TaskList {
		jobNameList = append(jobNameList, job2jobStatus(*job))
	}
	return jobNameList
}

func getJobByJobId(uuId string) *Job {
	for _, job := range jobConfigV2.GetResidentTask() {
		if uuId == job.UUID {
			return job
		}
	}
	return nil
}

func JobRunResidentTask(jobId string) error {
	defer flushConfig()
	jh := getJobByJobId(jobId)
	if jh == nil {
		return errors.New("jobId不存在")
	}
	return jh.ForceRunJob()
}

func JobStopResidentTask(jobId string) error {
	jh := getJobByJobId(jobId)
	if jh == nil {
		return errors.New("jobId不存在")
	}
	defer flushConfig()
	jh.StopJob()
	return nil
}

func StopAll() {
	StartClose()
	wg := sync.WaitGroup{}
	for _, item := range jobConfigV2.GetResidentTask() {
		slog.Info(item.JobName + "准备退出")
		wg.Go(func(job *Job) func() {
			return func() {
				job.StopJob()
				slog.Info(job.JobName + "退出")
			}
		}(item))
	}
	wg.Wait()
}

func RunStartTime() time.Time {
	return startTime
}

func GetHttpConfig() BaseConfig {
	return jobConfigV2.Config
}

func getTaskByTaskId(uuId string) *Job {
	for _, job := range jobConfigV2.GetScheduledTask() {
		if uuId == job.UUID {
			return job
		}
	}
	return nil
}

var taskStatusLock sync.Mutex

func OpenCloseTask(taskId string, run bool) error {
	taskStatusLock.Lock()
	defer taskStatusLock.Unlock()

	job := getTaskByTaskId(taskId)
	if job == nil {
		return errors.New("taskId不存在")
	}

	defer flushConfig()
	job.Run = run

	if job.Run {
		if job.entityId != 0 {
			return errors.New("任务已注册")
		}
		entityId, err := c.AddFunc(job.Spec, func(job *Job) func() {
			return func() {
				execAction(job)
			}
		}(job))
		if err != nil {
			return err
		}
		job.entityId = entityId
	} else {
		if job.entityId == 0 {
			return nil
		}
		c.Remove(job.entityId)
		job.entityId = 0
	}
	return nil
}

func RunTask(taskId string) error {
	task := getTaskByTaskId(taskId)
	if task == nil {
		return errors.New("taskId不存在")
	}
	return task.RunOnce()
}

func SaveTask(job JobStatusShow) error {
	needFlush := false
	defer func() {
		if needFlush {
			flushConfig()
		}
	}()
	if job.UUID == "" {
		job.UUID = generateUUID()
		newJob := Job{
			JobSpec: JobSpec{
				UUID:    job.UUID,
				JobName: job.JobName,
				Link:    job.Link,
				Type:    JobType(job.Type),
				Run:     job.Run,
				BinPath: job.BinPath,
				Dir:     job.Dir,
				Spec:    job.Spec,
				Options: job.Options,
			},
		}
		newJob.ConfigInit()
		jobConfigV2.TaskList = append(jobConfigV2.TaskList, &newJob)
		needFlush = true
	} else {
		for _, jobItem := range jobConfigV2.TaskList {
			if jobItem.UUID == job.UUID {
				if jobItem.Run {
					return errors.New("任务处于开启状态不允许修改,如需修改请先关闭")
				}
				if jobItem.Type != JobType(job.Type) {
					return errors.New("任务类型不允许修改")
				}
				jobItem.JobName = job.JobName
				jobItem.Run = job.Run
				jobItem.BinPath = job.BinPath
				jobItem.Dir = job.Dir
				jobItem.Spec = job.Spec
				jobItem.Options = job.Options
				jobItem.Link = job.Link
				needFlush = true
			}
		}
	}
	return nil
}

func RemoveTask(job JobStatusShow) error {
	needFlush := false
	removed := false
	defer func() {
		if needFlush {
			flushConfig()
		}
	}()

	for i, jobItem := range jobConfigV2.TaskList {
		if jobItem.UUID == job.UUID {
			if jobItem.Run {
				return errors.New("任务处于开启状态不允许修改,如需修改请先关闭")
			}
			needFlush = true
			jobConfigV2.TaskList = append(jobConfigV2.TaskList[0:i], jobConfigV2.TaskList[i+1:]...)
			removed = true
			break
		}
	}
	if !removed {
		return errors.New("任务不存在或已删除")
	}
	return nil
}
