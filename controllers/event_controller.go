package controllers

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"fasttix-backend/config"
	"fasttix-backend/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

var validEventStatuses = map[string]bool{
	"draft":     true,
	"upcoming":  true,
	"ongoing":   true,
	"completed": true,
	"cancelled": true,
}

func parseFlexibleDateTime(dateStr string) (time.Time, error) {
	trimmed := strings.TrimSpace(dateStr)
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, trimmed); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("format tanggal '%s' tidak valid. Gunakan ISO 8601 / RFC3339 (contoh: 2026-12-01T19:00:00Z)", dateStr)
}

// processAndValidatePerformers validates and prepares performer records for insertion
func processAndValidatePerformers(rawPerformers []models.PerformerItemRequest) ([]models.EventPerformer, error) {
	var performers []models.EventPerformer
	seenSingers := make(map[int]bool)
	seenBands := make(map[int]bool)

	for _, item := range rawPerformers {
		var sID *int
		var bID *int

		// Support direct IDs or Type + ID
		if item.SingerID != nil && *item.SingerID > 0 {
			sID = item.SingerID
		}
		if item.BandID != nil && *item.BandID > 0 {
			bID = item.BandID
		}

		if strings.EqualFold(item.Type, "singer") && item.ID > 0 {
			if sID == nil {
				id := item.ID
				sID = &id
			}
		} else if strings.EqualFold(item.Type, "band") && item.ID > 0 {
			if bID == nil {
				id := item.ID
				bID = &id
			}
		}

		// Validation: check constraint ((singer_id IS NOT NULL AND band_id IS NULL) OR ...)
		if (sID == nil && bID == nil) || (sID != nil && bID != nil) {
			return nil, fmt.Errorf("setiap performer harus memilih salah satu: penyanyi solo ATAU band (tidak boleh keduanya atau kosong)")
		}

		if sID != nil {
			if seenSingers[*sID] {
				return nil, fmt.Errorf("duplikasi performer: penyanyi solo dengan ID %d dimasukkan lebih dari satu kali", *sID)
			}
			seenSingers[*sID] = true

			// Verify singer exists in database
			var singerExists bool
			if err := config.DB.Model(&models.Singer{}).
				Select("count(*) > 0").
				Where("id = ?", *sID).
				Find(&singerExists).Error; err != nil || !singerExists {
				return nil, fmt.Errorf("penyanyi solo dengan ID %d tidak ditemukan", *sID)
			}

			performers = append(performers, models.EventPerformer{
				SingerID: sID,
			})
		}

		if bID != nil {
			if seenBands[*bID] {
				return nil, fmt.Errorf("duplikasi performer: band dengan ID %d dimasukkan lebih dari satu kali", *bID)
			}
			seenBands[*bID] = true

			// Verify band exists in database
			var bandExists bool
			if err := config.DB.Model(&models.Band{}).
				Select("count(*) > 0").
				Where("id = ?", *bID).
				Find(&bandExists).Error; err != nil || !bandExists {
				return nil, fmt.Errorf("band dengan ID %d tidak ditemukan", *bID)
			}

			performers = append(performers, models.EventPerformer{
				BandID: bID,
			})
		}
	}

	return performers, nil
}

// CreateEvent - POST /api/events (Admin only)
func CreateEvent(c *fiber.Ctx) error {
	req := new(models.EventRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Format request tidak valid",
			Error:   err.Error(),
		})
	}

	if strings.TrimSpace(req.NamaKonser) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Nama konser wajib diisi",
		})
	}

	if strings.TrimSpace(req.Lokasi) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Lokasi konser wajib diisi",
		})
	}

	if strings.TrimSpace(req.TanggalMulai) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Tanggal mulai wajib diisi",
		})
	}

	tMulai, err := parseFlexibleDateTime(req.TanggalMulai)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: err.Error(),
		})
	}

	var tSelesai *time.Time
	if req.TanggalSelesai != nil && strings.TrimSpace(*req.TanggalSelesai) != "" {
		parsed, err := parseFlexibleDateTime(*req.TanggalSelesai)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
				Message: err.Error(),
			})
		}
		if parsed.Before(tMulai) {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
				Message: "Tanggal selesai tidak boleh lebih awal dari tanggal mulai",
			})
		}
		tSelesai = &parsed
	}

	status := "draft"
	if req.Status != nil && strings.TrimSpace(*req.Status) != "" {
		normalizedStatus := strings.ToLower(strings.TrimSpace(*req.Status))
		if !validEventStatuses[normalizedStatus] {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
				Message: "Status konser tidak valid. Pilihan: draft, upcoming, ongoing, completed, cancelled",
			})
		}
		status = normalizedStatus
	}

	penjualanDibuka := false
	if req.PenjualanDibuka != nil {
		penjualanDibuka = *req.PenjualanDibuka
	}

	// Validate performers
	performersToInsert, err := processAndValidatePerformers(req.Performers)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: err.Error(),
		})
	}

	event := models.Event{
		NamaKonser:      req.NamaKonser,
		Deskripsi:       req.Deskripsi,
		Lokasi:          req.Lokasi,
		TanggalMulai:    tMulai,
		TanggalSelesai:  tSelesai,
		Foto:            req.Foto,
		Status:          status,
		PenjualanDibuka: penjualanDibuka,
	}

	// GORM Transaction to insert event and performers atomically
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&event).Error; err != nil {
			return err
		}

		for i := range performersToInsert {
			performersToInsert[i].EventID = event.ID
			if err := tx.Create(&performersToInsert[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal membuat konser",
			Error:   err.Error(),
		})
	}

	// Reload event with performers populated
	config.DB.Preload("Performers.Singer").
		Preload("Performers.Band").
		First(&event, event.ID)

	return c.Status(fiber.StatusCreated).JSON(models.EventResponse{
		Message: "Konser berhasil dibuat",
		Data:    &event,
	})
}

