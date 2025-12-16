package jobmanager

import (
	"context"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// 运行状态名映射
var runStatusName = [...]string{
	"停止",
	"运行",
}

// String 返回运行状态的字符串表示
func (d RunStatus) String() string {
	if d < Stop || d > Running {
		return "Unknown"
	}
	return runStatusName[d]
}

// 最大允许的运行时间与连续失败上限
const maxExecutionTime = 10 * time.Second
const maxConsecutiveFailures = 3

// OutputType 输出类型定义
type OutputType int

const (
	OutputTypeStd  OutputType = iota + 1 // 输出到标准输入输出
	OutputTypeFile                       // 输出到文件
)

// RunOptions 定义任务的运行选项
type RunOptions struct {
	OutputType    OutputType `json:"outputType"`  // 输出方式
	OutputPath    string     `json:"outputPath"`  // 输出路径
	MaxFailures   int        `json:"maxFailures"` // 最大失败次数
	ShellPath     string     `json:"shellPath"`
	MinRunSeconds int        `json:"minRunSeconds"`
}

// JobType 表示任务类型（常驻或定时）
type JobType int

const (
	JobTypeResident  JobType = 1
	JobTypeScheduled JobType = 2
)

// JobSpec 定义任务的静态配置
type JobSpec struct {
	UUID    string     `json:"uuid"`
	JobName string     `json:"jobName"`
	Link    string     `json:"link"`
	Type    JobType    `json:"type"` // 运行模式 1 常驻 / 2 定时
	Run     bool       `json:"run"`
	BinPath string     `json:"binPath"`
	Dir     string     `json:"dir"`
	Spec    string     `json:"spec"`
	Options RunOptions `json:"options"` // 运行选项
}

// RunStatus 运行状态
type RunStatus int

const (
	Stop    RunStatus = 0
	Running RunStatus = 1
)

// JobRuntime 定义任务的动态运行时状态
type JobRuntime struct {
	status   RunStatus
	confLock *sync.Mutex
	cancel   context.CancelFunc

	entityId    cron.EntryID
	runOnceLock *sync.Mutex

	Pid         int  `json:"-"`
	RunningLoop bool `json:"-"`

	LastStart    time.Time     `json:"-"`
	LastExit     time.Time     `json:"-"`
	LastExitCode int           `json:"-"`
	LastDuration time.Duration `json:"-"`

	runtimeLogPath string
}

// Job 表示任务及其运行时状态
type Job struct {
	JobSpec
	JobRuntime
}

// BaseConfig 为全局配置
type BaseConfig struct {
	Dashboard struct {
		Port int `json:"port"`
	} `json:"dashboard"`
	DefaultOptions RunOptions `json:"defaultOptions"` // 运行选项
}

// JobConfig 是任务列表与配置的组合
type JobConfig struct {
	TaskList []*Job     `json:"taskList"`
	Config   BaseConfig `json:"config"`
}

// GetResidentTask 返回常驻任务列表
func (itself *JobConfig) GetResidentTask() []*Job {
	var r []*Job
	for _, item := range itself.TaskList {
		if item.Type == JobTypeResident {
			r = append(r, item)
		}
	}
	return r
}

// GetScheduledTask 返回定时任务列表
func (itself *JobConfig) GetScheduledTask() []*Job {
	var r []*Job
	for _, item := range itself.TaskList {
		if item.Type == JobTypeScheduled {
			r = append(r, item)
		}
	}
	return r
}

func (itself *JobConfig) GetJob(uuid string) *Job {
	for _, job := range itself.TaskList {
		if job.UUID == uuid {
			return job
		}
	}
	return nil
}

func (itself *JobConfig) AddJob(job *Job) {
	itself.TaskList = append(itself.TaskList, job)
}

func (itself *JobConfig) RemoveJob(uuid string) bool {
	for i, job := range itself.TaskList {
		if job.UUID == uuid {
			itself.TaskList = append(itself.TaskList[:i], itself.TaskList[i+1:]...)
			return true
		}
	}
	return false
}
