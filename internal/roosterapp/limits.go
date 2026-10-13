package roosterapp

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/leancodebox/rooster/internal/engine"
)

func executionLimits() (engine.Limits, error) {
	var limits engine.Limits
	for _, option := range []struct {
		name   string
		target *int
	}{
		{"ROOSTER_MAX_CONCURRENT", &limits.MaxConcurrent},
		{"ROOSTER_MAX_PER_TASK", &limits.MaxPerTask},
	} {
		if raw := os.Getenv(option.name); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 0 {
				return limits, fmt.Errorf("%s must be a nonnegative integer", option.name)
			}
			*option.target = value
		}
	}
	if raw := os.Getenv("ROOSTER_EXECUTION_TIMEOUT"); raw != "" {
		value, err := time.ParseDuration(raw)
		if err != nil || value < 0 {
			return limits, fmt.Errorf("ROOSTER_EXECUTION_TIMEOUT must be a nonnegative duration")
		}
		limits.ExecutionTimeout = value
	}
	return limits, nil
}
