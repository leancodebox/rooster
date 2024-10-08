package jobmanager

import (
	"github.com/robfig/cron/v3"
	"os/exec"
	"sync"
	"time"
)

var runStatusName = [...]string{
	"停止",
	"运行",
}

func (d RunStatus) String() string {
	if d < Stop || d > Running {
		return "Unknown"
	}
	return runStatusName[d]
}

const maxExecutionTime = 10 * time.Second // 最大允许的运行时间
const maxConsecutiveFailures = 3          // 连续失败次数的最大值

type OutputType int

const (
	OutputTypeStd  OutputType = iota + 1 // 输出到标准输入输出
	OutputTypeFile                       // 输出到文件
)

type RunOptions struct {
	OutputType  OutputType `json:"outputType"`  // 输出方式
	OutputPath  string     `json:"outputPath"`  // 输出路径
	MaxFailures int        `json:"maxFailures"` // 最大失败次数
}

type Job struct {
	UUID    string     `json:"uuid"`
	JobName string     `json:"jobName"`
	Type    int        `json:"type"` //运行模式 0 常驻 1 定时
	Run     bool       `json:"run"`
	BinPath string     `json:"binPath"`
	Params  []string   `json:"params"`
	Dir     string     `json:"dir"`
	Spec    string     `json:"spec"`
	Options RunOptions `json:"options"` // 运行选项

	status   RunStatus
	confLock *sync.Mutex
	cmd      *exec.Cmd

	entityId    cron.EntryID
	runOnceLock *sync.Mutex
}

type BaseConfig struct {
	Dashboard struct {
		Port int `json:"port"`
	} `json:"dashboard"`
	DefaultOptions RunOptions `json:"defaultOptions"` // 运行选项
}

type JobConfigV2 struct {
	TaskList []*Job     `json:"taskList"`
	Config   BaseConfig `json:"config"`
}

func (itself *JobConfigV2) GetResidentTask() []*Job {
	var r []*Job
	for _, item := range itself.TaskList {
		if item.Type == 1 {
			r = append(r, item)
		}
	}
	return r
}
func (itself *JobConfigV2) GetScheduledTask() []*Job {
	var r []*Job
	for _, item := range itself.TaskList {
		if item.Type == 2 {
			r = append(r, item)
		}
	}
	return r
}
