package models

import "time"

type Singer struct {
	ID           int       `gorm:"primaryKey;autoIncrement" json:"id"`
	NamaPenyanyi string    `gorm:"column:nama_penyanyi;not null" json:"nama_penyanyi"`
	NamaPanggung *string   `gorm:"column:nama_panggung" json:"nama_panggung"`
	Genre        *string   `gorm:"column:genre" json:"genre"`
	TanggalLahir *string   `gorm:"column:tanggal_lahir;type:date" json:"tanggal_lahir"`
	Asal         *string   `gorm:"column:asal" json:"asal"`
	Deskripsi    *string   `gorm:"column:deskripsi" json:"deskripsi"`
	Foto         *string   `gorm:"column:foto" json:"foto"`
	CreatedAt    time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (Singer) TableName() string {
	return "singer"
}

type SingerRequest struct {
	NamaPenyanyi string  `json:"nama_penyanyi"`
	NamaPanggung *string `json:"nama_panggung"`
	Genre        *string `json:"genre"`
	TanggalLahir *string `json:"tanggal_lahir"`
	Asal         *string `json:"asal"`
	Deskripsi    *string `json:"deskripsi"`
	Foto         *string `json:"foto"`
}

type SingerResponse struct {
	Message string  `json:"message"`
	Data    *Singer `json:"data,omitempty"`
}

type SingerListResponse struct {
	Message string   `json:"message"`
	Total   int64    `json:"total"`
	Data    []Singer `json:"data"`
}
