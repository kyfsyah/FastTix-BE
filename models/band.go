package models

import "time"

type Band struct {
	ID           int       `gorm:"primaryKey;autoIncrement" json:"id"`
	NamaBand     string    `gorm:"column:nama_band;not null" json:"nama_band"`
	Genre        *string   `gorm:"column:genre" json:"genre"`
	TahunBerdiri *int      `gorm:"column:tahun_berdiri" json:"tahun_berdiri"`
	Asal         *string   `gorm:"column:asal" json:"asal"`
	Deskripsi    *string   `gorm:"column:deskripsi" json:"deskripsi"`
	Foto         *string   `gorm:"column:foto" json:"foto"`
	CreatedAt    time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (Band) TableName() string {
	return "band"
}

type BandRequest struct {
	NamaBand     string  `json:"nama_band"`
	Genre        *string `json:"genre"`
	TahunBerdiri *int    `json:"tahun_berdiri"`
	Asal         *string `json:"asal"`
	Deskripsi    *string `json:"deskripsi"`
	Foto         *string `json:"foto"`
}

type BandResponse struct {
	Message string `json:"message"`
	Data    *Band  `json:"data,omitempty"`
}

type BandListResponse struct {
	Message string `json:"message"`
	Total   int64  `json:"total"`
	Data    []Band `json:"data"`
}
