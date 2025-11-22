//go:build windows

package jobmanager

import (
	"context"
	"os"
	"os/exec"
)

func buildCmd(job *Job) *exec.Cmd {
	shell := "cmd.exe"
	args := append([]string{"/C", job.BinPath}, job.Params...)
	cmd := exec.Command(shell, args...)
	HideWindows(cmd)
	cmd.Env = os.Environ()
	cmd.Dir = job.Dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func buildCmdWithCtx(ctx context.Context, job *Job) *exec.Cmd {
	shell := "cmd.exe"
	args := append([]string{"/C", job.BinPath}, job.Params...)
	cmd := exec.CommandContext(ctx, shell, args...)
	HideWindows(cmd)
	cmd.Env = os.Environ()
	cmd.Dir = job.Dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}
