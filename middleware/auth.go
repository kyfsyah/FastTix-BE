package middleware

import (
	"fmt"
	"os"
	"strings"

	"fasttix-backend/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func Protected(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Unauthorized: Missing Token",
		})
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Unauthorized: Invalid Token Format",
		})
	}

	tokenString := parts[1]
	secretKey := os.Getenv("JWT_SECRET")

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	})

	if err != nil || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Unauthorized: Invalid Token",
		})
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// Pastikan token yang digunakan adalah access token
		if tokenType, ok := claims["token_type"].(string); ok && tokenType != "access" {
			return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
				Message: "Unauthorized: Token yang digunakan bukan access token",
			})
		}
		// Set user claims in context
		c.Locals("user", claims)
		return c.Next()
	}

	return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
		Message: "Unauthorized",
	})
}

// AdminOnly membatasi akses hanya untuk pengguna dengan role 'admin' atau 'super_admin'
func AdminOnly(c *fiber.Ctx) error {
	userClaims, ok := c.Locals("user").(jwt.MapClaims)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{
			Message: "Unauthorized: Klaim token tidak ditemukan",
		})
	}

	role, ok := userClaims["role"].(string)
	if !ok || (role != "admin" && role != "super_admin") {
		return c.Status(fiber.StatusForbidden).JSON(models.ErrorResponse{
			Message: "Forbidden: Akses ditolak. Hanya role 'admin' atau 'super_admin' yang diizinkan",
		})
	}

	return c.Next()
}
