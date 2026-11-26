package middleware

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"fasttix-backend/config"
	"fasttix-backend/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// IsGlobalAdmin memeriksa apakah role pengguna adalah admin atau superadmin pada tingkat platform global.
// Hanya role 'admin' dan 'superadmin' yang sah.
func IsGlobalAdmin(role string) bool {
	r := strings.ToLower(strings.TrimSpace(role))
	return r == "admin" || r == "superadmin"
}

// ExtractUserIDFromClaims mengekstrak user_id dari JWT MapClaims secara aman.
// Mengembalikan (id, true) hanya jika id bertipe valid dan bernilai positif (> 0).
func ExtractUserIDFromClaims(claims jwt.MapClaims) (int, bool) {
	if val, ok := claims["user_id"]; ok && val != nil {
		var id int
		switch v := val.(type) {
		case float64:
			if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 || math.Trunc(v) != v {
				return 0, false
			}
			if v > float64(math.MaxInt) || v >= float64(uint64(1)<<63) {
				return 0, false
			}
			id = int(v)
		case int:
			id = v
		case int64:
			if v > int64(^uint(0)>>1) || v <= 0 {
				return 0, false
			}
			id = int(v)
		case string:
			parsed, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				return 0, false
			}
			id = parsed
		default:
			return 0, false
		}

		if id > 0 {
			return id, true
		}
	}
	return 0, false
}

// Protected memvalidasi access token JWT, memastikan algoritma HS256, klaim exp,
// token_type access, dan memverifikasi identitas serta role terbaru dari database.
func Protected(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Unauthorized: Missing Token",
		})
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" || strings.TrimSpace(parts[1]) == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Unauthorized: Invalid Token Format",
		})
	}

	tokenString := strings.TrimSpace(parts[1])
	secretKey := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secretKey == "" {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Internal server error: Konfigurasi keamanan belum diatur",
		})
	}

	// Parsing token dengan pembatasan ketat hanya pada algoritma HS256 dan kewajiban klaim exp
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v, expected HS256", token.Header["alg"])
		}
		return []byte(secretKey), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())

	if err != nil || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Unauthorized: Token tidak valid atau telah kadaluarsa",
		})
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Unauthorized: Klaim token tidak valid",
		})
	}

	// Wajibkan klaim exp
	if expVal, ok := claims["exp"]; !ok || expVal == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Unauthorized: Token wajib memiliki masa kadaluarsa (exp)",
		})
	}

	// Pastikan jenis token adalah access token
	tokenType, ok := claims["token_type"].(string)
	if !ok || tokenType != "access" {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Unauthorized: Token yang digunakan bukan access token",
		})
	}

	userID, hasUserID := ExtractUserIDFromClaims(claims)
	if !hasUserID || userID <= 0 {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Unauthorized: Klaim identitas pengguna tidak valid",
		})
	}

	// Wajibkan koneksi database aktif untuk verifikasi role terkini
	if config.DB == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Internal server error: Layanan database tidak tersedia",
		})
	}

	var user models.User
	err = config.DB.WithContext(c.UserContext()).
		Table("users").
		Select("users.id, users.nama, users.email, users.no_hp, users.role_id, roles.nama_role as role").
		Joins("JOIN roles ON roles.id = users.role_id").
		Where("users.id = ?", userID).
		First(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
				Message: "Unauthorized: Pengguna tidak ditemukan",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Terjadi kesalahan pada server saat memverifikasi sesi",
		})
	}

	freshRole := strings.ToLower(strings.TrimSpace(user.Role))

	// Simpan data otentikasi standar ke Fiber Locals
	c.Locals("user", claims)
	c.Locals("userID", user.ID)
	c.Locals("userRole", freshRole)
	c.Locals("currentUser", &user)

	return c.Next()
}

// AdminOnly membatasi akses hanya untuk pengguna dengan global role 'admin' atau 'superadmin'
func AdminOnly(c *fiber.Ctx) error {
	role, ok := c.Locals("userRole").(string)
	if !ok || strings.TrimSpace(role) == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Unauthorized: Sesi autentikasi tidak ditemukan",
		})
	}

	if !IsGlobalAdmin(role) {
		return c.Status(fiber.StatusForbidden).JSON(models.ErrorResponse{
			Message: "Forbidden: Akses ditolak. Hanya role 'admin' atau 'superadmin' yang diizinkan",
		})
	}

	return c.Next()
}

// AdminOrOrganizer membatasi akses untuk pengguna dengan global role 'admin', 'superadmin', atau 'organizer'
func AdminOrOrganizer(c *fiber.Ctx) error {
	role, ok := c.Locals("userRole").(string)
	if !ok || strings.TrimSpace(role) == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Unauthorized: Sesi autentikasi tidak ditemukan",
		})
	}

	roleLower := strings.ToLower(strings.TrimSpace(role))
	if !IsGlobalAdmin(roleLower) && roleLower != "organizer" {
		return c.Status(fiber.StatusForbidden).JSON(models.ErrorResponse{
			Message: "Forbidden: Akses ditolak. Hanya role 'admin', 'superadmin', atau 'organizer' yang diizinkan",
		})
	}

	return c.Next()
}
