//go:build !windows

package jobmanager

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

func buildCmd(job *Job) *exec.Cmd {
	shell := resolveShell(job)
	fullCommand := job.BinPath
	args := []string{"-lc", fullCommand}
	slog.Info(shell)
	slog.Info(strings.Join(args, " "))
	cmd := exec.Command(shell, args...)
	HideWindows(cmd)
	cmd.Env = loadUnixEnv(shell)
	cmd.Dir = job.Dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func buildCmdWithCtx(ctx context.Context, job *Job) *exec.Cmd {
	shell := resolveShell(job)
	fullCommand := job.BinPath
	args := []string{"-lc", fullCommand}
	slog.Info("command", "bin", shell, "args", strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, shell, args...)
	HideWindows(cmd)
	cmd.Env = loadUnixEnv(shell)
	cmd.Dir = job.Dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func resolveShell(job *Job) string {
	shell := job.Options.ShellPath
	if shell == "" {
		shell = os.Getenv("SHELL")
	}
	if shell == "" {
		if runtime.GOOS == "darwin" {
			shell = "/bin/zsh"
		} else {
			shell = "/bin/bash"
		}
	}
	return shell
}

func enrichUnixEnv() []string {
	env := os.Environ()
	p := os.Getenv("PATH")
	u, _ := user.Current()
	h := os.Getenv("HOME")
	if h == "" && u != nil {
		h = u.HomeDir
	}
	if h != "" {
		env = replaceEnv(env, "HOME", h)
	}
	var add []string
	add = append(add, "/usr/local/bin")
	add = append(add, "/opt/homebrew/bin")
	add = append(add, "/usr/bin")
	add = append(add, "/bin")
	add = append(add, "/usr/sbin")
	add = append(add, "/sbin")
	b, _ := os.ReadFile("/etc/paths")
	if len(b) > 0 {
		for _, ln := range strings.Split(string(b), "\n") {
			v := strings.TrimSpace(ln)
			if v != "" {
				add = append(add, v)
			}
		}
	}
	ds, _ := os.ReadDir("/etc/paths.d")
	for _, d := range ds {
		if d.IsDir() {
			continue
		}
		fp := filepath.Join("/etc/paths.d", d.Name())
		bb, _ := os.ReadFile(fp)
		if len(bb) == 0 {
			continue
		}
		for _, ln := range strings.Split(string(bb), "\n") {
			v := strings.TrimSpace(ln)
			if v != "" {
				add = append(add, v)
			}
		}
	}
	parts := []string{}
	seen := map[string]bool{}
	for _, s := range strings.Split(p, ":") {
		v := strings.TrimSpace(s)
		if v == "" {
			continue
		}
		if !seen[v] {
			seen[v] = true
			parts = append(parts, v)
		}
	}
	for _, v := range add {
		vv := strings.TrimSpace(v)
		if vv == "" {
			continue
		}
		if !seen[vv] {
			seen[vv] = true
			parts = append(parts, vv)
		}
	}
	np := strings.Join(parts, ":")
	env = replaceEnv(env, "PATH", np)
	return env
}

func replaceEnv(env []string, key, val string) []string {
	k := key + "="
	out := make([]string, 0, len(env)+1)
	r := false
	for _, e := range env {
		if strings.HasPrefix(e, k) {
			out = append(out, k+val)
			r = true
		} else {
			out = append(out, e)
		}
	}
	if !r {
		out = append(out, k+val)
	}
	return out
}

func loadUnixEnv(shell string) []string {
	outEnv := enrichUnixEnv()
	var script string
	if strings.Contains(shell, "zsh") {
		script = "[ -f ~/.zshenv ] && source ~/.zshenv; [ -f ~/.zprofile ] && source ~/.zprofile; [ -f ~/.zshrc ] && source ~/.zshrc; env -0"
	} else {
		script = "[ -f ~/.bash_profile ] && source ~/.bash_profile; [ -f ~/.profile ] && source ~/.profile; [ -f ~/.bashrc ] && source ~/.bashrc; env -0"
	}
	cmd := exec.Command(shell, "-lc", script)
	cmd.Env = outEnv
	b, err := cmd.Output()
	if err != nil || len(b) == 0 {
		return outEnv
	}
	parts := strings.Split(string(b), "\x00")
	env := make([]string, 0, len(parts))
	for _, kv := range parts {
		if kv == "" {
			continue
		}
		env = append(env, kv)
	}
	p := ""
	for _, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			p = strings.TrimPrefix(kv, "PATH=")
			break
		}
	}
	if p != "" {
		merged := enrichPaths(p)
		env = replaceEnv(env, "PATH", merged)
	}
	return env
}

func enrichPaths(p string) string {
	add := []string{"/usr/local/bin", "/opt/homebrew/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin"}
	parts := []string{}
	seen := map[string]bool{}
	for _, s := range strings.Split(p, ":") {
		v := strings.TrimSpace(s)
		if v == "" {
			continue
		}
		if !seen[v] {
			seen[v] = true
			parts = append(parts, v)
		}
	}
	for _, v := range add {
		vv := strings.TrimSpace(v)
		if vv == "" {
			continue
		}
		if !seen[vv] {
			seen[vv] = true
			parts = append(parts, vv)
		}
	}
	return strings.Join(parts, ":")
}
