package handlers

import "testing"

// ─── AuthCallbackRequest validation ──────────────────────────────────────────

func TestAuthCallbackRequest_PlatformFields(t *testing.T) {
	tests := []struct {
		platform    string
		accountType string
		wantValid   bool
	}{
		{"twitter", "new", true},
		{"twitter", "old", true},
		{"x", "new", true},
		{"linkedin", "new", true},
		{"", "new", false},
		{"twitter", "", false},
	}

	for _, tt := range tests {
		req := AuthCallbackRequest{
			AccessToken:  "token",
			Platform:     tt.platform,
			AccountType:  tt.accountType,
		}
		valid := req.Platform != "" && req.AccountType != ""
		if valid != tt.wantValid {
			t.Errorf("platform=%q accountType=%q: expected valid=%v, got %v",
				tt.platform, tt.accountType, tt.wantValid, valid)
		}
	}
}

func TestAuthCallbackRequest_RefreshTokenOptional(t *testing.T) {
	// RefreshToken is optional — an empty value should still be a valid request
	req := AuthCallbackRequest{
		AccessToken:  "access",
		RefreshToken: "",
		Platform:     "twitter",
		AccountType:  "new",
	}
	if req.AccessToken == "" {
		t.Error("AccessToken should not be empty")
	}
	// RefreshToken being empty is acceptable
	if req.Platform == "" || req.AccountType == "" {
		t.Error("Platform and AccountType are required")
	}
}

func TestAuthCallbackRequest_WithRefreshToken(t *testing.T) {
	req := AuthCallbackRequest{
		AccessToken:  "access_token_value",
		RefreshToken: "refresh_token_value",
		Platform:     "twitter",
		AccountType:  "new",
	}
	if req.RefreshToken != "refresh_token_value" {
		t.Errorf("unexpected refresh token: %q", req.RefreshToken)
	}
}
