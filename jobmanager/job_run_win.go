//go:build windows

package jobmanager

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func HideWindows(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

// JobInit 初始化并执行
func (itself *Job) JobInit() {
	itself.confLock.Lock()
	defer itself.confLock.Unlock()
	if itself.cmd == nil {
		job := itself.jobConfig
		cmd := exec.Command(job.BinPath, job.Params...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		cmd.Dir = job.Dir
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		itself.cmd = cmd
		go itself.jobGuard()
		return nil
	}
	return errors.New("程序运行中")
}
