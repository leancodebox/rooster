package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/leancodebox/rooster/internal/domain"
)

func TestTaskAndExecutionLifecycle(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "rooster.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	task := domain.Task{ID: "task-1", Name: "worker", Kind: domain.TaskKindResident, CommandMode: domain.CommandModeShell, Command: "echo ready", Environment: map[string]string{"MODE": "test"}, OverlapPolicy: domain.OverlapSkip, RestartPolicy: domain.RestartOnFailure, MaxRetries: 3, MinRunSeconds: 10}
	task, err = s.CreateTask(ctx, task)
	if err != nil {
		t.Fatal(err)
	}
	task.Enabled = true
	if _, err := s.UpdateTask(ctx, task); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Enabled || got.Environment["MODE"] != "test" {
		t.Fatalf("unexpected task: %#v", got)
	}
	execution := domain.Execution{ID: "exec-1", TaskID: task.ID, Trigger: domain.TriggerManual, Status: domain.ExecutionQueued, LogPath: "run.log"}
	if _, err := s.CreateExecution(ctx, execution); err != nil {
		t.Fatal(err)
	}
	runs, err := s.ListExecutions(ctx, task.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].ID != execution.ID {
		t.Fatalf("unexpected executions: %#v", runs)
	}
}

func TestMarkInterruptedExecutions(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "rooster.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	task := domain.Task{ID: "task", Name: "worker", Kind: domain.TaskKindResident, CommandMode: domain.CommandModeShell, Command: "sleep 10", OverlapPolicy: domain.OverlapSkip, RestartPolicy: domain.RestartNever}
	if _, err := s.CreateTask(ctx, task); err != nil {
		t.Fatal(err)
	}
	execution := domain.Execution{ID: "run", TaskID: task.ID, Trigger: domain.TriggerManual, Status: domain.ExecutionRunning, LogPath: "run.log"}
	if _, err := s.CreateExecution(ctx, execution); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkInterruptedExecutions(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetExecution(ctx, execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.ExecutionFailed || got.FinishedAt == nil {
		t.Fatalf("unexpected execution: %#v", got)
	}
}

func TestEmptyCollectionsAreNonNil(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "rooster.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	tasks, err := s.ListTasks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if tasks == nil || len(tasks) != 0 {
		t.Fatalf("tasks = %#v, want empty non-nil slice", tasks)
	}
	executions, err := s.ListExecutions(ctx, "missing", 10)
	if err != nil {
		t.Fatal(err)
	}
	if executions == nil || len(executions) != 0 {
		t.Fatalf("executions = %#v, want empty non-nil slice", executions)
	}
}
