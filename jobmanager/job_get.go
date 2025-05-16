package jobmanager

import (
	"errors"
	"log/slog"
	"sync"
	"time"
)

// JobStatus job运行状态
type JobStatus struct {
	UUID    string     `json:"uuid"`
	JobName string     `json:"jobName"`
	Type    int        `json:"type"` //运行模式 0 常驻 1 定时
	Run     bool       `json:"run"`
	BinPath string     `json:"binPath"`
	Params  []string   `json:"params"`
	Dir     string     `json:"dir"`
	Spec    string     `json:"spec"`
	Options RunOptions `json:"options"` // 运行选项

	Status RunStatus `json:"status"`
}

// 类型转化
func job2jobStatus(job Job) JobStatus {
	return JobStatus{
		UUID:    job.UUID,
		JobName: job.JobName,
		Type:    job.Type,
		Run:     job.Run,
		BinPath: job.BinPath,
		Params:  job.Params,
		Dir:     job.Dir,
		Spec:    job.Spec,
		Options: job.Options,
		Status:  job.status,
	}
}

func JobList() []JobStatus {
	var jobNameList []JobStatus
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
	jh.StopJob(true)
	return nil
}

func StopAll() {
	for _, item := range jobConfigV2.GetResidentTask() {
		item.StopJob()
		slog.Info(item.JobName + "退出")
	}
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
				execAction(*job)
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

func SaveTask(job JobStatus) error {
	needFlush := false
	defer func() {
		if needFlush == true {
			err := flushConfig()
			if err != nil {
				slog.Error("flushConfig", "err", err)
			}
		}
	}()
	if job.UUID == "" {
		job.UUID = generateUUID()
		newJob := Job{
			UUID:    job.UUID,
			JobName: job.JobName,
			Type:    job.Type,
			Run:     job.Run,
			BinPath: job.BinPath,
			Params:  job.Params,
			Dir:     job.Dir,
			Spec:    job.Spec,
			Options: job.Options,
		}
		newJob.ConfigInit()
		jobConfigV2.TaskList = append(jobConfigV2.TaskList, &newJob)
		needFlush = true
	} else {
		for _, jobItem := range jobConfigV2.TaskList {
			if jobItem.UUID == job.UUID {
				if jobItem.Run == true {
					return errors.New("任务处于开启状态不允许修改,如需修改请先关闭")
				}
				if jobItem.Type != jobItem.Type {
					return errors.New("任务类型不允许修改")
				}
				jobItem.JobName = job.JobName
				jobItem.Run = job.Run
				jobItem.BinPath = job.BinPath
				jobItem.Params = job.Params
				jobItem.Dir = job.Dir
				jobItem.Spec = job.Spec
				jobItem.Options = job.Options
				needFlush = true
			}
		}
	}
	return nil
}

func RemoveTask(job JobStatus) error {
	needFlush := false
	defer func() {
		if needFlush == true {
			err := flushConfig()
			if err != nil {
				slog.Error("flushConfig", "err", err)
			}
		}
	}()

	for i, jobItem := range jobConfigV2.TaskList {
		if jobItem.UUID == job.UUID {
			if jobItem.Run == true {
				return errors.New("任务处于开启状态不允许修改,如需修改请先关闭")
			}
			needFlush = true
			jobConfigV2.TaskList = append(jobConfigV2.TaskList[0:i], jobConfigV2.TaskList[i+1:]...)
			break
		}
	}
	return nil
}
