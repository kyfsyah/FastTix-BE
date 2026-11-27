package controllers

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"unicode/utf8"

	"fasttix-backend/config"
	"fasttix-backend/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type createEventCategoryRequest struct {
	NamaKategori string  `json:"nama_kategori"`
	Slug         string  `json:"slug"`
	Deskripsi    *string `json:"deskripsi"`
}

type updateEventCategoryRequest struct {
	NamaKategori *string `json:"nama_kategori"`
	Slug         *string `json:"slug"`
	Deskripsi    *string `json:"deskripsi"`
}

type eventCategoryResponse struct {
	Message string                `json:"message"`
	Data    *models.EventCategory `json:"data,omitempty"`
}

type eventCategoryListResponse struct {
	Message string                 `json:"message"`
	Total   int64                  `json:"total"`
	Data    []models.EventCategory `json:"data"`
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	errLower := strings.ToLower(err.Error())
	return strings.Contains(errLower, "duplicate key") ||
		strings.Contains(errLower, "unique constraint") ||
		strings.Contains(errLower, "23505")
}

func isForeignKeyConstraintError(err error) bool {
	if err == nil {
		return false
	}
	errLower := strings.ToLower(err.Error())
	return strings.Contains(errLower, "violates foreign key constraint") ||
		strings.Contains(errLower, "foreign key") ||
		strings.Contains(errLower, "23503") ||
		strings.Contains(errLower, "fk_events_category")
}

// GetAllEventCategories - GET /api/event-categories
func GetAllEventCategories(c *fiber.Ctx) error {
	if config.DB == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Layanan database tidak tersedia",
		})
	}

	var categories []models.EventCategory
	var total int64

	if err := config.DB.WithContext(c.UserContext()).Model(&models.EventCategory{}).Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal menghitung data kategori event",
			Error:   err.Error(),
		})
	}

	if err := config.DB.WithContext(c.UserContext()).Order("id ASC").Find(&categories).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal mengambil data kategori event",
			Error:   err.Error(),
		})
	}

	return c.JSON(eventCategoryListResponse{
		Message: "Data kategori event berhasil diambil",
		Total:   total,
		Data:    categories,
	})
}

// GetEventCategoryByID - GET /api/event-categories/:id
func GetEventCategoryByID(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "ID kategori event tidak valid",
		})
	}

	if config.DB == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Layanan database tidak tersedia",
		})
	}

	var category models.EventCategory
	if err := config.DB.WithContext(c.UserContext()).First(&category, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
				Message: "Kategori event tidak ditemukan",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal mengambil data kategori event",
			Error:   err.Error(),
		})
	}

	return c.JSON(eventCategoryResponse{
		Message: "Detail kategori event berhasil diambil",
		Data:    &category,
	})
}

