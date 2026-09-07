package engine

import (
	"testing"
	"time"
)

func TestNextRunTimes(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	current := time.Date(2026, time.September, 7, 10, 7, 0, 0, location)

	got := nextRunTimes("15 * * * *", current, 5)
	if len(got) != 5 {
		t.Fatalf("next runs count = %d, want 5", len(got))
	}
	for index, run := range got {
		want := time.Date(2026, time.September, 7, 10+index, 15, 0, 0, location)
		if !run.Equal(want) {
			t.Errorf("next run %d = %s, want %s", index, run, want)
		}
	}
}

func TestNextRunTimesRejectsInvalidSchedule(t *testing.T) {
	if got := nextRunTimes("not cron", time.Now(), 5); got != nil {
		t.Fatalf("next runs = %v, want nil", got)
	}
}
