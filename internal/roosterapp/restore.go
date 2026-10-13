package roosterapp

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	storesqlite "github.com/leancodebox/rooster/internal/store/sqlite"
)

func RestoreSnapshot(ctx context.Context, source, destination string) error {
	target, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("backup must be a regular file")
	}
	// Only standalone snapshots are supported, not a live database with a WAL.
	if wal, err := os.Stat(source + "-wal"); err == nil && wal.Size() > 0 {
		return fmt.Errorf("source has a WAL; download a consistent snapshot first")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Mkdir(target, 0o700); err != nil {
		return fmt.Errorf("restore requires a new destination directory: %w", err)
	}
	complete := false
	locked := false
	databaseCreated := false
	logsCreated := false
	var lock *os.File
	defer func() {
		if !complete && locked {
			// Remove only restore-owned artifacts; never recursively erase a
			// destination another process might have populated during failure.
			if databaseCreated {
				for _, name := range []string{"rooster.db", "rooster.db-wal", "rooster.db-shm", "rooster.db-journal"} {
					_ = os.Remove(filepath.Join(target, name))
				}
			}
			if logsCreated {
				_ = os.Remove(filepath.Join(target, "logs"))
			}
		}
		if lock != nil {
			_ = lock.Close()
		}
		if !complete && locked {
			_ = os.Remove(filepath.Join(target, "rooster.lock"))
			_ = os.Remove(target)
		}
	}()
	lock, err = os.OpenFile(filepath.Join(target, "rooster.lock"), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	if err := lockFile(lock); err != nil {
		return err
	}
	locked = true
	path := filepath.Join(target, "rooster.db")
	output, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	databaseCreated = true
	_, copyErr := io.Copy(output, input)
	syncErr := output.Sync()
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if syncErr != nil {
		return syncErr
	}
	if closeErr != nil {
		return closeErr
	}
	logDir := filepath.Join(target, "logs")
	if err := os.Mkdir(logDir, 0o700); err != nil {
		return err
	}
	logsCreated = true
	if err := storesqlite.PrepareRestoredSnapshot(ctx, path, logDir); err != nil {
		return err
	}
	complete = true
	return nil
}
