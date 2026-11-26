package middleware

import (
	"math"
	"net/http/httptest"
	"testing"
	"time"

	"fasttix-backend/config"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func TestIsGlobalAdmin(t *testing.T) {
	tests := []struct {
		role     string
		expected bool
	}{
		{"admin", true},
		{"Admin", true},
		{"ADMIN", true},
		{"superadmin", true},
		{"SuperAdmin", true},
		{"SUPERADMIN", true},
		{"super_admin", false}, // Bukan alias resmi
		{"SUPER_ADMIN", false},
		{"organizer", false},
		{"customer", false},
		{"user", false},
		{"", false},
		{"   ", false},
	}

	for _, tc := range tests {
		t.Run(tc.role, func(t *testing.T) {
			got := IsGlobalAdmin(tc.role)
			if got != tc.expected {
				t.Errorf("IsGlobalAdmin(%q) = %v, expected %v", tc.role, got, tc.expected)
			}
		})
	}
}

func TestExtractUserIDFromClaims(t *testing.T) {
	tests := []struct {
		name       string
		claims     jwt.MapClaims
		expectedID int
		expectedOK bool
	}{
		{"float64 valid", jwt.MapClaims{"user_id": float64(10)}, 10, true},
		{"float64 zero", jwt.MapClaims{"user_id": float64(0)}, 0, false},
		{"float64 negative", jwt.MapClaims{"user_id": float64(-5)}, 0, false},
		{"float64 non-integer", jwt.MapClaims{"user_id": float64(10.5)}, 0, false},
		{"float64 overflow", jwt.MapClaims{"user_id": float64(1e30)}, 0, false},
		{"float64 max int64 boundary overflow", jwt.MapClaims{"user_id": float64(uint64(1) << 63)}, 0, false},
		{"float64 +Inf", jwt.MapClaims{"user_id": math.Inf(1)}, 0, false},
		{"float64 NaN", jwt.MapClaims{"user_id": math.NaN()}, 0, false},
		{"int valid", jwt.MapClaims{"user_id": 20}, 20, true},
		{"int zero", jwt.MapClaims{"user_id": 0}, 0, false},
		{"int negative", jwt.MapClaims{"user_id": -20}, 0, false},
		{"int64 valid", jwt.MapClaims{"user_id": int64(30)}, 30, true},
		{"int64 zero", jwt.MapClaims{"user_id": int64(0)}, 0, false},
		{"int64 negative", jwt.MapClaims{"user_id": int64(-30)}, 0, false},
		{"string valid", jwt.MapClaims{"user_id": "40"}, 40, true},
		{"string zero", jwt.MapClaims{"user_id": "0"}, 0, false},
		{"string negative", jwt.MapClaims{"user_id": "-40"}, 0, false},
		{"string invalid", jwt.MapClaims{"user_id": "abc"}, 0, false},
		{"string overflow", jwt.MapClaims{"user_id": "99999999999999999999999999999"}, 0, false},
		{"nil", jwt.MapClaims{"user_id": nil}, 0, false},
		{"missing", jwt.MapClaims{}, 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, ok := ExtractUserIDFromClaims(tc.claims)
			if ok != tc.expectedOK || id != tc.expectedID {
				t.Errorf("[%s] ExtractUserIDFromClaims() = (%d, %v), expected (%d, %v)",
					tc.name, id, ok, tc.expectedID, tc.expectedOK)
			}
		})
	}
}

