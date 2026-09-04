package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/leancodebox/rooster/internal/domain"
	storepkg "github.com/leancodebox/rooster/internal/store"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL; PRAGMA busy_timeout = 5000;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("configure sqlite: %w", err)
	}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS tasks (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL CHECK (kind IN ('resident', 'scheduled')),
  enabled INTEGER NOT NULL DEFAULT 0,
  command_mode TEXT NOT NULL CHECK (command_mode IN ('shell', 'exec')),
  command TEXT NOT NULL,
  arguments_json TEXT NOT NULL DEFAULT '[]',
  working_dir TEXT NOT NULL DEFAULT '',
  shell TEXT NOT NULL DEFAULT '',
  environment_json TEXT NOT NULL DEFAULT '{}',
  schedule TEXT NOT NULL DEFAULT '',
  overlap_policy TEXT NOT NULL DEFAULT 'skip',
  restart_policy TEXT NOT NULL DEFAULT 'on_failure',
  max_retries INTEGER NOT NULL DEFAULT 3,
  min_run_seconds INTEGER NOT NULL DEFAULT 10,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS executions (
  id TEXT PRIMARY KEY,
  task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  trigger TEXT NOT NULL,
  status TEXT NOT NULL,
  pid INTEGER NOT NULL DEFAULT 0,
  started_at TEXT,
  finished_at TEXT,
  exit_code INTEGER,
  error TEXT NOT NULL DEFAULT '',
  log_path TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_executions_task_created
  ON executions(task_id, created_at DESC);
INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES (1, CURRENT_TIMESTAMP);`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}
	return nil
}

func (s *Store) ListTasks(ctx context.Context) ([]domain.Task, error) {
	rows, err := s.db.QueryContext(ctx, taskSelect+" ORDER BY created_at ASC")
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()
	tasks := make([]domain.Task, 0)
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (s *Store) GetTask(ctx context.Context, id string) (domain.Task, error) {
	t, err := scanTask(s.db.QueryRowContext(ctx, taskSelect+" WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Task{}, storepkg.ErrNotFound
	}
	return t, err
}

func (s *Store) CreateTask(ctx context.Context, t domain.Task) (domain.Task, error) {
	if err := t.Validate(); err != nil {
		return domain.Task{}, err
	}
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	args, _ := json.Marshal(t.Arguments)
	env, _ := json.Marshal(t.Environment)
	_, err := s.db.ExecContext(ctx, `INSERT INTO tasks
 (id,name,description,kind,enabled,command_mode,command,arguments_json,working_dir,shell,environment_json,schedule,overlap_policy,restart_policy,max_retries,min_run_seconds,created_at,updated_at)
 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, t.ID, t.Name, t.Description, t.Kind, t.Enabled, t.CommandMode, t.Command, args, t.WorkingDir, t.Shell, env, t.Schedule, t.OverlapPolicy, t.RestartPolicy, t.MaxRetries, t.MinRunSeconds, formatTime(t.CreatedAt), formatTime(t.UpdatedAt))
	if err != nil {
		return domain.Task{}, fmt.Errorf("create task: %w", err)
	}
	return t, nil
}

func (s *Store) UpdateTask(ctx context.Context, t domain.Task) (domain.Task, error) {
	if err := t.Validate(); err != nil {
		return domain.Task{}, err
	}
	t.UpdatedAt = time.Now().UTC()
	args, _ := json.Marshal(t.Arguments)
	env, _ := json.Marshal(t.Environment)
	res, err := s.db.ExecContext(ctx, `UPDATE tasks SET name=?,description=?,enabled=?,command_mode=?,command=?,arguments_json=?,working_dir=?,shell=?,environment_json=?,schedule=?,overlap_policy=?,restart_policy=?,max_retries=?,min_run_seconds=?,updated_at=? WHERE id=? AND kind=?`, t.Name, t.Description, t.Enabled, t.CommandMode, t.Command, args, t.WorkingDir, t.Shell, env, t.Schedule, t.OverlapPolicy, t.RestartPolicy, t.MaxRetries, t.MinRunSeconds, formatTime(t.UpdatedAt), t.ID, t.Kind)
	if err != nil {
		return domain.Task{}, fmt.Errorf("update task: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.Task{}, storepkg.ErrNotFound
	}
	return t, nil
}

func (s *Store) DeleteTask(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM tasks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return storepkg.ErrNotFound
	}
	return nil
}

