package main

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

// wsAccess returns the workspace owner and the caller's workspace role.
func wsAccess(wsID, userID string) (ownerID string, wsRole string, err error) {
	err = db.QueryRow(`SELECT owner_id FROM workspaces WHERE id = ?`, wsID).Scan(&ownerID)
	if err != nil {
		return "", "", err
	}
	if ownerID == userID {
		return ownerID, RoleArchitect, nil
	}
	db.QueryRow(`SELECT role FROM workspace_members WHERE workspace_id = ? AND user_id = ?`, wsID, userID).Scan(&wsRole)
	return ownerID, wsRole, nil
}

func createWorkspaceHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(ctxUserID).(string)
	var req WorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		jsonError(w, "Workspace name is required", http.StatusBadRequest)
		return
	}
	ws := Workspace{ID: generateID(), OwnerID: userID, Name: req.Name, Description: req.Description}
	tx, err := db.Begin()
	if err != nil {
		jsonError(w, "Error creating workspace", http.StatusInternalServerError)
		return
	}
	if _, err = tx.Exec(`INSERT INTO workspaces (id, owner_id, name, description) VALUES (?, ?, ?, ?)`,
		ws.ID, ws.OwnerID, ws.Name, ws.Description); err != nil {
		tx.Rollback()
		jsonError(w, "Error creating workspace", http.StatusInternalServerError)
		return
	}
	if _, err = tx.Exec(`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES (?, ?, ?)`,
		ws.ID, userID, RoleArchitect); err != nil {
		tx.Rollback()
		jsonError(w, "Error creating workspace", http.StatusInternalServerError)
		return
	}
	tx.Commit()
	broadcastSignal("workspace_event", map[string]interface{}{"type": "created", "workspace_id": ws.ID})
	jsonResponse(w, ws, http.StatusCreated)
}

func listWorkspacesHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(ctxUserID).(string)
	rows, err := db.Query(`SELECT w.id, w.owner_id, w.name, w.description, w.created_at, w.updated_at, m.role
		FROM workspaces w JOIN workspace_members m ON m.workspace_id = w.id
		WHERE m.user_id = ? ORDER BY w.created_at DESC`, userID)
	if err != nil {
		jsonError(w, "Error fetching workspaces", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	workspaces := []Workspace{}
	for rows.Next() {
		var ws Workspace
		if err := rows.Scan(&ws.ID, &ws.OwnerID, &ws.Name, &ws.Description, &ws.CreatedAt, &ws.UpdatedAt, &ws.UserRole); err != nil {
			continue
		}
		workspaces = append(workspaces, ws)
	}
	jsonResponse(w, workspaces, http.StatusOK)
}

func getWorkspaceHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(ctxUserID).(string)
	platformRole, _ := r.Context().Value(ctxUserRole).(string)
	id := mux.Vars(r)["id"]
	ownerID, wsRole, err := wsAccess(id, userID)
	if err == sql.ErrNoRows {
		jsonError(w, "Workspace not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "Database error", http.StatusInternalServerError)
		return
	}
	if ownerID != userID && wsRole == "" && platformRole != RoleAdmin {
		jsonError(w, "Workspace not found", http.StatusNotFound)
		return
	}
	if platformRole == RoleAdmin && ownerID != userID && wsRole == "" {
		wsRole = RoleAdmin
	}
	var ws Workspace
	db.QueryRow(`SELECT id, owner_id, name, description, created_at, updated_at FROM workspaces WHERE id = ?`, id).
		Scan(&ws.ID, &ws.OwnerID, &ws.Name, &ws.Description, &ws.CreatedAt, &ws.UpdatedAt)
	ws.UserRole = wsRole
	jsonResponse(w, ws, http.StatusOK)
}

func updateWorkspaceHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(ctxUserID).(string)
	platformRole, _ := r.Context().Value(ctxUserRole).(string)
	id := mux.Vars(r)["id"]
	ownerID, wsRole, err := wsAccess(id, userID)
	if err == sql.ErrNoRows {
		jsonError(w, "Workspace not found", http.StatusNotFound)
		return
	}
	canEdit := platformRole == RoleAdmin || ownerID == userID || wsRole == RoleArchitect || wsRole == RoleMaintainer
	if !canEdit {
		jsonError(w, "Maintainer role or higher required", http.StatusForbidden)
		return
	}
	var req WorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	db.Exec(`UPDATE workspaces SET name = ?, description = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, req.Name, req.Description, id)
	broadcastSignal("workspace_event", map[string]interface{}{"type": "updated", "workspace_id": id})
	var ws Workspace
	db.QueryRow(`SELECT id, owner_id, name, description, created_at, updated_at FROM workspaces WHERE id = ?`, id).
		Scan(&ws.ID, &ws.OwnerID, &ws.Name, &ws.Description, &ws.CreatedAt, &ws.UpdatedAt)
	jsonResponse(w, ws, http.StatusOK)
}

func deleteWorkspaceHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(ctxUserID).(string)
	platformRole, _ := r.Context().Value(ctxUserRole).(string)
	id := mux.Vars(r)["id"]
	ownerID, wsRole, err := wsAccess(id, userID)
	if err == sql.ErrNoRows {
		jsonError(w, "Workspace not found", http.StatusNotFound)
		return
	}
	canManage := platformRole == RoleAdmin || ownerID == userID || wsRole == RoleArchitect
	if !canManage {
		jsonError(w, "Architect role required", http.StatusForbidden)
		return
	}
	db.Exec(`DELETE FROM workspaces WHERE id = ?`, id)
	broadcastSignal("workspace_event", map[string]interface{}{"type": "deleted", "workspace_id": id})
	jsonResponse(w, MessageResponse{Message: "Workspace deleted successfully"}, http.StatusOK)
}

func listMembersHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(ctxUserID).(string)
	platformRole, _ := r.Context().Value(ctxUserRole).(string)
	id := mux.Vars(r)["id"]
	ownerID, wsRole, err := wsAccess(id, userID)
	if err == sql.ErrNoRows {
		jsonError(w, "Workspace not found", http.StatusNotFound)
		return
	}
	if ownerID != userID && wsRole == "" && platformRole != RoleAdmin {
		jsonError(w, "Workspace not found", http.StatusNotFound)
		return
	}
	rows, err := db.Query(`SELECT u.id, u.email, u.name, m.role, m.created_at
		FROM workspace_members m JOIN users u ON u.id = m.user_id WHERE m.workspace_id = ? ORDER BY m.created_at`, id)
	if err != nil {
		jsonError(w, "Error fetching members", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	members := []WorkspaceMember{}
	for rows.Next() {
		var m WorkspaceMember
		if err := rows.Scan(&m.UserID, &m.Email, &m.Name, &m.Role, &m.CreatedAt); err != nil {
			continue
		}
		members = append(members, m)
	}
	jsonResponse(w, members, http.StatusOK)
}

func addMemberHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(ctxUserID).(string)
	platformRole, _ := r.Context().Value(ctxUserRole).(string)
	id := mux.Vars(r)["id"]
	ownerID, wsRole, err := wsAccess(id, userID)
	if err == sql.ErrNoRows {
		jsonError(w, "Workspace not found", http.StatusNotFound)
		return
	}
	if platformRole != RoleAdmin && ownerID != userID && wsRole != RoleArchitect {
		jsonError(w, "Architect role required to manage members", http.StatusForbidden)
		return
	}
	var req MemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if !workspaceRoles[req.Role] {
		jsonError(w, "Role must be architect, maintainer, or viewer", http.StatusBadRequest)
		return
	}
	var targetID string
	if err := db.QueryRow(`SELECT id FROM users WHERE email = ?`, req.Email).Scan(&targetID); err != nil {
		jsonError(w, "No user with that email", http.StatusNotFound)
		return
	}
	if _, err := db.Exec(`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES (?, ?, ?)`, id, targetID, req.Role); err != nil {
		jsonError(w, "User is already a member", http.StatusConflict)
		return
	}
	broadcastSignal("workspace_event", map[string]interface{}{"type": "member_added", "workspace_id": id})
	jsonResponse(w, MessageResponse{Message: "Member added"}, http.StatusCreated)
}

func removeMemberHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(ctxUserID).(string)
	platformRole, _ := r.Context().Value(ctxUserRole).(string)
	vars := mux.Vars(r)
	wsID, targetID := vars["id"], vars["userID"]
	ownerID, wsRole, err := wsAccess(wsID, userID)
	if err == sql.ErrNoRows {
		jsonError(w, "Workspace not found", http.StatusNotFound)
		return
	}
	if platformRole != RoleAdmin && ownerID != userID && wsRole != RoleArchitect {
		jsonError(w, "Architect role required to manage members", http.StatusForbidden)
		return
	}
	if targetID == ownerID {
		jsonError(w, "Cannot remove the workspace owner", http.StatusConflict)
		return
	}
	db.Exec(`DELETE FROM workspace_members WHERE workspace_id = ? AND user_id = ?`, wsID, targetID)
	broadcastSignal("workspace_event", map[string]interface{}{"type": "member_removed", "workspace_id": wsID})
	jsonResponse(w, MessageResponse{Message: "Member removed"}, http.StatusOK)
}