func TestAdminOnlyMiddleware(t *testing.T) {
	tests := []struct {
		name         string
		setupContext func(c *fiber.Ctx)
		expectedCode int
	}{
		{
			name: "Admin role passes",
			setupContext: func(c *fiber.Ctx) {
				c.Locals("userRole", "admin")
			},
			expectedCode: fiber.StatusOK,
		},
		{
			name: "Superadmin role passes",
			setupContext: func(c *fiber.Ctx) {
				c.Locals("userRole", "superadmin")
			},
			expectedCode: fiber.StatusOK,
		},
		{
			name: "Customer role forbidden",
			setupContext: func(c *fiber.Ctx) {
				c.Locals("userRole", "customer")
			},
			expectedCode: fiber.StatusForbidden,
		},
		{
			name: "Organizer role forbidden from AdminOnly",
			setupContext: func(c *fiber.Ctx) {
				c.Locals("userRole", "organizer")
			},
			expectedCode: fiber.StatusForbidden,
		},
		{
			name: "super_admin role rejected because it is not official",
			setupContext: func(c *fiber.Ctx) {
				c.Locals("userRole", "super_admin")
			},
			expectedCode: fiber.StatusForbidden,
		},
		{
			name:         "No auth context unauthorized",
			setupContext: func(c *fiber.Ctx) {},
			expectedCode: fiber.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			subApp := fiber.New()
			subApp.Use(func(c *fiber.Ctx) error {
				tc.setupContext(c)
				return c.Next()
			})
			subApp.Get("/test", AdminOnly, func(c *fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			resp, err := subApp.Test(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.StatusCode != tc.expectedCode {
				t.Errorf("[%s] status = %d, expected %d", tc.name, resp.StatusCode, tc.expectedCode)
			}
		})
	}
}

func TestAdminOrOrganizerMiddleware(t *testing.T) {
	tests := []struct {
		name         string
		role         string
		hasContext   bool
		expectedCode int
	}{
		{"Admin passes", "admin", true, fiber.StatusOK},
		{"Superadmin passes", "superadmin", true, fiber.StatusOK},
		{"Organizer passes", "organizer", true, fiber.StatusOK},
		{"Customer forbidden", "customer", true, fiber.StatusForbidden},
		{"Random role forbidden", "random_role", true, fiber.StatusForbidden},
		{"super_admin alias forbidden", "super_admin", true, fiber.StatusForbidden},
		{"No auth context unauthorized", "", false, fiber.StatusUnauthorized},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			app.Use(func(c *fiber.Ctx) error {
				if tc.hasContext {
					c.Locals("userRole", tc.role)
				}
				return c.Next()
			})
			app.Get("/test", AdminOrOrganizer, func(c *fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.StatusCode != tc.expectedCode {
				t.Errorf("[%s] status = %d, expected %d", tc.name, resp.StatusCode, tc.expectedCode)
			}
		})
	}
}

func TestProtectedMiddleware_EdgeCases(t *testing.T) {
	secret := "test-secret-key-for-jwt-testing-123456"
	t.Setenv("JWT_SECRET", secret)

	app := fiber.New()
	app.Get("/protected", Protected, func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	// 1. Missing Authorization header -> 401
	req := httptest.NewRequest("GET", "/protected", nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("expected 401 for missing token, got %d", resp.StatusCode)
	}

	// 2. Invalid Authorization format -> 401
	invalidAuthHeaders := []string{
		"InvalidHeaderFormat",
		"Basic some-token",
		"Bearer   ",
		"Bearer token extra",
	}
	for _, header := range invalidAuthHeaders {
		req = httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", header)
		resp, _ = app.Test(req)
		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Errorf("expected 401 for invalid format %q, got %d", header, resp.StatusCode)
		}
	}

	// 3. Token malformed -> 401
	req = httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer not.a.valid.jwt.token")
	resp, _ = app.Test(req)
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("expected 401 for malformed token, got %d", resp.StatusCode)
	}

	// 4. Token expired -> 401
	expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":    1,
		"token_type": "access",
		"exp":        time.Now().Add(-1 * time.Hour).Unix(),
	})
	expiredStr, _ := expiredToken.SignedString([]byte(secret))
	req = httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+expiredStr)
	resp, _ = app.Test(req)
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("expected 401 for expired token, got %d", resp.StatusCode)
	}

	// 5. Token without exp claim -> 401
	noExpToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":    1,
		"token_type": "access",
	})
	noExpStr, _ := noExpToken.SignedString([]byte(secret))
	req = httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+noExpStr)
	resp, _ = app.Test(req)
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("expected 401 for token without exp claim, got %d", resp.StatusCode)
	}

	// 6. Refresh token used on protected route -> 401
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":    1,
		"token_type": "refresh",
		"exp":        time.Now().Add(time.Hour).Unix(),
	})
	refreshStr, _ := refreshToken.SignedString([]byte(secret))
	req = httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+refreshStr)
	resp, _ = app.Test(req)
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("expected 401 for refresh token on protected route, got %d", resp.StatusCode)
	}

	// 7. Non-HS256 algorithm (None / RS256 signing attempt) -> 401
	noneToken := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"user_id":    1,
		"token_type": "access",
		"exp":        time.Now().Add(time.Hour).Unix(),
	})
	noneStr, _ := noneToken.SignedString(jwt.UnsafeAllowNoneSignatureType)
	req = httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+noneStr)
	resp, _ = app.Test(req)
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("expected 401 for non-HS256 token, got %d", resp.StatusCode)
	}

	// 8. HS256 token signed with wrong secret -> 401
	wrongSecretToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":    1,
		"token_type": "access",
		"exp":        time.Now().Add(time.Hour).Unix(),
	})
	wrongSecretStr, _ := wrongSecretToken.SignedString([]byte("wrong-secret-key-for-jwt-testing-999999"))
	req = httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+wrongSecretStr)
	resp, _ = app.Test(req)
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("expected 401 for HS256 token with wrong secret, got %d", resp.StatusCode)
	}

	// 8. Database is nil when verifying user -> 500
	savedDB := config.DB
	config.DB = nil
	defer func() { config.DB = savedDB }()

	validToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":    1,
		"token_type": "access",
		"exp":        time.Now().Add(time.Hour).Unix(),
	})
	validStr, _ := validToken.SignedString([]byte(secret))
	req = httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+validStr)
	resp, _ = app.Test(req)
	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Errorf("expected 500 when database is nil, got %d", resp.StatusCode)
	}
}
