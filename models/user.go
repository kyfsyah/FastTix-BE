package models

import "time"

type Role struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	NamaRole  string    `gorm:"column:nama_role;unique;not null" json:"nama_role"`
	CreatedAt time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (Role) TableName() string {
	return "roles"
}

type User struct {
	ID        int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Nama      string    `gorm:"column:nama;not null" json:"nama"`
	Email     string    `gorm:"column:email;unique;not null" json:"email"`
	Password  string    `gorm:"column:password;not null" json:"-"` // tidak dikembalikan ke JSON
	NoHP      string    `gorm:"column:no_hp" json:"no_hp"`
	RoleID    int       `gorm:"column:role_id;not null" json:"role_id"`
	Role      string    `gorm:"->" json:"role,omitempty"` // populated dari JOIN
	RoleData  *Role     `gorm:"foreignKey:RoleID" json:"role_data,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (User) TableName() string {
	return "users"
}

// Request & Response structs

type RegisterRequest struct {
	Nama     string `json:"nama"`
	Email    string `json:"email"`
	Password string `json:"password"`
	NoHP     string `json:"no_hp"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Message      string `json:"message"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	User         User   `json:"user"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshResponse struct {
	Message      string `json:"message"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type ProfileResponse struct {
	Message string `json:"message"`
	User    User   `json:"user"`
}

type ErrorResponse struct {
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

type SuccessResponse struct {
	Message string `json:"message"`
}
