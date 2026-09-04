//go:build !windows

package engine

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/leancodebox/rooster/internal/domain"
	"github.com/leancodebox/rooster/internal/runner"
	storesqlite "github.com/leancodebox/rooster/internal/store/sqlite"
)

func TestManualExecutionCanBeStoppedAndRecorded(t *testing.T) {
	dir := t.TempDir()
	st, err := storesqlite.Open(filepath.Join(dir, "rooster.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	eng := New(st, runner.New(runner.NewEnvironmentProvider()), filepath.Join(dir, "logs"))
	if err := eng.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	task, err := eng.CreateTask(context.Background(), domain.Task{Name: "worker", Kind: domain.TaskKindResident, CommandMode: domain.CommandModeShell, Command: "sleep 30", OverlapPolicy: domain.OverlapSkip, RestartPolicy: domain.RestartNever})
	if err != nil {
		t.Fatal(err)
	}
	execution, err := eng.Run(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.StopExecution(execution.ID); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		items, err := eng.ListExecutions(context.Background(), task.ID, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) > 0 && items[0].Status == domain.ExecutionCanceled {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	items, err := eng.ListExecutions(context.Background(), task.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Status != domain.ExecutionCanceled {
		t.Fatalf("unexpected executions: %#v", items)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := eng.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestRejectsInvalidCronBeforePersistence(t *testing.T) {
	dir := t.TempDir()
	st, err := storesqlite.Open(filepath.Join(dir, "rooster.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	eng := New(st, runner.New(runner.NewEnvironmentProvider()), filepath.Join(dir, "logs"))
	_, err = eng.CreateTask(context.Background(), domain.Task{Name: "bad schedule", Kind: domain.TaskKindScheduled, CommandMode: domain.CommandModeShell, Command: "echo hi", Schedule: "not cron", OverlapPolicy: domain.OverlapSkip, RestartPolicy: domain.RestartNever})
	if err == nil {
		t.Fatal("expected cron validation error")
	}
	tasks, err := st.ListTasks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Fatalf("invalid task was persisted: %#v", tasks)
	}
}
