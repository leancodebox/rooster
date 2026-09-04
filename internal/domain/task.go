package domain

import (
	"errors"
	"net/url"
	"strings"
	"time"
)

type TaskKind string

const (
	TaskKindResident  TaskKind = "resident"
	TaskKindScheduled TaskKind = "scheduled"
)

type CommandMode string

const (
	CommandModeShell CommandMode = "shell"
	CommandModeExec  CommandMode = "exec"
)

type OverlapPolicy string

const (
	OverlapSkip     OverlapPolicy = "skip"
	OverlapParallel OverlapPolicy = "parallel"
)

type RestartPolicy string

const (
	RestartOnFailure RestartPolicy = "on_failure"
	RestartAlways    RestartPolicy = "always"
	RestartNever     RestartPolicy = "never"
)

type Task struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Link          string            `json:"link"`
	Kind          TaskKind          `json:"kind"`
	Enabled       bool              `json:"enabled"`
	CommandMode   CommandMode       `json:"commandMode"`
	Command       string            `json:"command"`
	Arguments     []string          `json:"arguments"`
	WorkingDir    string            `json:"workingDir"`
	Shell         string            `json:"shell"`
	Environment   map[string]string `json:"environment"`
	Schedule      string            `json:"schedule"`
	OverlapPolicy OverlapPolicy     `json:"overlapPolicy"`
	RestartPolicy RestartPolicy     `json:"restartPolicy"`
	MaxRetries    int               `json:"maxRetries"`
	MinRunSeconds int               `json:"minRunSeconds"`
	CreatedAt     time.Time         `json:"createdAt"`
	UpdatedAt     time.Time         `json:"updatedAt"`
}

func (t Task) Validate() error {
	if strings.TrimSpace(t.Name) == "" {
		return errors.New("task name is required")
	}
	if t.Link != "" {
		link, err := url.ParseRequestURI(t.Link)
		if err != nil || link.Host == "" || link.Scheme != "http" && link.Scheme != "https" {
			return errors.New("task link must be an http or https URL")
		}
	}
	if t.Kind != TaskKindResident && t.Kind != TaskKindScheduled {
		return errors.New("task kind must be resident or scheduled")
	}
	if t.CommandMode != CommandModeShell && t.CommandMode != CommandModeExec {
		return errors.New("command mode must be shell or exec")
	}
	if strings.TrimSpace(t.Command) == "" {
		return errors.New("command is required")
	}
	if t.Kind == TaskKindScheduled && strings.TrimSpace(t.Schedule) == "" {
		return errors.New("scheduled task requires a cron expression")
	}
	if t.OverlapPolicy != OverlapSkip && t.OverlapPolicy != OverlapParallel {
		return errors.New("overlap policy must be skip or parallel")
	}
	if t.MaxRetries < 0 || t.MinRunSeconds < 0 {
		return errors.New("retry values cannot be negative")
	}
	for key, value := range t.Environment {
		if key == "" || strings.ContainsAny(key, "=\x00") || strings.ContainsRune(value, '\x00') {
			return errors.New("environment variables contain an invalid key or value")
		}
	}
	return nil
}
