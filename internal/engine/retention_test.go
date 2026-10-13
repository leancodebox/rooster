package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/leancodebox/rooster/internal/domain"
	"github.com/leancodebox/rooster/internal/runner"
	storesqlite "github.com/leancodebox/rooster/internal/store/sqlite"
)

func TestRetentionPreviewsAndProtectsRecentAndRunningExecutions(t *testing.T) {
	dir := t.TempDir()
	st, err := storesqlite.Open(filepath.Join(dir, "rooster.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	eng := New(st, runner.New(runner.NewEnvironmentProvider()), filepath.Join(dir, "logs"))
	task, err := eng.CreateTask(context.Background(), domain.Task{Name: "retention", Kind: domain.TaskKindResident, CommandMode: domain.CommandModeShell, Command: "echo test", OverlapPolicy: domain.OverlapSkip, RestartPolicy: domain.RestartNever})
	if err != nil {
		t.Fatal(err)
	}
	logDir := filepath.Join(dir, "logs", task.ID)
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().AddDate(0, 0, -60)
	create := func(status domain.ExecutionStatus, created time.Time) domain.Execution {
		t.Helper()
		id := uuid.NewString()
		execution := domain.Execution{ID: id, TaskID: task.ID, Status: status, Trigger: domain.TriggerManual, CreatedAt: created, LogPath: filepath.Join(logDir, id+".log")}
		if status != domain.ExecutionRunning {
			execution.FinishedAt = &created
		}
		if err := os.WriteFile(execution.LogPath, []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := st.CreateExecution(context.Background(), execution); err != nil {
			t.Fatal(err)
		}
		return execution
	}
	remove := create(domain.ExecutionSucceeded, old)
	archive := filepath.Join(logDir, remove.ID+"-2026-01-01T00-00-00.000.log.gz")
	if err := os.WriteFile(archive, []byte("archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	running := create(domain.ExecutionRunning, old.Add(time.Hour))
	recent := create(domain.ExecutionSucceeded, time.Now().UTC())
	preview, err := eng.Retain(context.Background(), 30, 1, true)
	if err != nil || preview.Candidates != 1 || preview.Files != 2 || preview.Bytes != 11 || preview.Deleted != 0 {
		t.Fatalf("preview=%+v error=%v", preview, err)
	}
	if _, err := os.Stat(remove.LogPath); err != nil {
		t.Fatal("preview deleted a log")
	}
	result, err := eng.Retain(context.Background(), 30, 1, false)
	if err != nil || result.Deleted != 1 || len(result.Errors) != 0 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	for _, execution := range []domain.Execution{running, recent} {
		if _, err := st.GetExecution(context.Background(), execution.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(execution.LogPath); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(archive); !os.IsNotExist(err) {
		t.Fatalf("archive survived: %v", err)
	}
}

func TestRetentionRefusesOutsideLogPath(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(dir, "outside.log")
	if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	eng := &Engine{logDir: filepath.Join(dir, "logs")}
	_, _, err := eng.retentionFiles(domain.Execution{ID: uuid.NewString(), TaskID: uuid.NewString(), LogPath: outside})
	if err == nil {
		t.Fatal("accepted external log path")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal(err)
	}
}
