package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"taawun/pkg/models"
	"taawun/pkg/services"
)

type UserHandler struct {
	service *services.UserService
}

func NewUserHandler(service *services.UserService) *UserHandler {
	return &UserHandler{service: service}
}

func (h *UserHandler) GetUsers(w http.ResponseWriter, r *http.Request) {
	if !requireAdminUser(w, r) {
		return
	}
	users, err := h.service.GetAllUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}
	if !requireSelfOrAdmin(w, r, id) {
		return
	}

	user, err := h.service.GetUser(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}
	if !requireSelfOrAdmin(w, r, id) {
		return
	}

	var req models.UpdateUserRequest
	if accepted, _ := decodeBoundedJSON(w, r, &req, maximumPublicAuthBodyBytes); !accepted {
		return
	}

	user, err := h.service.UpdateUser(id, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}
	if !requireSelfOrAdmin(w, r, id) {
		return
	}

	if err := h.service.DeleteUser(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r.Context())
	if !ok {
		http.Error(w, "User not found in context", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func requireAdminUser(w http.ResponseWriter, r *http.Request) bool {
	user, ok := CurrentUser(r.Context())
	if !ok {
		http.Error(w, "User not found in context", http.StatusUnauthorized)
		return false
	}
	if user.Role != models.RoleAdmin {
		http.Error(w, "Administrator access required", http.StatusForbidden)
		return false
	}
	return true
}

func requireSelfOrAdmin(w http.ResponseWriter, r *http.Request, targetUserID int) bool {
	user, ok := CurrentUser(r.Context())
	if !ok {
		http.Error(w, "User not found in context", http.StatusUnauthorized)
		return false
	}
	if user.ID != targetUserID && user.Role != models.RoleAdmin {
		http.Error(w, "User access forbidden", http.StatusForbidden)
		return false
	}
	return true
}
