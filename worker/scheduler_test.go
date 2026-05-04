package worker

import "testing"

// ─── safePrefix ──────────────────────────────────────────────────────────────

func TestSafePrefix_ShorterThanN(t *testing.T) {
	got := safePrefix("hello", 10)
	if got != "hello" {
		t.Errorf("expected unchanged string, got %q", got)
	}
}

func TestSafePrefix_ExactlyN(t *testing.T) {
	got := safePrefix("hello", 5)
	if got != "hello" {
		t.Errorf("expected unchanged string at exact length, got %q", got)
	}
}

func TestSafePrefix_LongerThanN(t *testing.T) {
	got := safePrefix("hello world", 5)
	if got != "hello" {
		t.Errorf("expected %q, got %q", "hello", got)
	}
}

func TestSafePrefix_EmptyString(t *testing.T) {
	got := safePrefix("", 5)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestSafePrefix_ZeroN(t *testing.T) {
	got := safePrefix("hello", 0)
	if got != "" {
		t.Errorf("expected empty string with n=0, got %q", got)
	}
}
