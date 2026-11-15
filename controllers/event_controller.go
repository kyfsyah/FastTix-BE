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
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

var validEventStatuses = map[string]bool{
	"draft":     true,
	"upcoming":  true,
	"ongoing":   true,
	"completed": true,
	"cancelled": true,
}

func getAuthUser(c *fiber.Ctx) (userID int, role string) {
	if claims, ok := c.Locals("user").(jwt.MapClaims); ok {
		if r, ok := claims["role"].(string); ok {
			role = strings.ToLower(strings.TrimSpace(r))
		}
		if id, ok := extractUserIDFromClaims(claims); ok {
			userID = id
		}
	}
	return userID, role
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
func processAndValidatePerformers(c *fiber.Ctx, rawPerformers []models.PerformerItemRequest) ([]models.EventPerformer, error) {
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

		if item.ID > 0 {
			switch strings.ToLower(item.Type) {
			case "singer":
				if sID == nil {
					id := item.ID
					sID = &id
				}
			case "band":
				if bID == nil {
					id := item.ID
					bID = &id
				}
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
			if err := config.DB.WithContext(c.UserContext()).Model(&models.Singer{}).
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
			if err := config.DB.WithContext(c.UserContext()).Model(&models.Band{}).
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

// CreateEvent - POST /api/events (Admin or Organizer)
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

	// Validasi venue_id jika dikirim
	if req.VenueID != nil {
		if *req.VenueID <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
				Message: "venue_id tidak valid",
			})
		}

		// Periksa apakah venue benar-benar tersedia di database
		var venue models.Venue
		if err := config.DB.WithContext(c.UserContext()).First(&venue, *req.VenueID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
					Message: fmt.Sprintf("Venue dengan ID %d tidak ditemukan di database", *req.VenueID),
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
				Message: "Gagal memverifikasi data venue",
				Error:   err.Error(),
			})
		}
	}

	// Identifikasi organizer dari autentikasi pengguna
	currentUserID, currentUserRole := getAuthUser(c)
	var organizerID *int

	switch currentUserRole {
	case "organizer":
		// Jika role adalah organizer, ikat organizer_id ke ID user yang sedang login
		if currentUserID > 0 {
			organizerID = &currentUserID
		}
	case "admin", "super_admin":
		// Jika role admin/super_admin, boleh menentukan organizer_id yang valid
		if req.OrganizerID != nil && *req.OrganizerID > 0 {
			var count int64
			if err := config.DB.WithContext(c.UserContext()).Table("users").
				Where("id = ?", *req.OrganizerID).Count(&count).Error; err == nil && count > 0 {
				organizerID = req.OrganizerID
			} else {
				return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
					Message: fmt.Sprintf("Organizer dengan ID %d tidak ditemukan", *req.OrganizerID),
				})
			}
		}
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
	performersToInsert, err := processAndValidatePerformers(c, req.Performers)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: err.Error(),
		})
	}

	event := models.Event{
		NamaKonser:      strings.TrimSpace(req.NamaKonser),
		Deskripsi:       req.Deskripsi,
		VenueID:         req.VenueID,
		OrganizerID:     organizerID,
		TanggalMulai:    tMulai,
		TanggalSelesai:  tSelesai,
		Foto:            req.Foto,
		Status:          status,
		PenjualanDibuka: penjualanDibuka,
	}

	// GORM Transaction to insert event and performers atomically
	err = config.DB.WithContext(c.UserContext()).Transaction(func(tx *gorm.DB) error {
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

	// Reload event with Venue and Performers populated
	config.DB.WithContext(c.UserContext()).
		Preload("Venue").
		Preload("Performers.Singer").
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

	query := config.DB.WithContext(c.UserContext()).Model(&models.Event{})

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

	if err := query.Preload("Venue").
		Preload("Performers.Singer").
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
	if err := config.DB.WithContext(c.UserContext()).
		Preload("Venue").
		Preload("Performers.Singer").
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

// UpdateEvent - PUT /api/events/:id (Admin or Organizer)
func UpdateEvent(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "ID konser tidak valid",
		})
	}

	var existingEvent models.Event
	if err := config.DB.WithContext(c.UserContext()).First(&existingEvent, id).Error; err != nil {
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

	// Pemeriksaan otorisasi: Organizer hanya boleh mengubah event miliknya
	currentUserID, currentUserRole := getAuthUser(c)
	if currentUserRole == "organizer" {
		if existingEvent.OrganizerID == nil || *existingEvent.OrganizerID != currentUserID {
			return c.Status(fiber.StatusForbidden).JSON(models.ErrorResponse{
				Message: "Forbidden: Anda hanya diizinkan mengubah event milik organizer Anda sendiri",
			})
		}
	}

	req := new(models.EventRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Format request tidak valid",
			Error:   err.Error(),
		})
	}

	if strings.TrimSpace(req.NamaKonser) != "" {
		existingEvent.NamaKonser = strings.TrimSpace(req.NamaKonser)
	}
	if req.Deskripsi != nil {
		existingEvent.Deskripsi = req.Deskripsi
	}
	if req.Foto != nil {
		existingEvent.Foto = req.Foto
	}

	// Ganti venue jika venue_id dikirimkan. Jika tidak dikirim, pertahankan venue lama.
	if req.VenueID != nil {
		if *req.VenueID <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
				Message: "venue_id tidak valid",
			})
		}

		var venue models.Venue
		if err := config.DB.WithContext(c.UserContext()).First(&venue, *req.VenueID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
					Message: fmt.Sprintf("Venue dengan ID %d tidak ditemukan di database", *req.VenueID),
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
				Message: "Gagal memverifikasi data venue",
				Error:   err.Error(),
			})
		}

		existingEvent.VenueID = req.VenueID
	}

	// Hanya admin/super_admin yang dapat mengganti organizer_id saat update
	if (currentUserRole == "admin" || currentUserRole == "super_admin") && req.OrganizerID != nil {
		if *req.OrganizerID > 0 {
			var count int64
			if err := config.DB.WithContext(c.UserContext()).Table("users").
				Where("id = ?", *req.OrganizerID).Count(&count).Error; err == nil && count > 0 {
				existingEvent.OrganizerID = req.OrganizerID
			} else {
				return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
					Message: fmt.Sprintf("Organizer dengan ID %d tidak ditemukan", *req.OrganizerID),
				})
			}
		}
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
		performersToInsert, err = processAndValidatePerformers(c, req.Performers)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
				Message: err.Error(),
			})
		}
	}

	// GORM Transaction: Simpan event dan performers secara atomic
	err = config.DB.WithContext(c.UserContext()).Transaction(func(tx *gorm.DB) error {
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

	// Reload with associations (Venue, Singer, Band)
	config.DB.WithContext(c.UserContext()).
		Preload("Venue").
		Preload("Performers.Singer").
		Preload("Performers.Band").
		First(&existingEvent, existingEvent.ID)

	return c.JSON(models.EventResponse{
		Message: "Konser berhasil diperbarui",
		Data:    &existingEvent,
	})
}

// DeleteEvent - DELETE /api/events/:id (Admin or Organizer)
func DeleteEvent(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "ID konser tidak valid",
		})
	}

	var event models.Event
	if err := config.DB.WithContext(c.UserContext()).First(&event, id).Error; err != nil {
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

	// Pemeriksaan otorisasi: Organizer hanya boleh menghapus event miliknya
	currentUserID, currentUserRole := getAuthUser(c)
	if currentUserRole == "organizer" {
		if event.OrganizerID == nil || *event.OrganizerID != currentUserID {
			return c.Status(fiber.StatusForbidden).JSON(models.ErrorResponse{
				Message: "Forbidden: Anda hanya diizinkan menghapus event milik organizer Anda sendiri",
			})
		}
	}

	// Jangan izinkan penghapusan jika tiket konser sudah dibuka untuk penjualan atau konser sudah berjalan
	if event.PenjualanDibuka {
		return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{
			Message: "Konser tidak dapat dihapus karena tiket penjualan konser sudah dibuka",
		})
	}
	if event.Status == "ongoing" || event.Status == "completed" {
		return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{
			Message: fmt.Sprintf("Konser dengan status '%s' tidak dapat dihapus", event.Status),
		})
	}

	if err := config.DB.WithContext(c.UserContext()).Delete(&event).Error; err != nil {
		errLower := strings.ToLower(err.Error())
		if strings.Contains(errLower, "violates foreign key constraint") ||
			strings.Contains(errLower, "foreign key") {
			return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{
				Message: "Konser tidak dapat dihapus karena masih memiliki relasi data penting (tiket, transaksi, atau pembayaran)",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal menghapus konser",
			Error:   err.Error(),
		})
	}

	return c.JSON(models.SuccessResponse{
		Message: "Konser berhasil dihapus",
	})
}
