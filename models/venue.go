package models

import "time"

type Venue struct {
	ID             int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Nama           string    `gorm:"column:nama;not null" json:"nama"`
	Alamat         string    `gorm:"column:alamat;not null" json:"alamat"`
	Kota           string    `gorm:"column:kota;not null" json:"kota"`
	Provinsi       *string   `gorm:"column:provinsi" json:"provinsi,omitempty"`
	Negara         *string   `gorm:"column:negara" json:"negara,omitempty"`
	Deskripsi      *string   `gorm:"column:deskripsi" json:"deskripsi,omitempty"`
	Foto           *string   `gorm:"column:foto" json:"foto,omitempty"`
	KapasitasTotal int       `gorm:"column:kapasitas_total;not null" json:"kapasitas_total"`
	CreatedAt      time.Time `gorm:"column:created_at;type:timestamptz;default:CURRENT_TIMESTAMP" json:"created_at,omitempty"`
	UpdatedAt      time.Time `gorm:"column:updated_at;type:timestamptz;default:CURRENT_TIMESTAMP" json:"updated_at,omitempty"`

	Areas []VenueArea `gorm:"foreignKey:VenueID;references:ID" json:"areas,omitempty"`
}

func (Venue) TableName() string {
	return "venues"
}

type VenueArea struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	VenueID   int64     `gorm:"column:venue_id;not null" json:"venue_id"`
	NamaArea  string    `gorm:"column:nama_area;not null" json:"nama_area"`
	Tipe      string    `gorm:"column:tipe;not null" json:"tipe"` // standing / seating
	Kapasitas int       `gorm:"column:kapasitas;not null" json:"kapasitas"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;default:CURRENT_TIMESTAMP" json:"created_at,omitempty"`

	Seats []VenueSeat `gorm:"foreignKey:VenueAreaID;references:ID" json:"seats,omitempty"`
}

func (VenueArea) TableName() string {
	return "venue_areas"
}

type VenueSeat struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	VenueAreaID int64     `gorm:"column:venue_area_id;not null" json:"venue_area_id"`
	NomorKursi  string    `gorm:"column:nomor_kursi;not null" json:"nomor_kursi"`
	Baris       *string   `gorm:"column:baris" json:"baris,omitempty"`
	CreatedAt   time.Time `gorm:"column:created_at;type:timestamptz;default:CURRENT_TIMESTAMP" json:"created_at,omitempty"`
}

func (VenueSeat) TableName() string {
	return "venue_seats"
}
