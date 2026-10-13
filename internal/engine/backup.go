package engine

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/leancodebox/rooster/internal/domain"
)

func (e *Engine) ExportTasks(ctx context.Context) ([]domain.Task, error) {
	return e.store.ListTasks(ctx)
}

func (e *Engine) WriteBackup(ctx context.Context, writer io.Writer) error {
	dir, err := os.MkdirTemp("", "rooster-backup-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "rooster.db")
	if err := e.store.Backup(ctx, path); err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(writer, file)
	return err
}
