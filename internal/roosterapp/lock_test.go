package roosterapp

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInstanceLockSubprocess(t *testing.T) {
	path := os.Getenv("ROOSTER_LOCK_TEST_HELPER")
	if path == "" {
		t.Skip("subprocess helper")
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := lockFile(file); err != nil {
		t.Fatal(err)
	}
	fmt.Println("locked")
	for {
		time.Sleep(time.Hour)
	}
}

func TestInstanceLockReleasedAfterProcessKill(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rooster.lock")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestInstanceLockSubprocess$")
	cmd.Env = append(os.Environ(), "ROOSTER_LOCK_TEST_HELPER="+path)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	defer func() {
		if !waited {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "locked" {
		t.Fatalf("helper did not acquire lock: %q %v", line, err)
	}
	file, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := lockFile(file); err == nil {
		t.Fatal("lock accepted while helper was alive")
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	waited = true
	if err := lockFile(file); err != nil {
		t.Fatalf("lock remained after abnormal exit: %v", err)
	}
}
