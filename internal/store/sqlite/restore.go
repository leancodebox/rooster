package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// PrepareRestoredSnapshot operates only on a copy in a newly reserved directory.
func PrepareRestoredSnapshot(ctx context.Context, path, logDir string) error {
	uri := (&url.URL{Scheme: "file", Path: path, RawQuery: "mode=rw"}).String()
	db, err := sql.Open("sqlite", uri)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var integrity string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		return fmt.Errorf("check backup integrity: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("backup integrity check failed: %s", integrity)
	}
	foreignKeys, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("check backup references: %w", err)
	}
	invalidReferences := foreignKeys.Next()
	referenceErr := foreignKeys.Err()
	foreignKeys.Close()
	if referenceErr != nil {
		return referenceErr
	}
	if invalidReferences {
		return fmt.Errorf("backup contains broken foreign-key references")
	}
	var version int
	if err := db.QueryRowContext(ctx, "SELECT MAX(version) FROM schema_migrations").Scan(&version); err != nil {
		return fmt.Errorf("read backup schema: %w", err)
	}
	if version != 2 {
		return fmt.Errorf("unsupported backup schema version: %d", version)
	}
	for _, query := range []string{taskSelect + " LIMIT 0", executionSelect + " LIMIT 0"} {
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			return fmt.Errorf("validate backup schema: %w", err)
		}
		rows.Close()
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "UPDATE tasks SET enabled=0"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE executions SET status='failed',finished_at=?,error='Interrupted before backup restore' WHERE status IN ('queued','starting','running','stopping')`, formatTime(time.Now().UTC())); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, "SELECT id,task_id FROM executions")
	if err != nil {
		return err
	}
	type identity struct{ id, taskID string }
	var executions []identity
	for rows.Next() {
		var execution identity
		if err := rows.Scan(&execution.id, &execution.taskID); err != nil {
			rows.Close()
			return err
		}
		if _, err := uuid.Parse(execution.id); err != nil {
			rows.Close()
			return fmt.Errorf("invalid execution ID in backup")
		}
		if _, err := uuid.Parse(execution.taskID); err != nil {
			rows.Close()
			return fmt.Errorf("invalid task ID in backup")
		}
		executions = append(executions, execution)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, execution := range executions {
		path := filepath.Join(logDir, execution.taskID, execution.id+".log")
		if _, err := tx.ExecContext(ctx, "UPDATE executions SET log_path=? WHERE id=?", path, execution.id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
