package controllers

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"fasttix-backend/config"
	"fasttix-backend/middleware"
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
	if r, ok := c.Locals("userRole").(string); ok && r != "" {
		role = r
	}
	if id, ok := c.Locals("userID").(int); ok && id > 0 {
		userID = id
	}

	if userID == 0 || role == "" {
		if claims, ok := c.Locals("user").(jwt.MapClaims); ok {
			if role == "" {
				if r, ok := claims["role"].(string); ok {
					role = strings.ToLower(strings.TrimSpace(r))
				}
			}
			if userID == 0 {
				if id, ok := middleware.ExtractUserIDFromClaims(claims); ok {
					userID = id
				}
			}
		}
	}
	return userID, role
}

// tryGetAuthUser mencoba membaca identitas pengguna dari Locals atau Authorization Bearer token secara opsional (untuk endpoint publik).
func tryGetAuthUser(c *fiber.Ctx) (userID int, role string) {
	userID, role = getAuthUser(c)
	if userID > 0 && role != "" {
		return userID, role
	}

	authHeader := c.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" && strings.TrimSpace(parts[1]) != "" {
			secretKey := strings.TrimSpace(os.Getenv("JWT_SECRET"))
			if secretKey != "" {
				token, err := jwt.Parse(strings.TrimSpace(parts[1]), func(token *jwt.Token) (interface{}, error) {
					if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
						return nil, fmt.Errorf("unexpected signing method")
					}
					return []byte(secretKey), nil
				})
				if err == nil && token.Valid {
					if claims, ok := token.Claims.(jwt.MapClaims); ok {
						if tokenType, ok := claims["token_type"].(string); ok && tokenType == "access" {
							if id, ok := middleware.ExtractUserIDFromClaims(claims); ok {
								userID = id
							}
							if r, ok := claims["role"].(string); ok {
								role = strings.ToLower(strings.TrimSpace(r))
							}
						}
					}
				}
			}
		}
	}
	return userID, role
}

// verifyOrganizerMemberAccess memvalidasi bahwa user merupakan member aktif dari organizer yang berstatus aktif,
// serta memiliki salah satu dari allowedRoles di dalam organizer tersebut.
func verifyOrganizerMemberAccess(c *fiber.Ctx, userID int, organizerID int, allowedRoles ...string) (*models.OrganizerMember, error) {
	var member models.OrganizerMember
	err := config.DB.WithContext(c.UserContext()).
		Joins("JOIN organizers ON organizers.id = organizer_members.organizer_id").
		Where("organizer_members.user_id = ? AND organizer_members.organizer_id = ?", userID, organizerID).
		Preload("Organizer").
		First(&member).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("Anda bukan anggota dari organizer ini")
		}
		return nil, errors.New("Gagal memverifikasi keanggotaan organizer")
	}

	if strings.ToLower(strings.TrimSpace(member.Status)) != "active" {
		return nil, fmt.Errorf("Status keanggotaan Anda di organizer ini tidak aktif (%s)", member.Status)
	}

	if member.Organizer != nil && strings.ToLower(strings.TrimSpace(member.Organizer.Status)) != "active" {
		return nil, fmt.Errorf("Organizer '%s' tidak berstatus aktif (status: %s)", member.Organizer.NamaOrganizer, member.Organizer.Status)
	}

	if len(allowedRoles) > 0 {
		roleAllowed := false
		memberRoleLower := strings.ToLower(strings.TrimSpace(member.Role))
		for _, allowed := range allowedRoles {
			if memberRoleLower == strings.ToLower(strings.TrimSpace(allowed)) {
				roleAllowed = true
				break
			}
		}
		if !roleAllowed {
			return nil, fmt.Errorf("Role keanggotaan Anda (%s) tidak memiliki izin untuk tindakan ini", member.Role)
		}
	}

	return &member, nil
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

