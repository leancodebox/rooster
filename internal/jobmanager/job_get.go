package jobmanager

import (
	"errors"
	"log/slog"
	"os"
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
	RealLogPath string `json:"realLogPath"`
	LogSize     int64  `json:"size"`
	LogModTime  string `json:"modTime"`
}

// ToStatusShow 转换为对外展示的状态结构
func (job *Job) ToStatusShow() JobStatusShow {
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

	// 填充日志信息
	if job.runtimeLogPath != "" {
		js.RealLogPath = job.runtimeLogPath
	}

	// 如果路径存在，检查文件状态
	if js.RealLogPath != "" {
		if st, err := os.Stat(js.RealLogPath); err == nil && !st.IsDir() {
			js.LogSize = st.Size()
			js.LogModTime = st.ModTime().Format("2006-01-02 15:04:05")
		}
	}

	return js
}

func (m *Manager) JobList() []JobStatusShow {
	var jobNameList []JobStatusShow
	for _, job := range m.config.TaskList {
		jobNameList = append(jobNameList, job.ToStatusShow())
	}
	return jobNameList
}

func JobList() []JobStatusShow {
	if DefaultManager != nil {
		return DefaultManager.JobList()
	}
	return nil
}

func (m *Manager) getJobByJobId(uuId string) *Job {
	return m.config.GetJob(uuId)
}

func (m *Manager) JobRunResidentTask(jobId string) error {
	defer m.flushConfig()
	jh := m.getJobByJobId(jobId)
	if jh == nil {
		return errors.New("jobId不存在")
	}
	return m.ForceRunJob(jh)
}

func JobRunResidentTask(jobId string) error {
	if DefaultManager != nil {
		return DefaultManager.JobRunResidentTask(jobId)
	}
	return errors.New("manager not initialized")
}

func (m *Manager) JobStopResidentTask(jobId string) error {
	jh := m.getJobByJobId(jobId)
	if jh == nil {
		return errors.New("jobId不存在")
	}
	defer m.flushConfig()
	m.StopJob(jh)
	return nil
}

func JobStopResidentTask(jobId string) error {
	if DefaultManager != nil {
		return DefaultManager.JobStopResidentTask(jobId)
	}
	return errors.New("manager not initialized")
}

func (m *Manager) StopAll() {
	m.StartClose()
	wg := sync.WaitGroup{}
	for _, item := range m.config.GetResidentTask() {
		slog.Info(item.JobName + "准备退出")
		wg.Go(func(job *Job) func() {
			return func() {
				m.StopJob(job)
				slog.Info(job.JobName + "退出")
			}
		}(item))
	}
	wg.Wait()
}

func StopAll() {
	if DefaultManager != nil {
		DefaultManager.StopAll()
	}
}

func (m *Manager) GetHttpConfig() BaseConfig {
	return m.config.Config
}

func GetHttpConfig() BaseConfig {
	if DefaultManager != nil {
		return DefaultManager.GetHttpConfig()
	}
	return BaseConfig{}
}

func (m *Manager) getTaskByTaskId(uuId string) *Job {
	return m.config.GetJob(uuId)
}

func (m *Manager) OpenCloseTask(taskId string, run bool) error {
	m.taskStatusLock.Lock()
	defer m.taskStatusLock.Unlock()

	job := m.getTaskByTaskId(taskId)
	if job == nil {
		return errors.New("taskId不存在")
	}

	defer m.flushConfig()
	job.Run = run

	if job.Run {
		if job.entityId != 0 {
			return errors.New("任务已注册")
		}
		entityId, err := m.cron.AddFunc(job.Spec, func(job *Job) func() {
			return func() {
				m.execAction(job)
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
		m.cron.Remove(job.entityId)
		job.entityId = 0
	}
	return nil
}

func OpenCloseTask(taskId string, run bool) error {
	if DefaultManager != nil {
		return DefaultManager.OpenCloseTask(taskId, run)
	}
	return errors.New("manager not initialized")
}

func (m *Manager) RunTask(taskId string) error {
	task := m.getTaskByTaskId(taskId)
	if task == nil {
		return errors.New("taskId不存在")
	}
	return m.RunScheduledJob(task)
}

func RunTask(taskId string) error {
	if DefaultManager != nil {
		return DefaultManager.RunTask(taskId)
	}
	return errors.New("manager not initialized")
}

func (m *Manager) SaveTask(job JobStatusShow) error {
	needFlush := false
	defer func() {
		if needFlush {
			m.flushConfig()
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
		m.ConfigInit(&newJob)
		m.config.AddJob(&newJob)
		needFlush = true
	} else {
		jobItem := m.config.GetJob(job.UUID)
		if jobItem != nil {
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
	return nil
}

func SaveTask(job JobStatusShow) error {
	if DefaultManager != nil {
		return DefaultManager.SaveTask(job)
	}
	return errors.New("manager not initialized")
}

func (m *Manager) RemoveTask(job JobStatusShow) error {
	defer m.flushConfig()

	jobItem := m.config.GetJob(job.UUID)
	if jobItem == nil {
		return errors.New("任务不存在或已删除")
	}

	if jobItem.Run {
		return errors.New("任务处于开启状态不允许修改,如需修改请先关闭")
	}

	if m.config.RemoveJob(job.UUID) {
		return nil
	}
	return errors.New("任务删除失败")
}

func RemoveTask(job JobStatusShow) error {
	if DefaultManager != nil {
		return DefaultManager.RemoveTask(job)
	}
	return errors.New("manager not initialized")
}
