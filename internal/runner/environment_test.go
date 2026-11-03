//go:build !windows

package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnvironmentRetriesEarlyLoginFailure(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ROOSTER_ENV_TEST_DIR", dir)
	shell := filepath.Join(dir, "shell")
	script := `#!/bin/sh
if [ ! -e "$ROOSTER_ENV_TEST_DIR/ready" ]; then
  /usr/bin/touch "$ROOSTER_ENV_TEST_DIR/ready"
  exit 1
fi
printf 'PATH=/ready/bin\000ROOSTER_READY=yes\000'
`
	if err := os.WriteFile(shell, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	p := NewEnvironmentProvider()
	if err := p.RefreshWithRetry(context.Background(), shell); err != nil {
		t.Fatal(err)
	}
	got := environmentMap(p.Resolve(nil))
	if got["ROOSTER_READY"] != "yes" {
		t.Fatal("login environment missing")
	}
	if _, exists := got["ROOSTER_ENV_TEST_DIR"]; exists {
		t.Fatal("variable removed by login shell was restored")
	}
}

func TestEnvironmentRejectsEmptyShellOutputAndKeepsSnapshot(t *testing.T) {
	p := NewEnvironmentProvider()
	p.snapshot = map[string]string{"PATH": "/previous/bin"}
	if err := p.Refresh(context.Background(), "/usr/bin/true"); err == nil {
		t.Fatal("expected empty environment to fail")
	}
	if got := environmentMap(p.Resolve(nil))["PATH"]; got != "/previous/bin" {
		t.Fatalf("snapshot replaced: %q", got)
	}
}

func TestEnvironmentRetryHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := NewEnvironmentProvider().RefreshWithRetry(ctx, "/does/not/exist"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}
