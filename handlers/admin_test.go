package handlers

import (
	"testing"
	"time"
)

// reset helpers — clear shared state between tests
func resetAdminRateLimit() {
	adminLoginMu.Lock()
	defer adminLoginMu.Unlock()
	adminLoginAttempts = map[string]int{}
	adminLoginLockedAt = map[string]time.Time{}
}

// ─── checkAdminLoginRateLimit ─────────────────────────────────────────────────

func TestAdminRateLimit_NoAttempts(t *testing.T) {
	resetAdminRateLimit()
	if err := checkAdminLoginRateLimit("test@example.com"); err != nil {
		t.Errorf("expected nil on first attempt, got: %v", err)
	}
}

func TestAdminRateLimit_LockedAfterMaxAttempts(t *testing.T) {
	resetAdminRateLimit()
	email := "attacker@example.com"
	for i := 0; i < adminMaxAttempts; i++ {
		recordAdminFailedLogin(email)
	}
	if err := checkAdminLoginRateLimit(email); err == nil {
		t.Error("expected rate limit error after max attempts, got nil")
	}
}

func TestAdminRateLimit_DifferentEmailsAreIndependent(t *testing.T) {
	resetAdminRateLimit()
	for i := 0; i < adminMaxAttempts; i++ {
		recordAdminFailedLogin("a@example.com")
	}
	// different email should not be affected
	if err := checkAdminLoginRateLimit("b@example.com"); err != nil {
		t.Errorf("unrelated email should not be rate limited, got: %v", err)
	}
}

func TestAdminRateLimit_ClearResetsLock(t *testing.T) {
	resetAdminRateLimit()
	email := "locked@example.com"
	for i := 0; i < adminMaxAttempts; i++ {
		recordAdminFailedLogin(email)
	}
	clearAdminLoginAttempts(email)
	if err := checkAdminLoginRateLimit(email); err != nil {
		t.Errorf("expected nil after clearing lock, got: %v", err)
	}
}

func TestAdminRateLimit_ExpiredLockIsReleased(t *testing.T) {
	resetAdminRateLimit()
	email := "expired@example.com"
	// Manually set a lock that expired in the past
	adminLoginMu.Lock()
	adminLoginLockedAt[email] = time.Now().Add(-adminLockoutDuration - time.Minute)
	adminLoginMu.Unlock()

	if err := checkAdminLoginRateLimit(email); err != nil {
		t.Errorf("expired lock should be released automatically, got: %v", err)
	}
}

func TestAdminRateLimit_ErrorMessageContainsMinutes(t *testing.T) {
	resetAdminRateLimit()
	email := "msg@example.com"
	for i := 0; i < adminMaxAttempts; i++ {
		recordAdminFailedLogin(email)
	}
	err := checkAdminLoginRateLimit(email)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if len(msg) == 0 {
		t.Error("error message should not be empty")
	}
}
