//go:build windows

package jobmanager

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func buildCmd(job *Job) *exec.Cmd {
	shell := "cmd.exe"
	args := []string{"/C", job.BinPath}
	cmd := exec.Command(shell, args...)
	HideWindows(cmd)
	cmd.Env = enrichWinEnv()
	cmd.Dir = job.Dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func buildCmdWithCtx(ctx context.Context, job *Job) *exec.Cmd {
	shell := "cmd.exe"
	args := []string{"/C", job.BinPath}
	cmd := exec.CommandContext(ctx, shell, args...)
	HideWindows(cmd)
	cmd.Env = enrichWinEnv()
	cmd.Dir = job.Dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func enrichWinEnv() []string {
	env := os.Environ()
	p := os.Getenv("PATH")
	var add []string
	add = append(add, `C:\\Windows\\System32`)
	add = append(add, `C:\\Windows`)
	gp := `C:\\Program Files\\Git\\bin`
	if st, err := os.Stat(gp); err == nil && st.IsDir() {
		add = append(add, gp)
	}
	gpx := `C:\\Program Files (x86)\\Git\\bin`
	if st, err := os.Stat(gpx); err == nil && st.IsDir() {
		add = append(add, gpx)
	}
	parts := []string{}
	seen := map[string]bool{}
	for _, s := range strings.Split(p, ";") {
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
		vv := strings.TrimSpace(filepath.Clean(v))
		if vv == "" {
			continue
		}
		if !seen[vv] {
			seen[vv] = true
			parts = append(parts, vv)
		}
	}
	np := strings.Join(parts, ";")
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
