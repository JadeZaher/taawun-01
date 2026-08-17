package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func registerHandler(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.Password == "" || req.Name == "" {
		jsonError(w, "Email, password, and name are required", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 6 {
		jsonError(w, "Password must be at least 6 characters", http.StatusBadRequest)
		return
	}
	var exists int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE email = ?", req.Email).Scan(&exists)
	if exists > 0 {
		jsonError(w, "User already exists", http.StatusConflict)
		return
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		jsonError(w, "Error creating user", http.StatusInternalServerError)
		return
	}
	user := User{ID: generateID(), Email: req.Email, Password: string(hashed), Name: req.Name, Role: RoleArchitect}
	_, err = db.Exec(`INSERT INTO users (id, email, password, name, role) VALUES (?, ?, ?, ?, ?)`,
		user.ID, user.Email, user.Password, user.Name, user.Role)
	if err != nil {
		jsonError(w, "Error creating user", http.StatusInternalServerError)
		return
	}
	token, err := generateToken(user)
	if err != nil {
		jsonError(w, "Error generating token", http.StatusInternalServerError)
		return
	}
	setAuthCookies(w, token)
	user.Password = ""
	jsonResponse(w, LoginResponse{Token: token, User: user}, http.StatusCreated)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.Password == "" {
		jsonError(w, "Email and password are required", http.StatusBadRequest)
		return
	}
	var user User
	err := db.QueryRow(`SELECT id, email, password, name, role FROM users WHERE email = ?`, req.Email).
		Scan(&user.ID, &user.Email, &user.Password, &user.Name, &user.Role)
	if err == sql.ErrNoRows {
		jsonError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	if err != nil {
		jsonError(w, "Database error", http.StatusInternalServerError)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		jsonError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	token, err := generateToken(user)
	if err != nil {
		jsonError(w, "Error generating token", http.StatusInternalServerError)
		return
	}
	setAuthCookies(w, token)
	user.Password = ""
	jsonResponse(w, LoginResponse{Token: token, User: user}, http.StatusOK)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	if tokenID, ok := r.Context().Value(ctxTokenID).(string); ok {
		db.Exec(`INSERT OR IGNORE INTO revoked_tokens (token_id, expires_at) VALUES (?, ?)`,
			tokenID, time.Now().Add(24*time.Hour).Format("2006-01-02 15:04:05"))
	}
	clearAuthCookies(w)
	jsonResponse(w, MessageResponse{Message: "Logged out successfully"}, http.StatusOK)
}

func profileHandler(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(ctxUserID).(string)
	var user User
	err := db.QueryRow(`SELECT id, email, name, role, created_at FROM users WHERE id = ?`, userID).
		Scan(&user.ID, &user.Email, &user.Name, &user.Role, &user.CreatedAt)
	if err != nil {
		jsonError(w, "User not found", http.StatusNotFound)
		return
	}
	jsonResponse(w, user, http.StatusOK)
}
