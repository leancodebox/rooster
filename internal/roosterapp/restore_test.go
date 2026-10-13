package roosterapp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/leancodebox/rooster/internal/domain"
	storesqlite "github.com/leancodebox/rooster/internal/store/sqlite"
)

func TestRestoreDisablesTasksAndIsolatesLogPaths(t *testing.T) {
	dir := t.TempDir()
	source, err := storesqlite.Open(filepath.Join(dir, "source.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	ctx := context.Background()
	task := domain.Task{ID: uuid.NewString(), Name: "source", Enabled: true, Kind: domain.TaskKindResident, CommandMode: domain.CommandModeShell, Command: "echo do-not-run", OverlapPolicy: domain.OverlapSkip, RestartPolicy: domain.RestartAlways}
	if _, err := source.CreateTask(ctx, task); err != nil {
		t.Fatal(err)
	}
	execution := domain.Execution{ID: uuid.NewString(), TaskID: task.ID, Trigger: domain.TriggerManual, Status: domain.ExecutionRunning, LogPath: filepath.Join(dir, "original.log")}
	if _, err := source.CreateExecution(ctx, execution); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(dir, "snapshot.db")
	if err := source.Backup(ctx, backup); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "restored")
	if err := RestoreSnapshot(ctx, backup, target); err != nil {
		t.Fatal(err)
	}
	restored, err := storesqlite.Open(filepath.Join(target, "rooster.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	gotTask, err := restored.GetTask(ctx, task.ID)
	if err != nil || gotTask.Enabled {
		t.Fatalf("restored task enabled: %+v %v", gotTask, err)
	}
	gotExecution, err := restored.GetExecution(ctx, execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(target, "logs", task.ID, execution.ID+".log")
	if gotExecution.LogPath != wantPath || gotExecution.Status != domain.ExecutionFailed || gotExecution.FinishedAt == nil {
		t.Fatalf("unexpected restored execution: %+v", gotExecution)
	}
	original, err := source.GetTask(ctx, task.ID)
	if err != nil || !original.Enabled {
		t.Fatalf("source task was modified: %+v %v", original, err)
	}
	if err := RestoreSnapshot(ctx, backup, target); err == nil {
		t.Fatal("restore overwrote an existing directory")
	}
}

func TestRestoreRejectsInvalidBackupWithoutLeavingTarget(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "invalid.db")
	if err := os.WriteFile(source, []byte("not a database"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "restored")
	if err := RestoreSnapshot(context.Background(), source, target); err == nil {
		t.Fatal("accepted corrupt backup")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("failed restore left a target: %v", err)
	}
	sentinel := filepath.Join(dir, "sentinel")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RestoreSnapshot(context.Background(), source, dir); err == nil {
		t.Fatal("accepted existing destination")
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "keep" {
		t.Fatal("existing destination was modified")
	}
}
