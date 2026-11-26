package controllers

import (
	"database/sql"
	"errors"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"fasttix-backend/config"
	"fasttix-backend/middleware"
	"fasttix-backend/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func getJWTSecret() (string, error) {
	secretKey := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secretKey == "" {
		return "", errors.New("JWT_SECRET belum dikonfigurasi pada environment variables")
	}
	return secretKey, nil
}

func extractUserIDFromClaims(claims jwt.MapClaims) (int, bool) {
	return middleware.ExtractUserIDFromClaims(claims)
}

func Register(c *fiber.Ctx) error {
	req := new(models.RegisterRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Format request tidak valid",
		})
	}

	// Validasi nama
	cleanName := strings.TrimSpace(req.Nama)
	if cleanName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Nama wajib diisi",
		})
	}

	// Normalisasi email
	cleanEmail := strings.ToLower(strings.TrimSpace(req.Email))
	if cleanEmail == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Email wajib diisi",
		})
	}
	if !isValidEmail(cleanEmail) {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Format email tidak valid",
		})
	}

	// Validasi nomor HP jika disertakan (kolom no_hp di database bersifat nullable)
	var noHP *string
	if req.NoHP != nil && strings.TrimSpace(*req.NoHP) != "" {
		cleanedPhone := strings.TrimSpace(*req.NoHP)
		if !isValidPhone(cleanedPhone) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
				Message: "Format nomor HP tidak valid (minimal 10-15 digit angka)",
			})
		}
		noHP = &cleanedPhone
	}

	// Validasi password (tanpa mengubah password asli dengan TrimSpace sebelum hashing)
	if strings.TrimSpace(req.Password) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Password wajib diisi",
		})
	}
	if utf8.RuneCountInString(req.Password) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Password minimal 8 karakter",
		})
	}
	if len(req.Password) > 72 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Password maksimal 72 byte",
		})
	}

	if config.DB == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Layanan database tidak tersedia",
		})
	}

	// 1. Pengecekan awal email terdaftar
	var count int64
	if err := config.DB.WithContext(c.UserContext()).Model(&models.User{}).
		Where("LOWER(email) = ?", cleanEmail).Count(&count).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Terjadi kesalahan pada server saat memproses registrasi",
		})
	}
	if count > 0 {
		return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{
			Message: "Email sudah digunakan",
		})
	}

	// 2. Hash password menggunakan bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal memproses keamanan password",
		})
	}

	// 3. Lookup role 'customer' dari tabel roles (tanpa hardcode ID fallback)
	var customerRole models.Role
	if err := config.DB.WithContext(c.UserContext()).
		Where("LOWER(nama_role) = ?", "customer").
		First(&customerRole).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal memproses registrasi: Role default tidak ditemukan pada sistem",
		})
	}

	newUser := models.User{
		Nama:     cleanName,
		Email:    cleanEmail,
		Password: string(hashedPassword),
		NoHP:     noHP,
		RoleID:   customerRole.ID,
	}

	// 4. Insert user ke database dengan penanganan race condition (unique constraint)
	if err := config.DB.WithContext(c.UserContext()).Create(&newUser).Error; err != nil {
		errLower := strings.ToLower(err.Error())
		if strings.Contains(errLower, "duplicate key") ||
			strings.Contains(errLower, "unique constraint") ||
			strings.Contains(errLower, "23505") {
			return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{
				Message: "Email sudah digunakan",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal menyimpan data pengguna",
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
		})
	}

	cleanEmail := strings.ToLower(strings.TrimSpace(req.Email))
	if cleanEmail == "" || strings.TrimSpace(req.Password) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Email dan password wajib diisi",
		})
	}

	if !isValidEmail(cleanEmail) {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Format email tidak valid",
		})
	}

	if config.DB == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Layanan database tidak tersedia",
		})
	}

	var user models.User
	var hashedPassword string

	// Query user dengan role terbaru dari database
	err := config.DB.WithContext(c.UserContext()).Table("users").
		Select("users.id, users.nama, users.email, users.password, users.no_hp, users.role_id, roles.nama_role as role").
		Joins("JOIN roles ON roles.id = users.role_id").
		Where("LOWER(users.email) = ?", cleanEmail).
		Row().Scan(&user.ID, &user.Nama, &user.Email, &hashedPassword, &user.NoHP, &user.RoleID, &user.Role)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Pesan umum untuk mencegah kebocoran informasi apakah email terdaftar
			return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
				Message: "Email atau password salah",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Terjadi kesalahan pada server saat memproses login",
		})
	}

	// Bandingkan password bcrypt
	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Email atau password salah",
		})
	}

	// Normalisasi role pengguna
	user.Role = strings.ToLower(strings.TrimSpace(user.Role))

	// Buat access token dan refresh token
	accessToken, refreshToken, err := generateTokens(user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal membuat sesi autentikasi",
		})
	}

	return c.JSON(models.LoginResponse{
		Message:      "Login berhasil",
		Token:        accessToken,
		RefreshToken: refreshToken,
		User:         user,
	})
}

