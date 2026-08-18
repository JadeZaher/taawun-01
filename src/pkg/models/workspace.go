package models

import (
	"time"
)

type Workspace struct {
	ID          int       `json:"id" db:"id" gorm:"primaryKey;autoIncrement"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	OwnerID     int       `json:"owner_id" db:"owner_id"`
	Status      string    `json:"status" db:"status"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type WorkspaceUser struct {
	WorkspaceID int       `json:"workspace_id" db:"workspace_id" gorm:"primaryKey"`
	UserID      int       `json:"user_id" db:"user_id" gorm:"primaryKey"`
	Role        string    `json:"role" db:"role" gorm:"not null;default:member"`
	JoinedAt    time.Time `json:"joined_at" db:"joined_at" gorm:"not null;default:CURRENT_TIMESTAMP"`
}

// WorkspaceMember is the workspace-scoped, non-sensitive membership view.
type WorkspaceMember struct {
	UserID   int       `json:"user_id"`
	Username string    `json:"username"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

type CreateWorkspaceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateWorkspaceRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
}

type AddUserToWorkspaceRequest struct {
	UserID int    `json:"user_id"`
	Role   string `json:"role"`
}

// WorkspaceCapability names an operation authorized from persisted workspace membership.
type WorkspaceCapability string

const (
	WorkspaceStatusActive                          = "active"
	WorkspaceStatusInactive                        = "inactive"
	WorkspaceStatusArchived                        = "archived"
	WorkspaceRoleOwner                             = "owner"
	WorkspaceRoleAdmin                             = "admin"
	WorkspaceRoleMember                            = "member"
	WorkspaceRoleViewer                            = "viewer"
	WorkspaceCapabilityView    WorkspaceCapability = "view"
	WorkspaceCapabilityAudit   WorkspaceCapability = "audit"
	WorkspaceCapabilityBuild   WorkspaceCapability = "build"
	WorkspaceCapabilityPublish WorkspaceCapability = "publish"
)
