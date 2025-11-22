//go:build windows

package jobmanager

import (
    "os/exec"
    "syscall"
)

func HideWindows(cmd *exec.Cmd) {
    cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

func KillProcessGroup(cmd *exec.Cmd) error {
    if cmd == nil || cmd.Process == nil {
        return nil
    }
    return cmd.Process.Kill()
}
