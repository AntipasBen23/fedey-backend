package worker

import (
	"strings"
	"testing"
)

// ─── safePrefix additional coverage ──────────────────────────────────────────

func TestSafePrefix_UnicodeBytes(t *testing.T) {
	// safePrefix works on bytes, not runes — verify it doesn't panic on ASCII
	s := "hello world"
	got := safePrefix(s, 5)
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
}

func TestSafePrefix_LargeN(t *testing.T) {
	s := "short"
	got := safePrefix(s, 10000)
	if got != s {
		t.Errorf("large n should return full string, got %q", got)
	}
}

// ─── Outreach eligibility logic (pure) ───────────────────────────────────────
// These mirror the eligibility conditions checked before sending outreach emails.

func isEligibleForOutreach(lastOnboardingStep string, emailSentCount int) bool {
	if lastOnboardingStep == "completed" {
		return false
	}
	if emailSentCount >= 3 {
		return false
	}
	return true
}

func TestOutreachEligibility_CompletedUser(t *testing.T) {
	if isEligibleForOutreach("completed", 0) {
		t.Error("completed users should not be eligible for outreach")
	}
}

func TestOutreachEligibility_MaxEmailsReached(t *testing.T) {
	if isEligibleForOutreach("/strategy", 3) {
		t.Error("users with 3+ emails sent should not be eligible")
	}
}

func TestOutreachEligibility_HalfwayUser(t *testing.T) {
	if !isEligibleForOutreach("/strategy", 1) {
		t.Error("halfway user with 1 email sent should be eligible")
	}
}

func TestOutreachEligibility_NewUser(t *testing.T) {
	if !isEligibleForOutreach("/hire", 0) {
		t.Error("new user should be eligible for outreach")
	}
}

// ─── Post snippet helper ──────────────────────────────────────────────────────

func makePostSnippet(content string) string {
	return safePrefix(content, 80)
}

func TestMakePostSnippet_Long(t *testing.T) {
	content := strings.Repeat("word ", 30)
	snippet := makePostSnippet(content)
	if len(snippet) > 80 {
		t.Errorf("snippet too long: %d chars", len(snippet))
	}
}

func TestMakePostSnippet_Short(t *testing.T) {
	content := "Short post"
	snippet := makePostSnippet(content)
	if snippet != content {
		t.Errorf("short post should pass through unchanged, got %q", snippet)
	}
}