func (s *Store) SetTaskEnabled(ctx context.Context, id string, enabled bool) error {
	res, err := s.db.ExecContext(ctx, "UPDATE tasks SET enabled=?, updated_at=? WHERE id=?", enabled, formatTime(time.Now().UTC()), id)
	if err != nil {
		return fmt.Errorf("set task enabled: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return storepkg.ErrNotFound
	}
	return nil
}

func (s *Store) CreateExecution(ctx context.Context, e domain.Execution) (domain.Execution, error) {
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO executions (id,task_id,trigger,status,pid,started_at,finished_at,exit_code,error,log_path,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, e.ID, e.TaskID, e.Trigger, e.Status, e.PID, nullableTime(e.StartedAt), nullableTime(e.FinishedAt), e.ExitCode, e.Error, e.LogPath, formatTime(e.CreatedAt))
	if err != nil {
		return domain.Execution{}, fmt.Errorf("create execution: %w", err)
	}
	return e, nil
}

func (s *Store) GetExecution(ctx context.Context, id string) (domain.Execution, error) {
	e, err := scanExecution(s.db.QueryRowContext(ctx, executionSelect+" WHERE id=?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Execution{}, storepkg.ErrNotFound
	}
	return e, err
}

func (s *Store) UpdateExecution(ctx context.Context, e domain.Execution) error {
	res, err := s.db.ExecContext(ctx, `UPDATE executions SET status=?,pid=?,started_at=?,finished_at=?,exit_code=?,error=?,log_path=? WHERE id=?`, e.Status, e.PID, nullableTime(e.StartedAt), nullableTime(e.FinishedAt), e.ExitCode, e.Error, e.LogPath, e.ID)
	if err != nil {
		return fmt.Errorf("update execution: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return storepkg.ErrNotFound
	}
	return nil
}

func (s *Store) ListExecutions(ctx context.Context, taskID string, limit int) ([]domain.Execution, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, executionSelect+` WHERE task_id=? ORDER BY created_at DESC LIMIT ?`, taskID, limit)
	if err != nil {
		return nil, fmt.Errorf("list executions: %w", err)
	}
	defer rows.Close()
	result := make([]domain.Execution, 0)
	for rows.Next() {
		e, err := scanExecution(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

func (s *Store) MarkInterruptedExecutions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `UPDATE executions SET status='failed', finished_at=?, error='Rooster stopped before this execution completed' WHERE status IN ('queued','starting','running','stopping')`, formatTime(time.Now().UTC()))
	if err != nil {
		return fmt.Errorf("recover interrupted executions: %w", err)
	}
	return nil
}

const taskSelect = `SELECT id,name,description,kind,enabled,command_mode,command,arguments_json,working_dir,shell,environment_json,schedule,overlap_policy,restart_policy,max_retries,min_run_seconds,created_at,updated_at FROM tasks`
const executionSelect = `SELECT id,task_id,trigger,status,pid,started_at,finished_at,exit_code,error,log_path,created_at FROM executions`

type scanner interface{ Scan(...any) error }

func scanTask(row scanner) (domain.Task, error) {
	var t domain.Task
	var args, env, created, updated string
	err := row.Scan(&t.ID, &t.Name, &t.Description, &t.Kind, &t.Enabled, &t.CommandMode, &t.Command, &args, &t.WorkingDir, &t.Shell, &env, &t.Schedule, &t.OverlapPolicy, &t.RestartPolicy, &t.MaxRetries, &t.MinRunSeconds, &created, &updated)
	if err != nil {
		return t, err
	}
	if err := json.Unmarshal([]byte(args), &t.Arguments); err != nil {
		return t, fmt.Errorf("decode arguments: %w", err)
	}
	if err := json.Unmarshal([]byte(env), &t.Environment); err != nil {
		return t, fmt.Errorf("decode environment: %w", err)
	}
	t.CreatedAt, err = parseTime(created)
	if err != nil {
		return t, err
	}
	t.UpdatedAt, err = parseTime(updated)
	return t, err
}

func scanExecution(row scanner) (domain.Execution, error) {
	var e domain.Execution
	var started, finished, exit sql.NullString
	var created string
	err := row.Scan(&e.ID, &e.TaskID, &e.Trigger, &e.Status, &e.PID, &started, &finished, &exit, &e.Error, &e.LogPath, &created)
	if err != nil {
		return e, err
	}
	e.CreatedAt, err = parseTime(created)
	if err != nil {
		return e, err
	}
	if started.Valid {
		v, er := parseTime(started.String)
		if er != nil {
			return e, er
		}
		e.StartedAt = &v
	}
	if finished.Valid {
		v, er := parseTime(finished.String)
		if er != nil {
			return e, er
		}
		e.FinishedAt = &v
	}
	if exit.Valid {
		var v int
		if _, er := fmt.Sscan(exit.String, &v); er != nil {
			return e, er
		}
		e.ExitCode = &v
	}
	return e, nil
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
func parseTime(v string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		return t, fmt.Errorf("parse time: %w", err)
	}
	return t, nil
}
func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return formatTime(*t)
}
