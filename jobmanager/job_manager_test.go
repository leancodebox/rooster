package jobmanager

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestName(t *testing.T) {
	//ctx := context.Background()
	//cmd := exec.CommandContext(ctx,"echo","1212")

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		select {
		case <-ctx.Done():
			t.Log("done")
		}
	}()

	t.Log("waitCancel")
	cancel()
	t.Log("123454321")
	time.Sleep(time.Second * 3)
}

func TestSH(t *testing.T) {

	sendFromAdmin := make(chan bool)
	defer func() {
		close(sendFromAdmin)
	}()
	cmdIsLife := true
	// 外层的 gochannel 负责重启
	go func() {
		sendToParent := make(chan bool)
		defer func() {
			cmdIsLife = false
			close(sendToParent)
		}()
		consecutiveFailures := 0
		for {
			// build start
			unitStartTime := time.Now()
			ctx, cancel := context.WithCancel(context.Background())
			cmd := exec.CommandContext(ctx, "/opt/homebrew/bin/php", "/Users/thh/workspace/about/tmp/p.php")
			cmd.Stdout = os.Stdout
			// build end
			go func() {
				defer func() {
					if err := recover(); err != nil {
						t.Log("panic", err)
					}
					sendToParent <- true
				}()
				// 准备启动
				if err := cmd.Start(); err != nil {
					slog.Error("cmd.start", "err", err.Error())
				}
				if err := cmd.Wait(); err != nil {
					slog.Error("cmd.start", "err", err.Error())
				}
			}()
			observe := false
			for {
				select {
				case <-sendFromAdmin:
					cancel()
				case <-sendToParent:
					observe = true
				}
				if observe {
					break
				}
			}
			executionTime := time.Since(unitStartTime)
			if executionTime <= maxExecutionTime {
				consecutiveFailures += 1
			} else {
				consecutiveFailures = 0
			}

			if consecutiveFailures >= maxConsecutiveFailures {
				msg := "程序连续3次启动失败，停止重启"
				slog.Info(msg)
				break
			} else {
				msg := "程序终止尝试重新运行"
				slog.Info(msg)
			}
		}

	}()
	slog.Info("adsa")

	time.Sleep(time.Second * 1)
	if cmdIsLife {
		sendFromAdmin <- true
	}
	time.Sleep(time.Second * 2)

	if cmdIsLife {
		sendFromAdmin <- true
	}
	time.Sleep(time.Second * 4)
}

func TestExecActionSetsStatusAndObservables(t *testing.T) {
	tmpDir := t.TempDir()
	job := Job{
		UUID:    generateUUID(),
		JobName: "test-echo",
		Type:    2,
		Run:     true,
		BinPath: "/bin/echo",
		Params:  []string{"ok"},
		Dir:     tmpDir,
		Options: RunOptions{OutputType: OutputTypeFile, OutputPath: tmpDir},
	}
	job.ConfigInit()
	execAction(&job)
	if job.status != Stop {
		t.Fatalf("status not Stop: %v", job.status)
	}
	if job.LastExitCode != 0 {
		t.Fatalf("exit code not 0: %v", job.LastExitCode)
	}
	if job.LastDuration <= 0 {
		t.Fatalf("duration not positive: %v", job.LastDuration)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, job.JobName+"_log.txt")); err != nil {
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
	// ensure cron starts and can be stopped
	job := &Job{UUID: generateUUID(), JobName: "cron-echo", Type: JobTypeScheduled, Run: true, BinPath: "/bin/echo", Params: []string{"hi"}}
	scheduleV2([]*Job{job})
	// stop immediately
	c.Stop()
}

func TestJobGuard_FailureBackoffWithoutSleep(t *testing.T) {
	// replace sleep to speed up
	old := sleepFn
	sleepFn = func(d time.Duration) {}
	defer func() { sleepFn = old }()

	j := &Job{UUID: generateUUID(), JobName: "fast-exit", Type: JobTypeResident, Run: true, BinPath: "/bin/echo", Params: []string{"x"}, Options: RunOptions{MaxFailures: 1, MinRunSeconds: int(maxExecutionTime.Seconds()) + 1}}
	j.ConfigInit()
	// make command exit quickly
	j.cmd = buildCmd(j)
	done := make(chan struct{})
	go func() { j.jobGuard(); close(done) }()
	// allow guard to run once and stop due to MaxFailures
	<-done
}

func TestStopJobCancelsContext(t *testing.T) {
	tmpDir := t.TempDir()
	j := &Job{UUID: generateUUID(), JobName: "stop-sleep", Type: JobTypeResident, Run: true, BinPath: "/bin/bash", Params: []string{"-lc", "sleep 2"}, Dir: tmpDir}
	j.ConfigInit()
	if err := j.JobInit(); err != nil {
		t.Fatalf("JobInit err: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	j.StopJob(true)
}

func TestClosedAndStartClose_Last(t *testing.T) {
	// place last to avoid affecting other tests
	StartClose()
	if !Closed() {
		t.Fatalf("Closed not true after StartClose")
	}
}
