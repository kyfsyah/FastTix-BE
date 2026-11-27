package models

import "time"

type EventCategory struct {
	ID           int       `gorm:"primaryKey" json:"id"`
	NamaKategori string    `gorm:"column:nama_kategori;not null;unique" json:"nama_kategori"`
	Slug         string    `gorm:"not null;unique" json:"slug"`
	Deskripsi    string    `gorm:"type:text" json:"deskripsi"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (EventCategory) TableName() string {
	return "event_categories"
}