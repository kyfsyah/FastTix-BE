package models

import "time"

type Event struct {
	ID              int        `gorm:"primaryKey;autoIncrement" json:"id"`
	NamaKonser      string     `gorm:"column:nama_konser;not null" json:"nama_konser"`
	Deskripsi       *string    `gorm:"column:deskripsi" json:"deskripsi,omitempty"`
	TanggalMulai    time.Time  `gorm:"column:tanggal_mulai;type:timestamptz;not null" json:"tanggal_mulai"`
	TanggalSelesai  *time.Time `gorm:"column:tanggal_selesai;type:timestamptz" json:"tanggal_selesai,omitempty"`
	Foto            *string    `gorm:"column:foto" json:"foto,omitempty"`
	Status          string     `gorm:"column:status;not null;default:'draft'" json:"status"`
	PenjualanDibuka bool       `gorm:"column:penjualan_dibuka;not null;default:false" json:"penjualan_dibuka"`
	OrganizerID     *int       `gorm:"column:organizer_id" json:"organizer_id,omitempty"`
	VenueID         *int64     `gorm:"column:venue_id" json:"venue_id,omitempty"`
	CategoryID      int        `gorm:"column:category_id;not null" json:"category_id"`
	CreatedAt       time.Time  `gorm:"column:created_at;type:timestamptz;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;type:timestamptz;default:CURRENT_TIMESTAMP" json:"updated_at"`

	Venue      *Venue           `gorm:"foreignKey:VenueID;references:ID" json:"venue,omitempty"`
	Organizer  *Organizer       `gorm:"foreignKey:OrganizerID;references:ID" json:"organizer,omitempty"`
	Category   *EventCategory   `gorm:"foreignKey:CategoryID;references:ID" json:"category,omitempty"`
	Performers []EventPerformer `gorm:"foreignKey:EventID;references:ID" json:"performers,omitempty"`
}

func (Event) TableName() string {
	return "events"
}

type EventRequest struct {
	NamaKonser      string                 `json:"nama_konser"`
	Deskripsi       *string                `json:"deskripsi"`
	TanggalMulai    string                 `json:"tanggal_mulai"`
	TanggalSelesai  *string                `json:"tanggal_selesai"`
	Foto            *string                `json:"foto"`
	Status          *string                `json:"status"`
	PenjualanDibuka *bool                  `json:"penjualan_dibuka"`
	OrganizerID     *int                   `json:"organizer_id"`
	VenueID         *int64                 `json:"venue_id"`
	CategoryID      *int                   `json:"category_id"`
	Performers      []PerformerItemRequest `json:"performers"`
}

type EventResponse struct {
	Message string `json:"message"`
	Data    *Event `json:"data,omitempty"`
}

type EventListResponse struct {
	Message string  `json:"message"`
	Total   int64   `json:"total"`
	Data    []Event `json:"data"`
}
