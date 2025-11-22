//go:build !windows

package jobmanager

import (
	"context"
	"os"
	"os/exec"
)

func buildCmd(job *Job) *exec.Cmd {
	shell := job.Options.ShellPath
	if shell == "" {
		shell = os.Getenv("SHELL")
	}
	if shell == "" {
		shell = "/bin/bash"
	}
	fullCommand := job.BinPath
	args := []string{"-lc", fullCommand}
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
	shell := job.Options.ShellPath
	if shell == "" {
		shell = os.Getenv("SHELL")
	}
	if shell == "" {
		shell = "/bin/bash"
	}
	fullCommand := job.BinPath
	args := []string{"-lc", fullCommand}
	cmd := exec.CommandContext(ctx, shell, args...)
	HideWindows(cmd)
	cmd.Env = os.Environ()
	cmd.Dir = job.Dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}
