package jobmanager

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

func createTestManager() *Manager {
	return &Manager{
		config: JobConfig{
			Config: BaseConfig{
				DefaultOptions: RunOptions{OutputType: OutputTypeFile, OutputPath: "/tmp", MaxFailures: 5},
			},
		},
		cron:      cron.New(),
		startTime: time.Now(),
	}
}

func mockHomeDir(t *testing.T) (string, func()) {
	tmpDir := t.TempDir()
	oldHome := userHomeDirFn
	userHomeDirFn = func() (string, error) { return tmpDir, nil }
	return tmpDir, func() { userHomeDirFn = oldHome }
}

func TestExecActionSetsStatusAndObservables(t *testing.T) {
	m := createTestManager()
	tmpDir, cleanup := mockHomeDir(t)
	defer cleanup()

	logDir, _ := getLogDir()
	job := Job{
		JobSpec: JobSpec{
			UUID:    generateUUID(),
			JobName: "test-echo",
			Type:    2,
			Run:     true,
			BinPath: "/bin/echo ok",
			Dir:     tmpDir,
			Options: RunOptions{OutputType: OutputTypeFile, OutputPath: logDir},
		},
	}
	m.ConfigInit(&job)
	m.execAction(&job)
	if job.status != Stop {
		t.Fatalf("status not Stop: %v", job.status)
	}
	if job.LastExitCode != 0 {
		t.Fatalf("exit code not 0: %v", job.LastExitCode)
	}
	if job.LastDuration <= 0 {
		t.Fatalf("duration not positive: %v", job.LastDuration)
	}
	if _, err := os.Stat(filepath.Join(logDir, job.JobName+"_log.txt")); err != nil {
		t.Fatalf("log file missing: %v", err)
	}
}

func TestGenerateUUID_ErrorBranch(t *testing.T) {
	old := uuidGen
	uuidGen = func() (uuid.UUID, error) { return uuid.UUID{}, fmt.Errorf("err") }
	defer func() { uuidGen = old }()
	s := generateUUID()
	if len(s) == 0 {
		t.Fatalf("uuid fallback empty")
	}
}

func TestGetConfigPath_ErrorHomeDir(t *testing.T) {
	old := userHomeDirFn
	userHomeDirFn = func() (string, error) { return "", fmt.Errorf("err") }
	defer func() { userHomeDirFn = old }()
	p, err := getConfigPath()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(p) == 0 {
		t.Fatalf("config path empty")
	}
}

func TestScheduleV2_StartAndStop(t *testing.T) {
	m := createTestManager()
	_, cleanup := mockHomeDir(t)
	defer cleanup()

	// ensure cron starts and can be stopped
	job := &Job{JobSpec: JobSpec{UUID: generateUUID(), JobName: "cron-echo", Type: JobTypeScheduled, Run: true, BinPath: "/bin/echo hi"}}
	m.scheduleV2([]*Job{job})
	// stop immediately
	m.cron.Stop()
}

func TestJobGuard_FailureBackoffWithoutSleep(t *testing.T) {
	m := createTestManager()
	_, cleanup := mockHomeDir(t)
	defer cleanup()

	// replace sleep to speed up
	old := sleepFn
	sleepFn = func(d time.Duration) {}
	defer func() { sleepFn = old }()

	j := &Job{JobSpec: JobSpec{UUID: generateUUID(), JobName: "fast-exit", Type: JobTypeResident, Run: true, BinPath: "/bin/echo x", Options: RunOptions{MaxFailures: 1, MinRunSeconds: int(maxExecutionTime.Seconds()) + 1}}}
	m.ConfigInit(j)
	// make command exit quickly
	done := make(chan struct{})
	go func() { m.runResidentJobLoop(j); close(done) }()
	// allow guard to run once and stop due to MaxFailures
	<-done
}

func TestStopJobCancelsContext(t *testing.T) {
	m := createTestManager()
	tmpDir, cleanup := mockHomeDir(t)
	defer cleanup()

	j := &Job{JobSpec: JobSpec{UUID: generateUUID(), JobName: "stop-sleep", Type: JobTypeResident, Run: true, BinPath: "sleep 2", Dir: tmpDir, Options: RunOptions{ShellPath: "/bin/bash"}}}
	m.ConfigInit(j)
	if err := m.StartResidentJob(j); err != nil {
		t.Fatalf("JobInit err: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	m.StopJob(j)

	// Wait a bit for goroutine to exit
	time.Sleep(100 * time.Millisecond)
}

func TestClosedAndStartClose_Last(t *testing.T) {
	m := createTestManager()
	DefaultManager = m
	// place last to avoid affecting other tests
	StartClose()
	if !Closed() {
		t.Fatalf("Closed not true after StartClose")
	}
}

func TestRuntimeLogPathInit(t *testing.T) {
	m := createTestManager()
	tmpDir := t.TempDir()

	job := &Job{
		JobSpec: JobSpec{
			UUID:    generateUUID(),
			JobName: "test-logpath",
			Options: RunOptions{
				OutputType: OutputTypeFile,
				OutputPath: tmpDir,
			},
		},
	}

	// ConfigInit should set runtimeLogPath
	m.ConfigInit(job)

	// Verify runtimeLogPath is set
	if job.runtimeLogPath == "" {
		t.Fatal("runtimeLogPath should be set by ConfigInit")
	}

	expectedPath := filepath.Join(tmpDir, "test-logpath_log.txt")
	if job.runtimeLogPath != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, job.runtimeLogPath)
	}

	// Verify ToStatusShow uses it
	status := job.ToStatusShow()
	if status.RealLogPath != expectedPath {
		t.Errorf("ToStatusShow RealLogPath expected %s, got %s", expectedPath, status.RealLogPath)
	}
}
