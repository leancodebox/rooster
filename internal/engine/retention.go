package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/leancodebox/rooster/internal/domain"
)

type RetentionResult struct {
	DryRun     bool     `json:"dryRun"`
	Candidates int      `json:"candidates"`
	Deleted    int      `json:"deleted"`
	Files      int      `json:"files"`
	Bytes      int64    `json:"bytes"`
	Errors     []string `json:"errors"`
}

func (e *Engine) Retain(ctx context.Context, days, keep int, dryRun bool) (RetentionResult, error) {
	result := RetentionResult{DryRun: dryRun, Errors: []string{}}
	if days < 1 || days > 36500 || keep < 1 {
		return result, errors.New("retention requires days between 1 and 36500 and keepPerTask >= 1")
	}
	candidates, err := e.store.RetentionCandidates(ctx, time.Now().UTC().Add(-time.Duration(days)*24*time.Hour), keep)
	if err != nil {
		return result, err
	}
	for _, execution := range candidates {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		e.mu.RLock()
		_, active := e.active[execution.ID]
		e.mu.RUnlock()
		if active {
			continue
		}
		paths, bytes, err := e.retentionFiles(execution)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", execution.ID, err))
			continue
		}
		result.Candidates++
		result.Files += len(paths)
		result.Bytes += bytes
		if dryRun {
			continue
		}
		failed := false
		for _, path := range paths {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", execution.ID, err))
				failed = true
				break
			}
		}
		if failed {
			continue
		}
		if err := e.store.DeleteFinishedExecution(ctx, execution.ID); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", execution.ID, err))
			continue
		}
		result.Deleted++
	}
	return result, nil
}

func (e *Engine) retentionFiles(execution domain.Execution) ([]string, int64, error) {
	if _, err := uuid.Parse(execution.ID); err != nil {
		return nil, 0, errors.New("unmanaged execution ID")
	}
	if _, err := uuid.Parse(execution.TaskID); err != nil {
		return nil, 0, errors.New("unmanaged task ID")
	}
	root, err := filepath.Abs(e.logDir)
	if err != nil {
		return nil, 0, err
	}
	directory := filepath.Join(root, execution.TaskID)
	expected := filepath.Join(directory, execution.ID+".log")
	actual, err := filepath.Abs(execution.LogPath)
	if err != nil || actual != expected {
		return nil, 0, errors.New("log is outside managed execution path")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	resolvedDir, err := filepath.EvalSymlinks(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	if resolvedDir != filepath.Join(resolvedRoot, execution.TaskID) {
		return nil, 0, errors.New("symlinked task log directory")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, 0, err
	}
	var paths []string
	var bytes int64
	for _, entry := range entries {
		name := entry.Name()
		if name != execution.ID+".log" && !(strings.HasPrefix(name, execution.ID+"-") && (strings.HasSuffix(name, ".log.gz") || strings.HasSuffix(name, ".log"))) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, 0, err
		}
		if !info.Mode().IsRegular() {
			return nil, 0, errors.New("nonregular execution log")
		}
		paths = append(paths, filepath.Join(directory, name))
		bytes += info.Size()
	}
	return paths, bytes, nil
}
