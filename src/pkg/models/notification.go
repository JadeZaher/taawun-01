package models

import (
	"time"
)

type Notification struct {
	ID        int       `json:"id" db:"id"`
	UserID    int       `json:"user_id" db:"user_id"`
	Type      string    `json:"type" db:"type"`
	Title     string    `json:"title" db:"title"`
	Message   string    `json:"message" db:"message"`
	Read      bool      `json:"read" db:"read"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type CreateNotificationRequest struct {
	UserID  int    `json:"user_id"`
	Type    string `json:"type"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

const (
	NotificationTypeWorkspace    = "workspace"
	NotificationTypeUser         = "user"
	NotificationTypeSystem       = "system"
	NotificationTypeInvitation   = "invitation"
)