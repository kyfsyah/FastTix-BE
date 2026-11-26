package models

import "time"

type Organizer struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	NamaOrganizer string    `gorm:"column:nama_organizer;not null" json:"nama_organizer"`
	Slug          string    `gorm:"column:slug;not null" json:"slug"`
	Deskripsi     *string   `gorm:"column:deskripsi" json:"deskripsi,omitempty"`
	Logo          *string   `gorm:"column:logo" json:"logo,omitempty"`
	Email         string    `gorm:"column:email;not null" json:"email"`
	NoHP          *string   `gorm:"column:no_hp" json:"no_hp,omitempty"`
	Alamat        *string   `gorm:"column:alamat" json:"alamat,omitempty"`
	Status        string    `gorm:"column:status;not null;default:'pending'" json:"status"` // pending, active, suspended
	CreatedBy     *int      `gorm:"column:created_by" json:"created_by,omitempty"`
	CreatedAt     time.Time `gorm:"column:created_at;type:timestamptz;default:CURRENT_TIMESTAMP" json:"created_at,omitempty"`
	UpdatedAt     time.Time `gorm:"column:updated_at;type:timestamptz;default:CURRENT_TIMESTAMP" json:"updated_at,omitempty"`

	Members []OrganizerMember `gorm:"foreignKey:OrganizerID;references:ID" json:"members,omitempty"`
	Events  []Event           `gorm:"foreignKey:OrganizerID;references:ID" json:"events,omitempty"`
}

func (Organizer) TableName() string {
	return "organizers"
}

type OrganizerMember struct {
	ID          int        `gorm:"primaryKey;autoIncrement" json:"id"`
	OrganizerID int        `gorm:"column:organizer_id;not null" json:"organizer_id"`
	UserID      int        `gorm:"column:user_id;not null" json:"user_id"`
	Role        string     `gorm:"column:role;not null" json:"role"`     // owner, admin, event_manager, ticketing, finance
	Status      string     `gorm:"column:status;not null" json:"status"` // active, inactive
	JoinedAt    *time.Time `gorm:"column:joined_at;type:timestamptz" json:"joined_at,omitempty"`
	CreatedAt   time.Time  `gorm:"column:created_at;type:timestamptz;default:CURRENT_TIMESTAMP" json:"created_at,omitempty"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;type:timestamptz;default:CURRENT_TIMESTAMP" json:"updated_at,omitempty"`

	Organizer *Organizer `gorm:"foreignKey:OrganizerID;references:ID" json:"organizer,omitempty"`
	User      *User      `gorm:"foreignKey:UserID;references:ID" json:"user,omitempty"`
}

func (OrganizerMember) TableName() string {
	return "organizer_members"
}
