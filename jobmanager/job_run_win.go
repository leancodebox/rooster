//go:build windows

package jobmanager

import (
	"os/exec"
	"syscall"
)

func HideWindows(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
