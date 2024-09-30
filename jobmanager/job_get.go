package jobmanager

import (
	"errors"
	"log/slog"
	"time"
)

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
	for _, job := range jobConfigV2.GetResidentTask() {
		jobNameList = append(jobNameList, job2jobStatus(*job))
	}
	for _, job := range jobConfigV2.GetScheduledTask() {
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

func JobRun(jobId string) error {
	jh := getJobByJobId(jobId)
	if jh == nil {
		return errors.New("jobId不存在")
	}
	return jh.ForceRunJob()
}

func JobStop(jobId string) error {
	jh := getJobByJobId(jobId)
	if jh == nil {
		return errors.New("jobId不存在")
	}
	jh.StopJob()
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

func RunTask(taskId string) error {
	task := getTaskByTaskId(taskId)
	if task == nil {
		return errors.New("taskId不存在")
	}
	return task.RunOnce()
}
