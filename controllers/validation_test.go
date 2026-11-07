package controllers

import (
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		email    string
		expected bool
	}{
		{"user@example.com", true},
		{"admin@fasttix.id", true},
		{"invalid-email", false},
		{"user@", false},
		{"@domain.com", false},
		{"", false},
	}

	for _, tc := range tests {
		result := isValidEmail(tc.email)
		if result != tc.expected {
			t.Errorf("isValidEmail(%q) = %v, expected %v", tc.email, result, tc.expected)
		}
	}
}

func TestParseFlexibleDateTime(t *testing.T) {
	tests := []struct {
		input       string
		shouldPass  bool
		expectedDay int
	}{
		{"2026-12-01T19:00:00Z", true, 1},
		{"2026-11-15 20:30:00", true, 15},
		{"2026-10-25", true, 25},
		{"invalid-date-format", false, 0},
	}

	for _, tc := range tests {
		parsed, err := parseFlexibleDateTime(tc.input)
		if tc.shouldPass && err != nil {
			t.Errorf("parseFlexibleDateTime(%q) unexpected error: %v", tc.input, err)
		} else if !tc.shouldPass && err == nil {
			t.Errorf("parseFlexibleDateTime(%q) expected error, but got nil", tc.input)
		} else if tc.shouldPass && parsed.Day() != tc.expectedDay {
			t.Errorf("parseFlexibleDateTime(%q) expected day %d, got %d", tc.input, tc.expectedDay, parsed.Day())
		}
	}
}

func TestValidEventStatuses(t *testing.T) {
	valid := []string{"draft", "upcoming", "ongoing", "completed", "cancelled"}
	for _, s := range valid {
		if !validEventStatuses[s] {
			t.Errorf("expected status %s to be valid", s)
		}
	}

	invalid := []string{"unknown", "published", "active", "deleted", ""}
	for _, s := range invalid {
		if validEventStatuses[s] {
			t.Errorf("expected status %s to be invalid", s)
		}
	}
}

func TestDateComparison(t *testing.T) {
	start, _ := parseFlexibleDateTime("2026-12-01T19:00:00Z")
	endEarlier, _ := parseFlexibleDateTime("2026-12-01T18:00:00Z")
	endLater, _ := parseFlexibleDateTime("2026-12-01T23:00:00Z")

	if !endEarlier.Before(start) {
		t.Errorf("expected endEarlier to be before start")
	}
	if endLater.Before(start) {
		t.Errorf("expected endLater not to be before start")
	}
}

func TestJwtExpirationConfig(t *testing.T) {
	// Test default fallback
	os.Unsetenv("JWT_ACCESS_EXP_MINUTES")
	os.Unsetenv("JWT_REFRESH_EXP_DAYS")

	if dur := getAccessExpDuration(); dur != 15*time.Minute {
		t.Errorf("expected default 15m, got %v", dur)
	}
	if dur := getRefreshExpDuration(); dur != 7*24*time.Hour {
		t.Errorf("expected default 7d, got %v", dur)
	}

	// Test custom values
	os.Setenv("JWT_ACCESS_EXP_MINUTES", "30")
	os.Setenv("JWT_REFRESH_EXP_DAYS", "14")
	if dur := getAccessExpDuration(); dur != 30*time.Minute {
		t.Errorf("expected custom 30m, got %v", dur)
	}
	if dur := getRefreshExpDuration(); dur != 14*24*time.Hour {
		t.Errorf("expected custom 14d, got %v", dur)
	}
}

func TestGetJWTSecret(t *testing.T) {
	// 1. Unset JWT_SECRET
	os.Unsetenv("JWT_SECRET")
	secret, err := getJWTSecret()
	if err == nil || secret != "" {
		t.Errorf("expected error when JWT_SECRET is unset, got secret %q", secret)
	}

	// 2. Set whitespace only
	os.Setenv("JWT_SECRET", "   ")
	secret, err = getJWTSecret()
	if err == nil || secret != "" {
		t.Errorf("expected error when JWT_SECRET is only whitespace, got secret %q", secret)
	}

	// 3. Valid secret
	os.Setenv("JWT_SECRET", "my_super_secret")
	secret, err = getJWTSecret()
	if err != nil || secret != "my_super_secret" {
		t.Errorf("expected secret 'my_super_secret', got %q with error: %v", secret, err)
	}
}

func TestExtractUserIDFromClaims(t *testing.T) {
	tests := []struct {
		name        string
		claims      jwt.MapClaims
		expectedID  int
		expectedOK  bool
	}{
		{
			name:        "Float64 user_id (standard jwt unmarshal)",
			claims:      jwt.MapClaims{"user_id": float64(12)},
			expectedID:  12,
			expectedOK:  true,
		},
		{
			name:        "Int user_id",
			claims:      jwt.MapClaims{"user_id": 45},
			expectedID:  45,
			expectedOK:  true,
		},
		{
			name:        "String user_id",
			claims:      jwt.MapClaims{"user_id": "78"},
			expectedID:  78,
			expectedOK:  true,
		},
		{
			name:        "Missing user_id",
			claims:      jwt.MapClaims{"email": "test@example.com"},
			expectedID:  0,
			expectedOK:  false,
		},
		{
			name:        "Invalid string user_id",
			claims:      jwt.MapClaims{"user_id": "not-a-number"},
			expectedID:  0,
			expectedOK:  false,
		},
		{
			name:        "Nil user_id",
			claims:      jwt.MapClaims{"user_id": nil},
			expectedID:  0,
			expectedOK:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, ok := extractUserIDFromClaims(tc.claims)
			if ok != tc.expectedOK || id != tc.expectedID {
				t.Errorf("[%s] extractUserIDFromClaims() = (%v, %v), expected (%v, %v)",
					tc.name, id, ok, tc.expectedID, tc.expectedOK)
			}
		})
	}
}
