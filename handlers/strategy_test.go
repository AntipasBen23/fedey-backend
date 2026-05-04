package handlers

import (
	"strings"
	"testing"
)

// ─── Strategy onboarding step validation ─────────────────────────────────────

func isValidOnboardingStep(step string) bool {
	valid := map[string]bool{
		"/hire":           true,
		"/learn":          true,
		"/grow":           true,
		"/strategy":       true,
		"completed":       true,
	}
	return valid[step]
}

func TestIsValidOnboardingStep_Hire(t *testing.T) {
	if !isValidOnboardingStep("/hire") {
		t.Error("/hire should be a valid onboarding step")
	}
}

func TestIsValidOnboardingStep_Learn(t *testing.T) {
	if !isValidOnboardingStep("/learn") {
		t.Error("/learn should be a valid onboarding step")
	}
}

func TestIsValidOnboardingStep_Strategy(t *testing.T) {
	if !isValidOnboardingStep("/strategy") {
		t.Error("/strategy should be a valid onboarding step")
	}
}

func TestIsValidOnboardingStep_Completed(t *testing.T) {
	if !isValidOnboardingStep("completed") {
		t.Error("completed should be a valid onboarding step")
	}
}

func TestIsValidOnboardingStep_Unknown(t *testing.T) {
	if isValidOnboardingStep("/unknown") {
		t.Error("unknown step should not be valid")
	}
}

func TestIsValidOnboardingStep_Empty(t *testing.T) {
	if isValidOnboardingStep("") {
		t.Error("empty step should not be valid")
	}
}

// ─── Job description sanitisation ────────────────────────────────────────────

func sanitiseJobDescription(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) > 500 {
		return trimmed[:500]
	}
	return trimmed
}

func TestSanitiseJobDescription_Normal(t *testing.T) {
	got := sanitiseJobDescription("  I am a founder  ")
	if got != "I am a founder" {
		t.Errorf("expected trimmed string, got %q", got)
	}
}

func TestSanitiseJobDescription_TooLong(t *testing.T) {
	long := strings.Repeat("a", 600)
	got := sanitiseJobDescription(long)
	if len(got) != 500 {
		t.Errorf("expected 500 chars, got %d", len(got))
	}
}

func TestSanitiseJobDescription_Exactly500(t *testing.T) {
	exact := strings.Repeat("b", 500)
	got := sanitiseJobDescription(exact)
	if got != exact {
		t.Errorf("500-char string should pass through unchanged")
	}
}

func TestSanitiseJobDescription_Empty(t *testing.T) {
	got := sanitiseJobDescription("   ")
	if got != "" {
		t.Errorf("whitespace-only should return empty, got %q", got)
	}
}

// ─── Platform context parsing ─────────────────────────────────────────────────

func parsePlatformFromContext(ctx string) string {
	if strings.Contains(ctx, "twitter") || strings.Contains(ctx, ":x:") {
		return "twitter"
	}
	if strings.Contains(ctx, "linkedin") {
		return "linkedin"
	}
	return "unknown"
}

func TestParsePlatformFromContext_Twitter(t *testing.T) {
	got := parsePlatformFromContext("platform:twitter:personal")
	if got != "twitter" {
		t.Errorf("expected twitter, got %q", got)
	}
}

func TestParsePlatformFromContext_LinkedIn(t *testing.T) {
	got := parsePlatformFromContext("platform:linkedin:brand")
	if got != "linkedin" {
		t.Errorf("expected linkedin, got %q", got)
	}
}

func TestParsePlatformFromContext_Unknown(t *testing.T) {
	got := parsePlatformFromContext("platform:instagram:personal")
	if got != "unknown" {
		t.Errorf("expected unknown, got %q", got)
	}
}

func TestParsePlatformFromContext_Empty(t *testing.T) {
	got := parsePlatformFromContext("")
	if got != "unknown" {
		t.Errorf("empty context should return unknown, got %q", got)
	}
}
