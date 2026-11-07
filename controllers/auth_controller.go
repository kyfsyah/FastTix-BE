package controllers

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"fasttix-backend/config"
	"fasttix-backend/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func getJWTSecret() (string, error) {
	secretKey := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secretKey == "" {
		return "", errors.New("JWT_SECRET belum dikonfigurasi pada environment variables")
	}
	return secretKey, nil
}

func extractUserIDFromClaims(claims jwt.MapClaims) (int, bool) {
	if val, ok := claims["user_id"]; ok && val != nil {
		switch v := val.(type) {
		case float64:
			return int(v), true
		case int:
			return v, true
		case int64:
			return int(v), true
		case string:
			if id, err := strconv.Atoi(v); err == nil {
				return id, true
			}
		}
	}
	return 0, false
}

func Register(c *fiber.Ctx) error {
	req := new(models.RegisterRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Format request tidak valid",
			Error:   err.Error(),
		})
	}

	// Validation
	if strings.TrimSpace(req.Nama) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Nama wajib diisi"})
	}
	if strings.TrimSpace(req.Email) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Email wajib diisi"})
	}
	if !isValidEmail(req.Email) {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Format email tidak valid"})
	}
	if req.Password == "" || len(req.Password) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Password minimal 8 karakter"})
	}

	// Check if email already exists using GORM with context
	var count int64
	if err := config.DB.WithContext(c.UserContext()).Model(&models.User{}).Where("email = ?", req.Email).Count(&count).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Terjadi kesalahan pada server",
			Error:   err.Error(),
		})
	}
	if count > 0 {
		return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{Message: "Email sudah digunakan"})
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal memproses password",
			Error:   err.Error(),
		})
	}

	// Query default role 'customer' / 'user' secara dinamis alih-alih hardcode ID 3
	var defaultRole models.Role
	roleID := 3 // fallback default jika lookup gagal
	if err := config.DB.WithContext(c.UserContext()).
		Where("LOWER(nama_role) IN ?", []string{"customer", "user"}).
		Order("id ASC").
		First(&defaultRole).Error; err == nil && defaultRole.ID > 0 {
		roleID = defaultRole.ID
	}

	newUser := models.User{
		Nama:     req.Nama,
		Email:    req.Email,
		Password: string(hashedPassword),
		NoHP:     req.NoHP,
		RoleID:   roleID,
	}

	if err := config.DB.WithContext(c.UserContext()).Create(&newUser).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal menyimpan user",
			Error:   err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(models.SuccessResponse{
		Message: "Registrasi berhasil",
	})
}

func Login(c *fiber.Ctx) error {
	req := new(models.LoginRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Format request tidak valid",
			Error:   err.Error(),
		})
	}

	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Password) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Email dan password wajib diisi"})
	}

	// Validasi format email saat login
	if !isValidEmail(req.Email) {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Format email tidak valid"})
	}

	var user models.User
	var hashedPassword string

	// Query user with role name via GORM with context
	err := config.DB.WithContext(c.UserContext()).Table("users").
		Select("users.id, users.nama, users.email, users.password, users.no_hp, users.role_id, r.nama_role as role").
		Joins("JOIN roles r ON users.role_id = r.id").
		Where("users.email = ?", req.Email).
		Row().Scan(&user.ID, &user.Nama, &user.Email, &hashedPassword, &user.NoHP, &user.RoleID, &user.Role)

	if err != nil {
		// Bedakan data tidak ditemukan (401) vs error server/database (500)
		if errors.Is(err, sql.ErrNoRows) {
			return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Email atau password salah"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Terjadi kesalahan pada server saat memproses login",
			Error:   err.Error(),
		})
	}

	// Compare passwords
	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Email atau password salah"})
	}

	// Generate Access and Refresh Tokens
	accessToken, refreshToken, err := generateTokens(user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal membuat token autentikasi",
			Error:   err.Error(),
		})
	}

	return c.JSON(models.LoginResponse{
		Message:      "Login berhasil",
		Token:        accessToken,
		RefreshToken: refreshToken,
		User:         user,
	})
}

