package models

import (
	"time"
)

type User struct {
	ID             int       `json:"id" db:"id" gorm:"primaryKey;autoIncrement"`
	Username       string    `json:"username" db:"username" gorm:"not null"`
	Email          string    `json:"email" db:"email" gorm:"not null"`
	Password       string    `json:"-" db:"password" gorm:"not null"`
	Role           string    `json:"role" db:"role" gorm:"not null;default:user"`
	Status         string    `json:"status" db:"status" gorm:"not null;default:active"`
	SessionVersion int64     `json:"-" db:"session_version" gorm:"not null;default:1"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type UpdateUserRequest struct {
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
}

type UpdateUserRoleRequest struct {
	Role string `json:"role"`
}

type UpdateUserStatusRequest struct {
	Status string `json:"status"`
}

const (
	RoleAdmin       = "admin"
	RoleUser        = "user"
	StatusActive    = "active"
	StatusInactive  = "inactive"
	StatusSuspended = "suspended"
)
