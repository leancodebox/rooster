//go:build !windows

package runner

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func configureProcess(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

func terminateProcessTree(cmd *exec.Cmd) error { return signalProcessGroup(cmd, syscall.SIGTERM) }
func killProcessTree(cmd *exec.Cmd) error      { return signalProcessGroup(cmd, syscall.SIGKILL) }

func signalProcessGroup(cmd *exec.Cmd, signal syscall.Signal) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	err := syscall.Kill(-cmd.Process.Pid, signal)
	if errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