// processAndValidatePerformers memvalidasi dan menyiapkan record performer konser
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

		// Validasi: penyanyi solo ATAU band (tidak boleh keduanya atau kosong)
		if (sID == nil && bID == nil) || (sID != nil && bID != nil) {
			return nil, fmt.Errorf("setiap performer harus memilih salah satu: penyanyi solo ATAU band (tidak boleh keduanya atau kosong)")
		}

		if sID != nil {
			if seenSingers[*sID] {
				return nil, fmt.Errorf("duplikasi performer: penyanyi solo dengan ID %d dimasukkan lebih dari satu kali", *sID)
			}
			seenSingers[*sID] = true

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

		var venue models.Venue
		if err := config.DB.WithContext(c.UserContext()).First(&venue, *req.VenueID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
					Message: fmt.Sprintf("Venue dengan ID %d tidak ditemukan di database", *req.VenueID),
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
				Message: "Gagal memverifikasi data venue",
			})
		}
	}

	// Identifikasi organizer dari autentikasi pengguna dan keanggotaan database
	currentUserID, currentUserRole := getAuthUser(c)
	var organizerID *int

	if middleware.IsGlobalAdmin(currentUserRole) {
		// Global Admin (admin / superadmin): boleh menentukan organizer_id yang valid
		if req.OrganizerID != nil && *req.OrganizerID > 0 {
			var org models.Organizer
			if err := config.DB.WithContext(c.UserContext()).First(&org, *req.OrganizerID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
						Message: fmt.Sprintf("Organizer dengan ID %d tidak ditemukan", *req.OrganizerID),
					})
				}
				return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
					Message: "Gagal memverifikasi data organizer",
				})
			}
			if strings.ToLower(strings.TrimSpace(org.Status)) != "active" {
				return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
					Message: fmt.Sprintf("Organizer '%s' tidak berstatus aktif (status: %s)", org.NamaOrganizer, org.Status),
				})
			}
			organizerID = req.OrganizerID
		}
	} else if currentUserRole == "organizer" {
		// Pengguna dengan global role 'organizer': wajib diverifikasi dari database
		if req.OrganizerID != nil && *req.OrganizerID > 0 {
			_, err := verifyOrganizerMemberAccess(c, currentUserID, *req.OrganizerID, "owner", "admin", "event_manager")
			if err != nil {
				return c.Status(fiber.StatusForbidden).JSON(models.ErrorResponse{
					Message: fmt.Sprintf("Forbidden: %s", err.Error()),
				})
			}
			organizerID = req.OrganizerID
		} else {
			var activeMembers []models.OrganizerMember
			if err := config.DB.WithContext(c.UserContext()).
				Joins("JOIN organizers ON organizers.id = organizer_members.organizer_id").
				Where("organizer_members.user_id = ? AND organizer_members.status = 'active' AND organizers.status = 'active'", currentUserID).
				Find(&activeMembers).Error; err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
					Message: "Gagal memeriksa data keanggotaan organizer",
				})
			}

			if len(activeMembers) == 0 {
				return c.Status(fiber.StatusForbidden).JSON(models.ErrorResponse{
					Message: "Forbidden: Anda tidak memiliki keanggotaan aktif pada organizer yang berstatus aktif",
				})
			}

			if len(activeMembers) > 1 {
				return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
					Message: "Anda terdaftar di lebih dari satu organizer aktif. Harap cantumkan 'organizer_id' pada request body.",
				})
			}

			targetMember := activeMembers[0]
			mRoleLower := strings.ToLower(strings.TrimSpace(targetMember.Role))
			if mRoleLower != "owner" && mRoleLower != "admin" && mRoleLower != "event_manager" {
				return c.Status(fiber.StatusForbidden).JSON(models.ErrorResponse{
					Message: fmt.Sprintf("Forbidden: Role Anda dalam organizer (%s) tidak memiliki izin untuk membuat event", targetMember.Role),
				})
			}
			targetID := targetMember.OrganizerID
			organizerID = &targetID
		}
	} else {
		return c.Status(fiber.StatusForbidden).JSON(models.ErrorResponse{
			Message: "Forbidden: Anda tidak memiliki izin untuk membuat event",
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
		})
	}

	// Reload event with Venue, Organizer, and Performers populated
	config.DB.WithContext(c.UserContext()).
		Preload("Venue").
		Preload("Organizer").
		Preload("Performers.Singer").
		Preload("Performers.Band").
		First(&event, event.ID)

	return c.Status(fiber.StatusCreated).JSON(models.EventResponse{
		Message: "Konser berhasil dibuat",
		Data:    &event,
	})
}

