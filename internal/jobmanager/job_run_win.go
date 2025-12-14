//go:build windows

package jobmanager

import (
	"os/exec"
	"syscall"
)

// HideWindows 在 Windows 上隐藏控制台窗口
func HideWindows(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

// KillProcessGroup 在 Windows 上终止进程
func KillProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

func ForceKillProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