// CreateEventCategory - POST /api/event-categories (Admin only)
func CreateEventCategory(c *fiber.Ctx) error {
	req := new(createEventCategoryRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Format request tidak valid",
			Error:   err.Error(),
		})
	}

	cleanName := strings.TrimSpace(req.NamaKategori)
	if cleanName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Nama kategori wajib diisi",
		})
	}
	if utf8.RuneCountInString(cleanName) > 100 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Nama kategori maksimal 100 karakter",
		})
	}

	cleanSlug := strings.ToLower(strings.TrimSpace(req.Slug))
	if cleanSlug == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Slug kategori wajib diisi",
		})
	}
	if utf8.RuneCountInString(cleanSlug) > 100 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Slug kategori maksimal 100 karakter",
		})
	}

	deskripsi := ""
	if req.Deskripsi != nil {
		deskripsi = strings.TrimSpace(*req.Deskripsi)
	}

	if config.DB == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Layanan database tidak tersedia",
		})
	}

	var existingCount int64
	if err := config.DB.WithContext(c.UserContext()).Model(&models.EventCategory{}).
		Where("LOWER(nama_kategori) = ? OR LOWER(slug) = ?", strings.ToLower(cleanName), cleanSlug).
		Count(&existingCount).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal memeriksa duplikasi kategori event",
			Error:   err.Error(),
		})
	}
	if existingCount > 0 {
		return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{
			Message: "Nama kategori atau slug sudah digunakan",
		})
	}

	category := models.EventCategory{
		NamaKategori: cleanName,
		Slug:         cleanSlug,
		Deskripsi:    deskripsi,
	}

	if err := config.DB.WithContext(c.UserContext()).Create(&category).Error; err != nil {
		if isUniqueConstraintError(err) {
			return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{
				Message: "Nama kategori atau slug sudah digunakan",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal menambahkan kategori event",
			Error:   err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(eventCategoryResponse{
		Message: "Kategori event berhasil ditambahkan",
		Data:    &category,
	})
}

// UpdateEventCategory - PUT /api/event-categories/:id (Admin only)
func UpdateEventCategory(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "ID kategori event tidak valid",
		})
	}

	req := new(updateEventCategoryRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Format request tidak valid",
			Error:   err.Error(),
		})
	}


	if req.NamaKategori == nil && req.Slug == nil && req.Deskripsi == nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Minimal satu field harus dikirim untuk diperbarui",
		})
	}

	if config.DB == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Layanan database tidak tersedia",
		})
	}

	var category models.EventCategory
	if err := config.DB.WithContext(c.UserContext()).First(&category, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
				Message: "Kategori event tidak ditemukan",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal mencari data kategori event",
			Error:   err.Error(),
		})
	}

	updates := make(map[string]interface{})

	if req.NamaKategori != nil {
		cleanName := strings.TrimSpace(*req.NamaKategori)
		if cleanName == "" {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
				Message: "Nama kategori wajib diisi",
			})
		}
		if utf8.RuneCountInString(cleanName) > 100 {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
				Message: "Nama kategori maksimal 100 karakter",
			})
		}

		var count int64
		if err := config.DB.WithContext(c.UserContext()).Model(&models.EventCategory{}).
			Where("LOWER(nama_kategori) = ? AND id <> ?", strings.ToLower(cleanName), id).
			Count(&count).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
				Message: "Gagal memeriksa duplikasi nama kategori",
				Error:   err.Error(),
			})
		}
		if count > 0 {
			return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{
				Message: "Nama kategori sudah digunakan",
			})
		}

		updates["nama_kategori"] = cleanName
		category.NamaKategori = cleanName
	}

	if req.Slug != nil {
		cleanSlug := strings.ToLower(strings.TrimSpace(*req.Slug))
		if cleanSlug == "" {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
				Message: "Slug kategori wajib diisi",
			})
		}
		if utf8.RuneCountInString(cleanSlug) > 100 {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
				Message: "Slug kategori maksimal 100 karakter",
			})
		}

		var count int64
		if err := config.DB.WithContext(c.UserContext()).Model(&models.EventCategory{}).
			Where("LOWER(slug) = ? AND id <> ?", cleanSlug, id).
			Count(&count).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
				Message: "Gagal memeriksa duplikasi slug kategori",
				Error:   err.Error(),
			})
		}
		if count > 0 {
			return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{
				Message: "Slug kategori sudah digunakan",
			})
		}

		updates["slug"] = cleanSlug
		category.Slug = cleanSlug
	}

	if req.Deskripsi != nil {
		cleanDesc := strings.TrimSpace(*req.Deskripsi)
		updates["deskripsi"] = cleanDesc
		category.Deskripsi = cleanDesc
	}

	now := time.Now()
	updates["updated_at"] = now
	category.UpdatedAt = now

	if err := config.DB.WithContext(c.UserContext()).Model(&category).Updates(updates).Error; err != nil {
		if isUniqueConstraintError(err) {
			return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{
				Message: "Nama kategori atau slug sudah digunakan",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal memperbarui kategori event",
			Error:   err.Error(),
		})
	}

	return c.JSON(eventCategoryResponse{
		Message: "Kategori event berhasil diperbarui",
		Data:    &category,
	})
}

// DeleteEventCategory - DELETE /api/event-categories/:id (Admin only)
func DeleteEventCategory(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "ID kategori event tidak valid",
		})
	}

	if config.DB == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Layanan database tidak tersedia",
		})
	}

	var category models.EventCategory
	if err := config.DB.WithContext(c.UserContext()).First(&category, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
				Message: "Kategori event tidak ditemukan",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal mencari data kategori event",
			Error:   err.Error(),
		})
	}

	var eventCount int64
	if err := config.DB.WithContext(c.UserContext()).Table("events").Where("category_id = ?", id).Count(&eventCount).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal memeriksa relasi kategori pada event",
			Error:   err.Error(),
		})
	}
	if eventCount > 0 {
		return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{
			Message: "Kategori event tidak dapat dihapus karena masih digunakan oleh event",
		})
	}

	if err := config.DB.WithContext(c.UserContext()).Delete(&category).Error; err != nil {
		if isForeignKeyConstraintError(err) {
			return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{
				Message: "Kategori event tidak dapat dihapus karena masih digunakan oleh event",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal menghapus kategori event",
			Error:   err.Error(),
		})
	}

	return c.JSON(models.SuccessResponse{
		Message: "Kategori event berhasil dihapus",
	})
}
