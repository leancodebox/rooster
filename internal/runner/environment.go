package runner

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

type EnvironmentProvider struct {
	mu       sync.RWMutex
	snapshot map[string]string
}

func NewEnvironmentProvider() *EnvironmentProvider {
	return &EnvironmentProvider{snapshot: environmentMap(os.Environ())}
}

func (p *EnvironmentProvider) Refresh(ctx context.Context, shell string) error {
	base := environmentMap(os.Environ())
	if runtime.GOOS != "windows" {
		if shell == "" {
			shell = defaultShell()
		}
		loadCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(loadCtx, shell, "-lic", "env -0")
		cmd.Env = environmentSlice(withFallbackPaths(base))
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			return err
		}
		loaded := parseNullEnvironment(out.Bytes())
		if len(loaded) > 0 {
			base = loaded
		}
	}
	base = withFallbackPaths(base)
	p.mu.Lock()
	p.snapshot = base
	p.mu.Unlock()
	return nil
}

func (p *EnvironmentProvider) Resolve(overrides map[string]string) []string {
	p.mu.RLock()
	merged := make(map[string]string, len(p.snapshot)+len(overrides))
	for k, v := range p.snapshot {
		merged[k] = v
	}
	p.mu.RUnlock()
	for k, v := range overrides {
		merged[k] = v
	}
	return environmentSlice(merged)
}

func parseNullEnvironment(raw []byte) map[string]string {
	result := map[string]string{}
	for _, part := range bytes.Split(raw, []byte{0}) {
		key, value, ok := strings.Cut(string(part), "=")
		if ok && key != "" {
			result[key] = value
		}
	}
	return result
}

func environmentMap(values []string) map[string]string {
	result := make(map[string]string, len(values))
	for _, value := range values {
		key, val, ok := strings.Cut(value, "=")
		if ok {
			result[key] = val
		}
	}
	return result
}

func environmentSlice(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result
}

func withFallbackPaths(values map[string]string) map[string]string {
	result := make(map[string]string, len(values)+1)
	for k, v := range values {
		result[k] = v
	}
	separator := string(os.PathListSeparator)
	paths := strings.Split(result["PATH"], separator)
	seen := map[string]bool{}
	for _, path := range paths {
		if path != "" {
			seen[path] = true
		}
	}
	var fallback []string
	if runtime.GOOS == "windows" {
		fallback = []string{`C:\\Windows\\System32`, `C:\\Windows`}
	} else {
		fallback = []string{"/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin"}
	}
	for _, path := range fallback {
		if !seen[path] {
			paths = append(paths, path)
		}
	}
	result["PATH"] = strings.Join(paths, separator)
	return result
}

func defaultShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	if runtime.GOOS == "darwin" {
		return "/bin/zsh"
	}
	return "/bin/bash"
}