// RefreshToken - POST /api/refresh (Tukar refresh token lama dengan access token baru)
func RefreshToken(c *fiber.Ctx) error {
	req := new(models.RefreshRequest)
	if err := c.BodyParser(req); err != nil || strings.TrimSpace(req.RefreshToken) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Format request tidak valid atau refresh_token kosong",
		})
	}

	secretKey, err := getJWTSecret()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Konfigurasi server JWT_SECRET belum diatur",
			Error:   err.Error(),
		})
	}

	token, err := jwt.Parse(req.RefreshToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	})

	if err != nil || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Refresh token tidak valid atau telah kadaluarsa",
		})
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Klaim token tidak valid",
		})
	}

	// Pastikan jenis token adalah refresh token
	if tokenType, ok := claims["token_type"].(string); !ok || tokenType != "refresh" {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Token yang diberikan bukan refresh token yang sah",
		})
	}

	userID, ok := extractUserIDFromClaims(claims)
	if !ok || userID <= 0 {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Data user pada token tidak valid",
		})
	}

	// Ambil data user terkini dari database with context
	var user models.User
	err = config.DB.WithContext(c.UserContext()).Table("users").
		Select("users.id, users.nama, users.email, users.no_hp, users.role_id, r.nama_role as role").
		Joins("JOIN roles r ON users.role_id = r.id").
		Where("users.id = ?", userID).
		Scan(&user).Error

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Terjadi kesalahan pada server saat memverifikasi pengguna",
			Error:   err.Error(),
		})
	}

	if user.ID == 0 {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Pengguna tidak ditemukan atau telah dinonaktifkan",
		})
	}

	// Buat access token & refresh token baru
	newAccessToken, newRefreshToken, err := generateTokens(user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal memperbarui token",
			Error:   err.Error(),
		})
	}

	return c.JSON(models.RefreshResponse{
		Message:      "Token berhasil diperbarui",
		Token:        newAccessToken,
		RefreshToken: newRefreshToken,
	})
}

func getAccessExpDuration() time.Duration {
	val := os.Getenv("JWT_ACCESS_EXP_MINUTES")
	if minutes, err := strconv.Atoi(val); err == nil && minutes > 0 {
		return time.Duration(minutes) * time.Minute
	}
	return 15 * time.Minute // default 15 menit
}

func getRefreshExpDuration() time.Duration {
	val := os.Getenv("JWT_REFRESH_EXP_DAYS")
	if days, err := strconv.Atoi(val); err == nil && days > 0 {
		return time.Duration(days) * 24 * time.Hour
	}
	return 7 * 24 * time.Hour // default 7 hari
}

func generateTokens(user models.User) (string, string, error) {
	secretKey, err := getJWTSecret()
	if err != nil {
		return "", "", err
	}

	// 1. Access Token (masa aktif menit, default 15 menit)
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":    user.ID,
		"nama":       user.Nama,
		"email":      user.Email,
		"role_id":    user.RoleID,
		"role":       user.Role,
		"token_type": "access",
		"exp":        time.Now().Add(getAccessExpDuration()).Unix(),
	})

	accessString, err := accessToken.SignedString([]byte(secretKey))
	if err != nil {
		return "", "", err
	}

	// 2. Refresh Token (masa aktif hari, default 7 hari)
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":    user.ID,
		"token_type": "refresh",
		"exp":        time.Now().Add(getRefreshExpDuration()).Unix(),
	})

	refreshString, err := refreshToken.SignedString([]byte(secretKey))
	if err != nil {
		return "", "", err
	}

	return accessString, refreshString, nil
}

func Profile(c *fiber.Ctx) error {
	// Get claims from context (set by auth middleware)
	userClaims, ok := c.Locals("user").(jwt.MapClaims)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Unauthorized: Klaim token tidak valid",
		})
	}

	userID, ok := extractUserIDFromClaims(userClaims)
	if !ok || userID <= 0 {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Unauthorized: ID pengguna pada token tidak valid",
		})
	}

	// Fetch fresh user data with role directly from DB using GORM with context
	var user models.User
	err := config.DB.WithContext(c.UserContext()).Table("users").
		Select("users.id, users.nama, users.email, users.no_hp, users.role_id, r.nama_role as role").
		Joins("JOIN roles r ON users.role_id = r.id").
		Where("users.id = ?", userID).
		Scan(&user).Error

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Terjadi kesalahan pada server saat mengambil profil pengguna",
			Error:   err.Error(),
		})
	}

	if user.ID == 0 {
		return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
			Message: "Pengguna tidak ditemukan",
		})
	}

	return c.JSON(models.ProfileResponse{
		Message: "Profile berhasil diakses",
		User:    user,
	})
}

func isValidEmail(email string) bool {
	regex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return regex.MatchString(email)
}
