//go:build windows

package runner

import (
	"fmt"
	"os/exec"
)

func configureProcess(cmd *exec.Cmd)           {}
func terminateProcessTree(cmd *exec.Cmd) error { return taskkill(cmd) }
func killProcessTree(cmd *exec.Cmd) error      { return taskkill(cmd) }
func taskkill(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return exec.Command("taskkill", "/PID", fmt.Sprint(cmd.Process.Pid), "/T", "/F").Run()
}
