package worker

import (
	"testing"
	"time"
)

// ─── Scheduling window helpers (pure logic) ───────────────────────────────────

// isWithinPostingWindow returns true if hour falls in the preferred posting window.
func isWithinPostingWindow(hour int) bool {
	// Best engagement: 8–10 AM and 5–7 PM
	return (hour >= 8 && hour <= 10) || (hour >= 17 && hour <= 19)
}

func TestIsWithinPostingWindow_MorningStart(t *testing.T) {
	if !isWithinPostingWindow(8) {
		t.Error("hour 8 should be in posting window")
	}
}

func TestIsWithinPostingWindow_MorningEnd(t *testing.T) {
	if !isWithinPostingWindow(10) {
		t.Error("hour 10 should be in posting window")
	}
}

func TestIsWithinPostingWindow_EveningStart(t *testing.T) {
	if !isWithinPostingWindow(17) {
		t.Error("hour 17 should be in posting window")
	}
}

func TestIsWithinPostingWindow_EveningEnd(t *testing.T) {
	if !isWithinPostingWindow(19) {
		t.Error("hour 19 should be in posting window")
	}
}

func TestIsWithinPostingWindow_Midday(t *testing.T) {
	if isWithinPostingWindow(12) {
		t.Error("hour 12 should not be in posting window")
	}
}

func TestIsWithinPostingWindow_Midnight(t *testing.T) {
	if isWithinPostingWindow(0) {
		t.Error("midnight should not be in posting window")
	}
}

func TestIsWithinPostingWindow_LateNight(t *testing.T) {
	if isWithinPostingWindow(23) {
		t.Error("hour 23 should not be in posting window")
	}
}

// ─── Retry delay calculation ──────────────────────────────────────────────────

func retryDelay(attempt int) time.Duration {
	base := 5 * time.Minute
	switch {
	case attempt <= 0:
		return base
	case attempt == 1:
		return 10 * time.Minute
	case attempt == 2:
		return 30 * time.Minute
	default:
		return 60 * time.Minute
	}
}

func TestRetryDelay_FirstAttempt(t *testing.T) {
	got := retryDelay(0)
	if got != 5*time.Minute {
		t.Errorf("attempt 0: expected 5m, got %v", got)
	}
}

func TestRetryDelay_SecondAttempt(t *testing.T) {
	got := retryDelay(1)
	if got != 10*time.Minute {
		t.Errorf("attempt 1: expected 10m, got %v", got)
	}
}

func TestRetryDelay_ThirdAttempt(t *testing.T) {
	got := retryDelay(2)
	if got != 30*time.Minute {
		t.Errorf("attempt 2: expected 30m, got %v", got)
	}
}

func TestRetryDelay_MaxAttempt(t *testing.T) {
	got := retryDelay(5)
	if got != 60*time.Minute {
		t.Errorf("attempt 5+: expected 60m, got %v", got)
	}
}

func TestRetryDelay_NegativeAttempt(t *testing.T) {
	got := retryDelay(-1)
	if got != 5*time.Minute {
		t.Errorf("negative attempt: expected base 5m, got %v", got)
	}
}
