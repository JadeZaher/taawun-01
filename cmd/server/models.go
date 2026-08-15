package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ---------- Roles (RBAC) ----------
const (
	RoleAdmin      = "admin"
	RoleArchitect  = "architect"
	RoleMaintainer = "maintainer"
	RoleViewer     = "viewer"
)

var platformRoles = map[string]bool{RoleAdmin: true, RoleArchitect: true, RoleMaintainer: true, RoleViewer: true}
var workspaceRoles = map[string]bool{RoleArchitect: true, RoleMaintainer: true, RoleViewer: true}

func canCreateWorkspace(role string) bool { return role == RoleAdmin || role == RoleArchitect }

// ---------- Models ----------
type User struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Password  string `json:"-"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

type Workspace struct {
	ID          string `json:"id"`
	OwnerID     string `json:"owner_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	UserRole    string `json:"current_user_role,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type WorkspaceMember struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

// ---------- DTOs ----------
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}
type APIError struct {
	Error string `json:"error"`
}
type MessageResponse struct {
	Message string `json:"message"`
}
type WorkspaceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
type MemberRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}
type RoleUpdateRequest struct {
	Role string `json:"role"`
}

// ---------- Utilities ----------
func jsonResponse(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, message string, status int) {
	jsonResponse(w, APIError{Error: message}, status)
}

func generateID() string { return fmt.Sprintf("%d", time.Now().UnixNano()) }

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
