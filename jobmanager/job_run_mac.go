//go:build darwin || (openbsd && !mips64)

package jobmanager

import (
    "os/exec"
    "syscall"
)

func HideWindows(cmd *exec.Cmd) {
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func KillProcessGroup(cmd *exec.Cmd) error {
    if cmd == nil || cmd.Process == nil {
        return nil
    }
    pid := cmd.Process.Pid
    return syscall.Kill(-pid, syscall.SIGTERM)
}
