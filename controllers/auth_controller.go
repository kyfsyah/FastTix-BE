package controllers

import (
	"os"
	"regexp"
	"time"

	"fasttix-backend/config"
	"fasttix-backend/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func Register(c *fiber.Ctx) error {
	req := new(models.RegisterRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Format request tidak valid",
			Error:   err.Error(),
		})
	}

	// Validation
	if req.Nama == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Nama wajib diisi"})
	}
	if req.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Email wajib diisi"})
	}
	if !isValidEmail(req.Email) {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Format email tidak valid"})
	}
	if req.Password == "" || len(req.Password) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Password minimal 8 karakter"})
	}

	// Check if email already exists using GORM
	var count int64
	if err := config.DB.Model(&models.User{}).Where("email = ?", req.Email).Count(&count).Error; err != nil {
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

	// Insert into DB with default role_id = 3 (customer)
	newUser := models.User{
		Nama:     req.Nama,
		Email:    req.Email,
		Password: string(hashedPassword),
		NoHP:     req.NoHP,
		RoleID:   3,
	}

	if err := config.DB.Create(&newUser).Error; err != nil {
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

	if req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{Message: "Email dan password wajib diisi"})
	}

	var user models.User
	var hashedPassword string

	// Query user with role name via GORM
	err := config.DB.Table("users").
		Select("users.id, users.nama, users.email, users.password, users.no_hp, users.role_id, r.nama_role as role").
		Joins("JOIN roles r ON users.role_id = r.id").
		Where("users.email = ?", req.Email).
		Row().Scan(&user.ID, &user.Nama, &user.Email, &hashedPassword, &user.NoHP, &user.RoleID, &user.Role)

	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Email atau password salah"})
	}

	// Compare passwords
	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password))
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Email atau password salah"})
	}

	// Generate JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"nama":    user.Nama,
		"email":   user.Email,
		"role_id": user.RoleID,
		"role":    user.Role,
		"exp":     time.Now().Add(time.Hour * 72).Unix(),
	})

	secretKey := os.Getenv("JWT_SECRET")
	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal membuat token",
			Error:   err.Error(),
		})
	}

	return c.JSON(models.LoginResponse{
		Message: "Login berhasil",
		Token:   tokenString,
		User:    user,
	})
}

func Profile(c *fiber.Ctx) error {
	// Get claims from context (set by auth middleware)
	userClaims, ok := c.Locals("user").(jwt.MapClaims)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Unauthorized: Klaim token tidak valid",
		})
	}

	userID := int(userClaims["user_id"].(float64))

	// Fetch fresh user data with role from DB using GORM
	var user models.User
	err := config.DB.Table("users").
		Select("users.id, users.nama, users.email, users.no_hp, users.role_id, r.nama_role as role").
		Joins("JOIN roles r ON users.role_id = r.id").
		Where("users.id = ?", userID).
		Scan(&user).Error

	if err != nil || user.ID == 0 {
		// Fallback to JWT claims if DB fetch fails
		user = models.User{
			ID:     userID,
			Nama:   userClaims["nama"].(string),
			Email:  userClaims["email"].(string),
			RoleID: int(userClaims["role_id"].(float64)),
			Role:   userClaims["role"].(string),
		}
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
