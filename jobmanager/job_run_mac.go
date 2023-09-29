//go:build darwin || (openbsd && !mips64)

package jobmanager

import (
	"errors"
	"os"
	"os/exec"
)

func HideWindows(cmd *exec.Cmd) {
}

// JobInit 初始化并执行
func (itself *Job) JobInit() error {
	itself.confLock.Lock()
	defer itself.confLock.Unlock()
	if itself.cmd == nil {
		cmd := exec.Command(itself.BinPath, itself.Params...)
		cmd.Dir = itself.Dir
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		itself.cmd = cmd
		go itself.jobGuard()
		return nil
	}
	return errors.New("程序运行中")
}
