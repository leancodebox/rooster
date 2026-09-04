//go:build !windows

package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/leancodebox/rooster/internal/domain"
)

func TestRunnerStopsAndReapsProcessTree(t *testing.T) {
	dir := t.TempDir()
	childFile := filepath.Join(dir, "child.pid")
	task := domain.Task{ID: "task", Name: "tree", Kind: domain.TaskKindResident, CommandMode: domain.CommandModeShell, Command: "sleep 30 & echo $! > '" + childFile + "'; wait", RestartPolicy: domain.RestartNever, OverlapPolicy: domain.OverlapSkip}
	r := New(NewEnvironmentProvider())
	r.stopGrace = 500 * time.Millisecond
	h, err := r.Start(context.Background(), task, filepath.Join(dir, "run.log"))
	if err != nil {
		t.Fatal(err)
	}
	var raw []byte
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		raw, _ = os.ReadFile(childFile)
		if len(raw) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	childPID, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("child pid: %v (%q)", err, raw)
	}
	h.Stop()
	select {
	case <-h.Done():
		result := h.Result()
		if !errors.Is(result.Err, context.Canceled) && result.ExitCode == 0 {
			t.Fatalf("unexpected result: %#v", result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runner did not stop")
	}
	if err := syscall.Kill(h.PID, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("parent process still exists: %v", err)
	}
	if err := syscall.Kill(childPID, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("child process still exists: %v", err)
	}
}

func TestRunnerUsesTaskEnvironmentAndWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "run.log")
	task := domain.Task{ID: "task", Name: "env", Kind: domain.TaskKindScheduled, CommandMode: domain.CommandModeShell, Command: `printf '%s:%s' "$ROOSTER_TEST" "$PWD"`, WorkingDir: dir, Environment: map[string]string{"ROOSTER_TEST": "ready"}, Schedule: "* * * * *", OverlapPolicy: domain.OverlapSkip, RestartPolicy: domain.RestartNever}
	h, err := New(NewEnvironmentProvider()).Start(context.Background(), task, logPath)
	if err != nil {
		t.Fatal(err)
	}
	result := h.Wait()
	if result.ExitCode != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := "ready:" + realDir
	if string(content) != want {
		t.Fatalf("log = %q, want %q", content, want)
	}
}
