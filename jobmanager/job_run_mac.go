//go:build darwin || (openbsd && !mips64)

package jobmanager

import (
	"os/exec"
	"syscall"
)

// HideWindows 在 Unix 上用于设置进程组，便于统一终止
func HideWindows(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// KillProcessGroup 终止整个进程组
func KillProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	pid := cmd.Process.Pid
	return syscall.Kill(-pid, syscall.SIGTERM)
}
