package controllers

import (
	"os"
	"testing"
	"time"
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
