package jobmanager

import (
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestConfigInit_MergeDefaults(t *testing.T) {
	jobConfigV2.Config.DefaultOptions = RunOptions{OutputType: OutputTypeStd, OutputPath: "", MaxFailures: 5, MinRunSeconds: 3, ShellPath: "/bin/bash"}
	j := &Job{UUID: generateUUID(), JobName: "merge", Type: JobTypeResident}
	j.ConfigInit()
	if j.Options.MaxFailures != 5 || j.Options.MinRunSeconds != 3 || j.Options.ShellPath == "" {
		t.Fatalf("defaults not merged: %+v", j.Options)
	}
}

func TestGetResidentScheduled(t *testing.T) {
	jobConfigV2.TaskList = []*Job{
		{UUID: generateUUID(), JobName: "r1", Type: JobTypeResident},
		{UUID: generateUUID(), JobName: "s1", Type: JobTypeScheduled},
	}
	if len(jobConfigV2.GetResidentTask()) != 1 || len(jobConfigV2.GetScheduledTask()) != 1 {
		t.Fatal("filter not working")
	}
}

func TestRegV2_WithJSON(t *testing.T) {
	conf := JobConfigV2{
		TaskList: []*Job{
			{UUID: generateUUID(), JobName: "res", Type: JobTypeResident, Run: false, BinPath: "/bin/echo ok"},
			{UUID: generateUUID(), JobName: "sch", Type: JobTypeScheduled, Run: true, Spec: "* * * * *", BinPath: "/bin/echo ok"},
		},
		Config: BaseConfig{DefaultOptions: RunOptions{OutputType: OutputTypeStd}},
	}
	b, _ := json.Marshal(conf)
	RegV2(b)
	c.Stop()
}

func TestScheduleV2_InvalidSpec(t *testing.T) {
	job := &Job{UUID: generateUUID(), JobName: "bad-spec", Type: JobTypeScheduled, Run: true, Spec: "", BinPath: "/bin/echo ok"}
	scheduleV2([]*Job{job})
	c.Stop()
}

func TestRegV2_InvalidJSON(t *testing.T) {
	RegV2([]byte("{"))
}

func TestStopAll(t *testing.T) {
	j := &Job{UUID: generateUUID(), JobName: "res-stp", Type: JobTypeResident, Run: true, BinPath: "/bin/echo bye"}
	j.ConfigInit()
	_ = j.JobInit()
	StopAll()
	StartOpen()
}

func TestStopJob_TimeoutKillPath(t *testing.T) {
	j := &Job{UUID: generateUUID(), JobName: "kill-path", Type: JobTypeResident, Run: true, BinPath: "trap '' INT; sleep 10", Options: RunOptions{ShellPath: "/bin/bash"}}
	j.ConfigInit()
	if err := j.JobInit(); err != nil {
		t.Fatalf("JobInit err: %v", err)
	}
	// ensure process started
	time.Sleep(100 * time.Millisecond)
	j.StopJob(true)
}

func TestRemoveTask_RunningError(t *testing.T) {
	jobConfigV2.TaskList = []*Job{{UUID: "rid", JobName: "r", Type: JobTypeScheduled, Run: true}}
	err := RemoveTask(JobStatusShow{UUID: "rid"})
	if err == nil {
		t.Fatal("expected error for running task remove")
	}
}

func TestRunStartTimeAndGetRunTime(t *testing.T) {
	if RunStartTime().IsZero() {
		t.Fatal("RunStartTime zero")
	}
	if GetRunTime() <= 0 {
		t.Fatal("GetRunTime not positive")
	}
}

func TestRegByUserConfigAndFlush(t *testing.T) {
	tmp := t.TempDir()
	old := userHomeDirFn
	userHomeDirFn = func() (string, error) { return tmp, nil }
	defer func() { userHomeDirFn = old }()
	if err := RegByUserConfig(); err != nil {
		t.Fatalf("RegByUserConfig err: %v", err)
	}
	// ensure file exists
	p, _ := getConfigPath()
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("config not written: %v", err)
	}
	if err := flushConfig(); err != nil {
		t.Fatalf("flush err: %v", err)
	}
}

func TestGenerateDefaultJobConfigContent(t *testing.T) {
	def := generateDefaultJobConfig()
	if len(def.TaskList) == 0 {
		t.Fatal("default config empty")
	}
	found := false
	for _, j := range def.TaskList {
		if j.Type == JobTypeResident && j.Run && j.JobName == "echo-loop" {
			found = true
			if runtime.GOOS == "windows" {
				if !strings.Contains(j.BinPath, "for /l") {
					t.Fatalf("windows loop not set: %+v", j)
				}
			} else {
				if !strings.Contains(j.BinPath, "while true") {
					t.Fatalf("unix loop not set: %+v", j)
				}
			}
		}
	}
	if !found {
		t.Fatal("echo-loop resident task missing")
	}
}

func TestJobRunOnce_DoubleCall(t *testing.T) {
	j := &Job{UUID: generateUUID(), JobName: "once", Type: JobTypeScheduled, BinPath: "/bin/echo x"}
	j.ConfigInit()
	if err := j.RunOnce(); err != nil {
		t.Fatalf("first runonce err: %v", err)
	}
	if err := j.RunOnce(); err == nil {
		t.Fatalf("second runonce should err")
	}
}

func TestForceRunJob_Init(t *testing.T) {
	j := &Job{UUID: generateUUID(), JobName: "force", Type: JobTypeResident, BinPath: "/bin/echo x"}
	j.ConfigInit()
	if err := j.ForceRunJob(); err != nil {
		t.Fatalf("ForceRunJob err: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	j.StopJob(true)
}

func TestGetHttpConfig(t *testing.T) {
	jobConfigV2.Config.DefaultOptions.OutputType = OutputTypeStd
	cfg := GetHttpConfig()
	if cfg.DefaultOptions.OutputType == 0 {
		t.Fatal("http config invalid")
	}
}

func TestJobListAndGetByIdAndRemove(t *testing.T) {
	jobConfigV2.TaskList = []*Job{}
	s := JobStatusShow{UUID: "", JobName: "j1", Type: int(JobTypeResident), Run: false, BinPath: "/bin/echo"}
	if err := SaveTask(s); err != nil {
		t.Fatalf("SaveTask err: %v", err)
	}
	list := JobList()
	if len(list) == 0 {
		t.Fatal("JobList empty")
	}
	id := list[0].UUID
	if getJobByJobId(id) == nil {
		t.Fatal("getJobByJobId nil")
	}
	// update
	s.UUID = id
	s.JobName = "j1-upd"
	if err := SaveTask(s); err != nil {
		t.Fatalf("SaveTask update err: %v", err)
	}
	// remove
	if err := RemoveTask(s); err != nil {
		t.Fatalf("RemoveTask err: %v", err)
	}
}

func TestResidentRunStopNonExist(t *testing.T) {
	if err := JobRunResidentTask("no-such"); err == nil {
		t.Fatal("expected error")
	}
	if err := JobStopResidentTask("no-such"); err == nil {
		t.Fatal("expected error")
	}
}
