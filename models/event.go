package models

import "time"

type Event struct {
	ID              int        `gorm:"primaryKey;autoIncrement" json:"id"`
	NamaKonser      string     `gorm:"column:nama_konser;not null" json:"nama_konser"`
	Deskripsi       *string    `gorm:"column:deskripsi" json:"deskripsi"`
	Lokasi          string     `gorm:"column:lokasi;not null" json:"lokasi"`
	TanggalMulai    time.Time  `gorm:"column:tanggal_mulai;type:timestamptz;not null" json:"tanggal_mulai"`
	TanggalSelesai  *time.Time `gorm:"column:tanggal_selesai;type:timestamptz" json:"tanggal_selesai"`
	Foto            *string    `gorm:"column:foto" json:"foto"`
	Status          string     `gorm:"column:status;not null;default:'draft'" json:"status"`
	PenjualanDibuka bool       `gorm:"column:penjualan_dibuka;not null;default:false" json:"penjualan_dibuka"`
	CreatedAt       time.Time  `gorm:"column:created_at;type:timestamptz;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;type:timestamptz;default:CURRENT_TIMESTAMP" json:"updated_at"`

	Performers []EventPerformer `gorm:"foreignKey:EventID;references:ID" json:"performers,omitempty"`
}

func (Event) TableName() string {
	return "events"
}

type EventRequest struct {
	NamaKonser      string                 `json:"nama_konser"`
	Deskripsi       *string                `json:"deskripsi"`
	Lokasi          string                 `json:"lokasi"`
	TanggalMulai    string                 `json:"tanggal_mulai"`
	TanggalSelesai  *string                `json:"tanggal_selesai"`
	Foto            *string                `json:"foto"`
	Status          *string                `json:"status"`
	PenjualanDibuka *bool                  `json:"penjualan_dibuka"`
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
