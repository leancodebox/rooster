//go:build !windows

package jobmanager

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"strings"
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
	slog.Info(shell)
	slog.Info(strings.Join(args, " "))
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
	slog.Info(shell)
	slog.Info(strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, shell, args...)
	HideWindows(cmd)
	cmd.Env = os.Environ()
	cmd.Dir = job.Dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}
