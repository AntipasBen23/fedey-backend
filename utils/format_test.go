package utils

import "testing"

// ─── safePrefix (pure utility) ────────────────────────────────────────────────

func safePrefix(s string, n int) string {
	if n >= len(s) {
		return s
	}
	return s[:n]
}

func TestSafePrefix_Empty(t *testing.T) {
	got := safePrefix("", 10)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestSafePrefix_Zero(t *testing.T) {
	got := safePrefix("hello", 0)
	if got != "" {
		t.Errorf("n=0 should return empty string, got %q", got)
	}
}

func TestSafePrefix_ExactLength(t *testing.T) {
	got := safePrefix("hello", 5)
	if got != "hello" {
		t.Errorf("exact length should return full string, got %q", got)
	}
}

func TestSafePrefix_Partial(t *testing.T) {
	got := safePrefix("hello world", 5)
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

func TestSafePrefix_BeyondLength(t *testing.T) {
	got := safePrefix("hi", 100)
	if got != "hi" {
		t.Errorf("n > len should return full string, got %q", got)
	}
}

// ─── formatCount ─────────────────────────────────────────────────────────────

func formatCount(n int) string {
	if n >= 1_000_000 {
		v := float64(n) / 1_000_000
		return formatFloat(v) + "M"
	}
	if n >= 1_000 {
		v := float64(n) / 1_000
		return formatFloat(v) + "K"
	}
	return itoa(n)
}

func formatFloat(f float64) string {
	// one decimal place
	i := int(f * 10)
	whole := i / 10
	frac := i % 10
	return itoa(whole) + "." + itoa(frac)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func TestFormatCount_Millions(t *testing.T) {
	got := formatCount(1_500_000)
	if got != "1.5M" {
		t.Errorf("expected 1.5M, got %q", got)
	}
}

func TestFormatCount_Thousands(t *testing.T) {
	got := formatCount(2_300)
	if got != "2.3K" {
		t.Errorf("expected 2.3K, got %q", got)
	}
}

func TestFormatCount_Under1000(t *testing.T) {
	got := formatCount(450)
	if got != "450" {
		t.Errorf("expected 450, got %q", got)
	}
}

func TestFormatCount_Zero(t *testing.T) {
	got := formatCount(0)
	if got != "0" {
		t.Errorf("expected 0, got %q", got)
	}
}

func TestFormatCount_Exactly1000(t *testing.T) {
	got := formatCount(1000)
	if got != "1.0K" {
		t.Errorf("expected 1.0K, got %q", got)
	}
}
