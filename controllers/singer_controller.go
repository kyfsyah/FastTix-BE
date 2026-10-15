package controllers

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"fasttix-backend/config"
	"fasttix-backend/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// CreateSinger - POST /api/singers (Admin only)
func CreateSinger(c *fiber.Ctx) error {
	req := new(models.SingerRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Format request tidak valid",
			Error:   err.Error(),
		})
	}

	if strings.TrimSpace(req.NamaPenyanyi) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Nama penyanyi wajib diisi",
		})
	}

	if req.TanggalLahir != nil && *req.TanggalLahir != "" {
		if _, err := time.Parse("2006-01-02", *req.TanggalLahir); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
				Message: "Format tanggal_lahir tidak valid. Gunakan format YYYY-MM-DD (contoh: 1995-12-30)",
			})
		}
	}

	singer := models.Singer{
		NamaPenyanyi: req.NamaPenyanyi,
		NamaPanggung: req.NamaPanggung,
		Genre:        req.Genre,
		TanggalLahir: req.TanggalLahir,
		Asal:         req.Asal,
		Deskripsi:    req.Deskripsi,
		Foto:         req.Foto,
	}

	if err := config.DB.Create(&singer).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal menambahkan penyanyi",
			Error:   err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(models.SingerResponse{
		Message: "Penyanyi berhasil ditambahkan",
		Data:    &singer,
	})
}

// GetSingers - GET /api/singers
func GetSingers(c *fiber.Ctx) error {
	var singers []models.Singer
	var total int64

	if err := config.DB.Model(&models.Singer{}).Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal menghitung data penyanyi",
			Error:   err.Error(),
		})
	}

	if err := config.DB.Order("id ASC").Find(&singers).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal mengambil data penyanyi",
			Error:   err.Error(),
		})
	}

	return c.JSON(models.SingerListResponse{
		Message: "Data penyanyi berhasil diambil",
		Total:   total,
		Data:    singers,
	})
}

// GetSingerByID - GET /api/singers/:id
func GetSingerByID(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "ID penyanyi tidak valid",
		})
	}

	var singer models.Singer
	if err := config.DB.First(&singer, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
				Message: "Penyanyi tidak ditemukan",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal mengambil data penyanyi",
			Error:   err.Error(),
		})
	}

	return c.JSON(models.SingerResponse{
		Message: "Detail penyanyi berhasil diambil",
		Data:    &singer,
	})
}

// UpdateSinger - PUT /api/singers/:id (Admin only)
func UpdateSinger(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "ID penyanyi tidak valid",
		})
	}

	var singer models.Singer
	if err := config.DB.First(&singer, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
				Message: "Penyanyi tidak ditemukan",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal mencari data penyanyi",
			Error:   err.Error(),
		})
	}

	req := new(models.SingerRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Format request tidak valid",
			Error:   err.Error(),
		})
	}

	if strings.TrimSpace(req.NamaPenyanyi) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Nama penyanyi wajib diisi",
		})
	}

	if req.TanggalLahir != nil && *req.TanggalLahir != "" {
		if _, err := time.Parse("2006-01-02", *req.TanggalLahir); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
				Message: "Format tanggal_lahir tidak valid. Gunakan format YYYY-MM-DD (contoh: 1995-12-30)",
			})
		}
	}

	// Update fields
	singer.NamaPenyanyi = req.NamaPenyanyi
	singer.NamaPanggung = req.NamaPanggung
	singer.Genre = req.Genre
	singer.TanggalLahir = req.TanggalLahir
	singer.Asal = req.Asal
	singer.Deskripsi = req.Deskripsi
	singer.Foto = req.Foto
	singer.UpdatedAt = time.Now()

	if err := config.DB.Save(&singer).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal memperbarui data penyanyi",
			Error:   err.Error(),
		})
	}

	return c.JSON(models.SingerResponse{
		Message: "Penyanyi berhasil diperbarui",
		Data:    &singer,
	})
}

// DeleteSinger - DELETE /api/singers/:id (Admin only)
func DeleteSinger(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "ID penyanyi tidak valid",
		})
	}

	var singer models.Singer
	if err := config.DB.First(&singer, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
				Message: "Penyanyi tidak ditemukan",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal mencari data penyanyi",
			Error:   err.Error(),
		})
	}

	if err := config.DB.Delete(&singer).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "violates foreign key constraint") ||
			strings.Contains(strings.ToLower(err.Error()), "foreign key") {
			return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{
				Message: "Penyanyi tidak dapat dihapus karena masih terdaftar pada konser/event",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal menghapus penyanyi",
			Error:   err.Error(),
		})
	}

	return c.JSON(models.SuccessResponse{
		Message: "Penyanyi berhasil dihapus",
	})
}
