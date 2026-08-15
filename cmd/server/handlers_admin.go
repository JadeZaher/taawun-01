package main

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

func listUsersHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`SELECT id, email, name, role, created_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		jsonError(w, "Error fetching users", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.CreatedAt); err != nil {
			continue
		}
		users = append(users, u)
	}
	jsonResponse(w, users, http.StatusOK)
}

func setUserRoleHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var req RoleUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if !platformRoles[req.Role] {
		jsonError(w, "Role must be admin, architect, maintainer, or viewer", http.StatusBadRequest)
		return
	}
	var currentRole string
	if err := db.QueryRow(`SELECT role FROM users WHERE id = ?`, id).Scan(&currentRole); err != nil {
		jsonError(w, "User not found", http.StatusNotFound)
		return
	}
	if currentRole == RoleAdmin && req.Role != RoleAdmin {
		var admins int
		db.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&admins)
		if admins <= 1 {
			jsonError(w, "Cannot demote the last admin", http.StatusConflict)
			return
		}
	}
	db.Exec(`UPDATE users SET role = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, req.Role, id)
	jsonResponse(w, MessageResponse{Message: "Role updated"}, http.StatusOK)
}