// GetEvents - GET /api/events (Publik dengan filter visibilitas)
func GetEvents(c *fiber.Ctx) error {
	var events []models.Event
	var total int64

	query := config.DB.WithContext(c.UserContext()).Model(&models.Event{})

	userID, userRole := tryGetAuthUser(c)
	statusParam := strings.ToLower(strings.TrimSpace(c.Query("status")))

	if middleware.IsGlobalAdmin(userRole) {
		// Global Admin dapat melihat semua event atau filter status yang diminta
		if statusParam != "" {
			query = query.Where("status = ?", statusParam)
		}
	} else if userRole == "organizer" && userID > 0 {
		// Organizer dapat melihat event publik + event milik organizer aktif tempat user terdaftar
		if statusParam != "" {
			if statusParam == "draft" {
				query = query.Where("status = 'draft' AND organizer_id IN (SELECT organizer_id FROM organizer_members WHERE user_id = ? AND status = 'active')", userID)
			} else {
				query = query.Where("status = ?", statusParam)
			}
		} else {
			query = query.Where("status IN ('upcoming', 'ongoing', 'completed') OR (organizer_id IN (SELECT organizer_id FROM organizer_members WHERE user_id = ? AND status = 'active'))", userID)
		}
	} else {
		// Publik & Customer: draft tidak boleh tampil ke publik
		if statusParam != "" {
			if statusParam == "draft" {
				return c.JSON(models.EventListResponse{
					Message: "Data konser berhasil diambil",
					Total:   0,
					Data:    []models.Event{},
				})
			}
			query = query.Where("status = ?", statusParam)
		} else {
			query = query.Where("status IN ('upcoming', 'ongoing', 'completed')")
		}
	}

	if err := query.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal menghitung data konser",
		})
	}

	if err := query.Preload("Venue").
		Preload("Organizer").
		Preload("Performers.Singer").
		Preload("Performers.Band").
		Order("tanggal_mulai ASC").
		Find(&events).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal mengambil data konser",
		})
	}

	return c.JSON(models.EventListResponse{
		Message: "Data konser berhasil diambil",
		Total:   total,
		Data:    events,
	})
}