// RefreshToken - POST /api/refresh (Menukar refresh token lama dengan access token baru)
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
			Message: "Konfigurasi server keamanan belum diatur",
		})
	}

	// Parsing refresh token dengan validasi ketat algoritma HS256 dan kewajiban klaim exp
	token, err := jwt.Parse(strings.TrimSpace(req.RefreshToken), func(token *jwt.Token) (interface{}, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("algoritma signing tidak valid")
		}
		return []byte(secretKey), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())

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

	// Validasi klaim exp
	if expVal, ok := claims["exp"]; !ok || expVal == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Refresh token wajib memiliki masa kadaluarsa (exp)",
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
			Message: "Data pengguna pada token tidak valid",
		})
	}

	if config.DB == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Layanan database tidak tersedia",
		})
	}

	// Ambil data dan role pengguna terbaru langsung dari database
	var user models.User
	err = config.DB.WithContext(c.UserContext()).Table("users").
		Select("users.id, users.nama, users.email, users.no_hp, users.role_id, roles.nama_role as role").
		Joins("JOIN roles ON roles.id = users.role_id").
		Where("users.id = ?", userID).
		First(&user).Error

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
				Message: "Pengguna tidak ditemukan",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Terjadi kesalahan pada server saat memverifikasi pengguna",
		})
	}

	user.Role = strings.ToLower(strings.TrimSpace(user.Role))

	// Buat access token & refresh token baru
	newAccessToken, newRefreshToken, err := generateTokens(user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal memperbarui sesi autentikasi",
		})
	}

	return c.JSON(models.RefreshResponse{
		Message:      "Token berhasil diperbarui",
		Token:        newAccessToken,
		RefreshToken: newRefreshToken,
	})
}

const (
	maxAccessExpMinutes = math.MaxInt64 / int64(time.Minute)
	maxRefreshExpDays   = math.MaxInt64 / int64(24*time.Hour)
)

func getAccessExpDuration() time.Duration {
	val := strings.TrimSpace(os.Getenv("JWT_ACCESS_EXP_MINUTES"))
	if minutes, err := strconv.ParseInt(val, 10, 64); err == nil && minutes > 0 && minutes <= maxAccessExpMinutes {
		return time.Duration(minutes) * time.Minute
	}
	return 15 * time.Minute // default 15 menit
}

func getRefreshExpDuration() time.Duration {
	val := strings.TrimSpace(os.Getenv("JWT_REFRESH_EXP_DAYS"))
	if days, err := strconv.ParseInt(val, 10, 64); err == nil && days > 0 && days <= maxRefreshExpDays {
		return time.Duration(days) * 24 * time.Hour
	}
	return 7 * 24 * time.Hour // default 7 hari
}

func generateTokens(user models.User) (string, string, error) {
	secretKey, err := getJWTSecret()
	if err != nil {
		return "", "", err
	}

	now := time.Now()

	// 1. Access Token (membawa identitas & role terbaru, masa aktif default 15 menit)
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":    user.ID,
		"nama":       user.Nama,
		"email":      user.Email,
		"role_id":    user.RoleID,
		"role":       user.Role,
		"token_type": "access",
		"iat":        now.Unix(),
		"exp":        now.Add(getAccessExpDuration()).Unix(),
	})

	accessString, err := accessToken.SignedString([]byte(secretKey))
	if err != nil {
		return "", "", err
	}

	// 2. Refresh Token (hanya user_id & token_type, masa aktif default 7 hari)
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":    user.ID,
		"token_type": "refresh",
		"iat":        now.Unix(),
		"exp":        now.Add(getRefreshExpDuration()).Unix(),
	})

	refreshString, err := refreshToken.SignedString([]byte(secretKey))
	if err != nil {
		return "", "", err
	}

	return accessString, refreshString, nil
}

func Profile(c *fiber.Ctx) error {
	// Ambil userID dari Fiber Locals (telah divalidasi oleh middleware Protected dari database)
	userID, ok := c.Locals("userID").(int)
	if !ok || userID <= 0 {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Unauthorized: Sesi pengguna tidak valid",
		})
	}

	if config.DB == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Layanan database tidak tersedia",
		})
	}

	// Ambil profil data user terbaru dari database
	var user models.User
	err := config.DB.WithContext(c.UserContext()).Table("users").
		Select("users.id, users.nama, users.email, users.no_hp, users.role_id, roles.nama_role as role, users.created_at, users.updated_at").
		Joins("JOIN roles ON roles.id = users.role_id").
		Where("users.id = ?", userID).
		First(&user).Error

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
				Message: "Pengguna tidak ditemukan",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Terjadi kesalahan pada server saat mengambil profil pengguna",
		})
	}

	user.Role = strings.ToLower(strings.TrimSpace(user.Role))

	return c.JSON(models.ProfileResponse{
		Message: "Profile berhasil diakses",
		User:    user,
	})
}

func isValidEmail(email string) bool {
	regex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return regex.MatchString(email)
}

func isValidPhone(phone string) bool {
	phone = strings.TrimSpace(phone)
	regex := regexp.MustCompile(`^(\+?[0-9]{10,15})$`)
	return regex.MatchString(phone)
}