// GetEvents - GET /api/events
func GetEvents(c *fiber.Ctx) error {
	var events []models.Event
	var total int64

	query := config.DB.Model(&models.Event{})

	// Optional filter by status
	statusParam := strings.TrimSpace(c.Query("status"))
	if statusParam != "" {
		query = query.Where("status = ?", strings.ToLower(statusParam))
	}

	if err := query.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal menghitung data konser",
			Error:   err.Error(),
		})
	}

	if err := query.Preload("Performers.Singer").
		Preload("Performers.Band").
		Order("tanggal_mulai ASC").
		Find(&events).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal mengambil data konser",
			Error:   err.Error(),
		})
	}

	return c.JSON(models.EventListResponse{
		Message: "Data konser berhasil diambil",
		Total:   total,
		Data:    events,
	})
}

// GetEventByID - GET /api/events/:id
func GetEventByID(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "ID konser tidak valid",
		})
	}

	var event models.Event
	if err := config.DB.Preload("Performers.Singer").
		Preload("Performers.Band").
		First(&event, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
				Message: "Konser tidak ditemukan",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal mengambil detail konser",
			Error:   err.Error(),
		})
	}

	return c.JSON(models.EventResponse{
		Message: "Detail konser berhasil diambil",
		Data:    &event,
	})
}

// UpdateEvent - PUT /api/events/:id (Admin only)
func UpdateEvent(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "ID konser tidak valid",
		})
	}

	var existingEvent models.Event
	if err := config.DB.First(&existingEvent, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
				Message: "Konser tidak ditemukan",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal mencari data konser",
			Error:   err.Error(),
		})
	}

	req := new(models.EventRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Format request tidak valid",
			Error:   err.Error(),
		})
	}

	if strings.TrimSpace(req.NamaKonser) != "" {
		existingEvent.NamaKonser = req.NamaKonser
	}
	if req.Deskripsi != nil {
		existingEvent.Deskripsi = req.Deskripsi
	}
	if strings.TrimSpace(req.Lokasi) != "" {
		existingEvent.Lokasi = req.Lokasi
	}
	if req.Foto != nil {
		existingEvent.Foto = req.Foto
	}

	if strings.TrimSpace(req.TanggalMulai) != "" {
		tMulai, err := parseFlexibleDateTime(req.TanggalMulai)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
				Message: err.Error(),
			})
		}
		existingEvent.TanggalMulai = tMulai
	}

	if req.TanggalSelesai != nil {
		if strings.TrimSpace(*req.TanggalSelesai) == "" {
			existingEvent.TanggalSelesai = nil
		} else {
			parsed, err := parseFlexibleDateTime(*req.TanggalSelesai)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
					Message: err.Error(),
				})
			}
			if parsed.Before(existingEvent.TanggalMulai) {
				return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
					Message: "Tanggal selesai tidak boleh lebih awal dari tanggal mulai",
				})
			}
			existingEvent.TanggalSelesai = &parsed
		}
	} else if existingEvent.TanggalSelesai != nil && existingEvent.TanggalSelesai.Before(existingEvent.TanggalMulai) {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Tanggal selesai tidak boleh lebih awal dari tanggal mulai",
		})
	}

	if req.Status != nil && strings.TrimSpace(*req.Status) != "" {
		normalizedStatus := strings.ToLower(strings.TrimSpace(*req.Status))
		if !validEventStatuses[normalizedStatus] {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
				Message: "Status konser tidak valid. Pilihan: draft, upcoming, ongoing, completed, cancelled",
			})
		}
		existingEvent.Status = normalizedStatus
	}

	if req.PenjualanDibuka != nil {
		existingEvent.PenjualanDibuka = *req.PenjualanDibuka
	}

	existingEvent.UpdatedAt = time.Now()

	// If performers are supplied in the update request, validate them
	var performersToInsert []models.EventPerformer
	updatePerformers := req.Performers != nil
	if updatePerformers {
		var err error
		performersToInsert, err = processAndValidatePerformers(req.Performers)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
				Message: err.Error(),
			})
		}
	}

	// GORM Transaction
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&existingEvent).Error; err != nil {
			return err
		}

		if updatePerformers {
			// Clear existing performers
			if err := tx.Where("event_id = ?", existingEvent.ID).Delete(&models.EventPerformer{}).Error; err != nil {
				return err
			}

			// Insert updated performers
			for i := range performersToInsert {
				performersToInsert[i].EventID = existingEvent.ID
				if err := tx.Create(&performersToInsert[i]).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal memperbarui konser",
			Error:   err.Error(),
		})
	}

	// Reload with associations
	config.DB.Preload("Performers.Singer").
		Preload("Performers.Band").
		First(&existingEvent, existingEvent.ID)

	return c.JSON(models.EventResponse{
		Message: "Konser berhasil diperbarui",
		Data:    &existingEvent,
	})
}

// DeleteEvent - DELETE /api/events/:id (Admin only)
func DeleteEvent(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "ID konser tidak valid",
		})
	}

	var event models.Event
	if err := config.DB.First(&event, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
				Message: "Konser tidak ditemukan",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal mencari data konser",
			Error:   err.Error(),
		})
	}

	// In database, event_performers has ON DELETE CASCADE on event_id.
	// Deleting the event will automatically cascade deletion of event_performers.
	if err := config.DB.Delete(&event).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal menghapus konser",
			Error:   err.Error(),
		})
	}

	return c.JSON(models.SuccessResponse{
		Message: "Konser berhasil dihapus",
	})
}