// GetEventByID - GET /api/events/:id (Publik dengan perlindungan status draft)
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
		Preload("Organizer").
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
		})
	}

	// Jika status event adalah 'draft', hanya admin atau anggota berwenang organizer pemilik yang boleh melihatnya
	if event.Status == "draft" {
		userID, userRole := tryGetAuthUser(c)
		allowed := false
		if middleware.IsGlobalAdmin(userRole) {
			allowed = true
		} else if userRole == "organizer" && userID > 0 && event.OrganizerID != nil {
			var count int64
			config.DB.WithContext(c.UserContext()).Table("organizer_members").
				Where("organizer_id = ? AND user_id = ? AND status = 'active' AND role IN ('owner', 'admin', 'event_manager')", *event.OrganizerID, userID).
				Count(&count)
			if count > 0 {
				allowed = true
			}
		}

		if !allowed {
			return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse{
				Message: "Konser tidak ditemukan",
			})
		}
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
		})
	}

	req := new(models.EventRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
			Message: "Format request tidak valid",
		})
	}

	currentUserID, currentUserRole := getAuthUser(c)

	if !middleware.IsGlobalAdmin(currentUserRole) {
		if currentUserRole != "organizer" {
			return c.Status(fiber.StatusForbidden).JSON(models.ErrorResponse{
				Message: "Forbidden: Anda tidak memiliki izin untuk memperbarui event",
			})
		}

		if existingEvent.OrganizerID == nil {
			return c.Status(fiber.StatusForbidden).JSON(models.ErrorResponse{
				Message: "Forbidden: Event ini tidak dikelola oleh organizer Anda",
			})
		}

		// Verifikasi keanggotaan aktif di organizer dan role (owner, admin, event_manager)
		_, err := verifyOrganizerMemberAccess(c, currentUserID, *existingEvent.OrganizerID, "owner", "admin", "event_manager")
		if err != nil {
			return c.Status(fiber.StatusForbidden).JSON(models.ErrorResponse{
				Message: fmt.Sprintf("Forbidden: %s", err.Error()),
			})
		}

		// Cegah manipulasi pergantian organizer_id oleh organizer
		if req.OrganizerID != nil && *req.OrganizerID != *existingEvent.OrganizerID {
			return c.Status(fiber.StatusForbidden).JSON(models.ErrorResponse{
				Message: "Forbidden: Anda tidak diizinkan memindahkan event ke organizer lain",
			})
		}
	} else {
		// Global Admin: jika ingin memindahkan organizer_id, validasi ke tabel organizers
		if req.OrganizerID != nil && *req.OrganizerID > 0 && (existingEvent.OrganizerID == nil || *req.OrganizerID != *existingEvent.OrganizerID) {
			var org models.Organizer
			if err := config.DB.WithContext(c.UserContext()).First(&org, *req.OrganizerID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
						Message: fmt.Sprintf("Organizer dengan ID %d tidak ditemukan", *req.OrganizerID),
					})
				}
				return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
					Message: "Gagal memverifikasi data organizer baru",
				})
			}
			if strings.ToLower(strings.TrimSpace(org.Status)) != "active" {
				return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse{
					Message: fmt.Sprintf("Organizer '%s' tidak berstatus aktif (status: %s)", org.NamaOrganizer, org.Status),
				})
			}
			existingEvent.OrganizerID = req.OrganizerID
		}
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
			})
		}

		existingEvent.VenueID = req.VenueID
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

	err = config.DB.WithContext(c.UserContext()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&existingEvent).Error; err != nil {
			return err
		}

		if updatePerformers {
			if err := tx.Where("event_id = ?", existingEvent.ID).Delete(&models.EventPerformer{}).Error; err != nil {
				return err
			}

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
		})
	}

	// Reload with associations (Venue, Organizer, Singer, Band)
	config.DB.WithContext(c.UserContext()).
		Preload("Venue").
		Preload("Organizer").
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
		})
	}

	currentUserID, currentUserRole := getAuthUser(c)
	if !middleware.IsGlobalAdmin(currentUserRole) {
		if currentUserRole != "organizer" {
			return c.Status(fiber.StatusForbidden).JSON(models.ErrorResponse{
				Message: "Forbidden: Anda tidak memiliki izin untuk menghapus event",
			})
		}

		if event.OrganizerID == nil {
			return c.Status(fiber.StatusForbidden).JSON(models.ErrorResponse{
				Message: "Forbidden: Event ini tidak dikelola oleh organizer Anda",
			})
		}

		// Hanya role owner atau admin di organizer yang berhak menghapus event
		_, err := verifyOrganizerMemberAccess(c, currentUserID, *event.OrganizerID, "owner", "admin")
		if err != nil {
			return c.Status(fiber.StatusForbidden).JSON(models.ErrorResponse{
				Message: fmt.Sprintf("Forbidden: %s", err.Error()),
			})
		}
	}

	// Jangan izinkan penghapusan jika tiket penjualan konser sudah dibuka atau event sedang berjalan
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
			strings.Contains(errLower, "foreign key") ||
			strings.Contains(errLower, "23503") {
			return c.Status(fiber.StatusConflict).JSON(models.ErrorResponse{
				Message: "Konser tidak dapat dihapus karena masih memiliki relasi data penting (tiket, transaksi, atau pembayaran)",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse{
			Message: "Gagal menghapus konser",
		})
	}

	return c.JSON(models.SuccessResponse{
		Message: "Konser berhasil dihapus",
	})
}
