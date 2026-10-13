package roosterapp

import (
	"testing"
	"time"
)

func TestExecutionLimits(t *testing.T) {
	t.Setenv("ROOSTER_MAX_CONCURRENT", "4")
	t.Setenv("ROOSTER_MAX_PER_TASK", "2")
	t.Setenv("ROOSTER_EXECUTION_TIMEOUT", "5m")
	limits, err := executionLimits()
	if err != nil || limits.MaxConcurrent != 4 || limits.MaxPerTask != 2 || limits.ExecutionTimeout != 5*time.Minute {
		t.Fatalf("limits=%+v error=%v", limits, err)
	}
	t.Setenv("ROOSTER_MAX_CONCURRENT", "-1")
	if _, err := executionLimits(); err == nil {
		t.Fatal("accepted negative concurrency")
	}
}
