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

// CreateBand - POST /api/bands (Admin only)
func CreateBand(c *fiber.Ctx) error {
	req := new(models.BandRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Format request tidak valid",
			Error:   err.Error(),
		})
	}

	if strings.TrimSpace(req.NamaBand) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Nama band wajib diisi",
		})
	}

	if req.TahunBerdiri != nil {
		currentYear := time.Now().Year()
		if *req.TahunBerdiri < 1800 || *req.TahunBerdiri > currentYear+1 {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
				Message: "Tahun berdiri tidak realistis",
			})
		}
	}

	band := models.Band{
		NamaBand:     req.NamaBand,
		Genre:        req.Genre,
		TahunBerdiri: req.TahunBerdiri,
		Asal:         req.Asal,
		Deskripsi:    req.Deskripsi,
		Foto:         req.Foto,
	}

	if err := config.DB.Create(&band).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal menambahkan band",
			Error:   err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(models.BandResponse{
		Message: "Band berhasil ditambahkan",
		Data:    &band,
	})
}

// GetBands - GET /api/bands
func GetBands(c *fiber.Ctx) error {
	var bands []models.Band
	var total int64

	if err := config.DB.Model(&models.Band{}).Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal menghitung data band",
			Error:   err.Error(),
		})
	}

	if err := config.DB.Order("id ASC").Find(&bands).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal mengambil data band",
			Error:   err.Error(),
		})
	}

	return c.JSON(models.BandListResponse{
		Message: "Data band berhasil diambil",
		Total:   total,
		Data:    bands,
	})
}

// GetBandByID - GET /api/bands/:id
func GetBandByID(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "ID band tidak valid",
		})
	}

	var band models.Band
	if err := config.DB.First(&band, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
				Message: "Band tidak ditemukan",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal mengambil data band",
			Error:   err.Error(),
		})
	}

	return c.JSON(models.BandResponse{
		Message: "Detail band berhasil diambil",
		Data:    &band,
	})
}

// UpdateBand - PUT /api/bands/:id (Admin only)
func UpdateBand(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "ID band tidak valid",
		})
	}

	var band models.Band
	if err := config.DB.First(&band, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
				Message: "Band tidak ditemukan",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal mencari data band",
			Error:   err.Error(),
		})
	}

	req := new(models.BandRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Format request tidak valid",
			Error:   err.Error(),
		})
	}

	if strings.TrimSpace(req.NamaBand) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Nama band wajib diisi",
		})
	}

	if req.TahunBerdiri != nil {
		currentYear := time.Now().Year()
		if *req.TahunBerdiri < 1800 || *req.TahunBerdiri > currentYear+1 {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
				Message: "Tahun berdiri tidak realistis",
			})
		}
	}

	band.NamaBand = req.NamaBand
	band.Genre = req.Genre
	band.TahunBerdiri = req.TahunBerdiri
	band.Asal = req.Asal
	band.Deskripsi = req.Deskripsi
	band.Foto = req.Foto
	band.UpdatedAt = time.Now()

	if err := config.DB.Save(&band).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal memperbarui data band",
			Error:   err.Error(),
		})
	}

	return c.JSON(models.BandResponse{
		Message: "Band berhasil diperbarui",
		Data:    &band,
	})
}

// DeleteBand - DELETE /api/bands/:id (Admin only)
func DeleteBand(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "ID band tidak valid",
		})
	}

	var band models.Band
	if err := config.DB.First(&band, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
				Message: "Band tidak ditemukan",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal mencari data band",
			Error:   err.Error(),
		})
	}

	if err := config.DB.Delete(&band).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "violates foreign key constraint") ||
			strings.Contains(strings.ToLower(err.Error()), "foreign key") {
			return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{
				Message: "Band tidak dapat dihapus karena masih terdaftar pada konser/event",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal menghapus band",
			Error:   err.Error(),
		})
	}

	return c.JSON(models.SuccessResponse{
		Message: "Band berhasil dihapus",
	})
}
