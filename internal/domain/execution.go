package domain

import "time"

type Trigger string

const (
	TriggerManual     Trigger = "manual"
	TriggerSchedule   Trigger = "schedule"
	TriggerSupervisor Trigger = "supervisor"
)

type ExecutionStatus string

const (
	ExecutionQueued    ExecutionStatus = "queued"
	ExecutionStarting  ExecutionStatus = "starting"
	ExecutionRunning   ExecutionStatus = "running"
	ExecutionStopping  ExecutionStatus = "stopping"
	ExecutionSucceeded ExecutionStatus = "succeeded"
	ExecutionFailed    ExecutionStatus = "failed"
	ExecutionCanceled  ExecutionStatus = "canceled"
)

type Execution struct {
	ID         string          `json:"id"`
	TaskID     string          `json:"taskId"`
	Trigger    Trigger         `json:"trigger"`
	Status     ExecutionStatus `json:"status"`
	PID        int             `json:"pid,omitempty"`
	StartedAt  *time.Time      `json:"startedAt,omitempty"`
	FinishedAt *time.Time      `json:"finishedAt,omitempty"`
	ExitCode   *int            `json:"exitCode,omitempty"`
	Error      string          `json:"error,omitempty"`
	LogPath    string          `json:"logPath"`
	CreatedAt  time.Time       `json:"createdAt"`
}
