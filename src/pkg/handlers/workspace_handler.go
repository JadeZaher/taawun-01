package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"taawun/pkg/models"
	"taawun/pkg/services"
)

type WorkspaceHandler struct {
	service *services.WorkspaceService
}

func NewWorkspaceHandler(service *services.WorkspaceService) *WorkspaceHandler {
	return &WorkspaceHandler{service: service}
}

func (h *WorkspaceHandler) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r.Context())
	if !ok {
		http.Error(w, "User not found in context", http.StatusUnauthorized)
		return
	}

	var req models.CreateWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	workspace, err := h.service.CreateWorkspace(user, &req)
	if err != nil {
		writeWorkspaceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(workspace)
}

func (h *WorkspaceHandler) GetWorkspaces(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r.Context())
	if !ok {
		http.Error(w, "User not found in context", http.StatusUnauthorized)
		return
	}

	workspaces, err := h.service.GetWorkspaces(user)
	if err != nil {
		writeWorkspaceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workspaces)
}

func (h *WorkspaceHandler) GetWorkspace(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r.Context())
	if !ok {
		http.Error(w, "User not found in context", http.StatusUnauthorized)
		return
	}
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid workspace ID", http.StatusBadRequest)
		return
	}

	workspace, err := h.service.GetWorkspace(user, id)
	if err != nil {
		writeWorkspaceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workspace)
}

func (h *WorkspaceHandler) GetWorkspacePeople(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	user, ok := CurrentUser(r.Context())
	if !ok {
		writeWorkspacePeopleError(w, http.StatusUnauthorized, "authentication_required", "Authentication is required.")
		return
	}
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil || id <= 0 {
		writeWorkspacePeopleError(w, http.StatusBadRequest, "invalid_workspace", "Workspace ID must be positive.")
		return
	}
	members, err := h.service.GetWorkspaceMembers(user, id)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrWorkspaceForbidden):
			writeWorkspacePeopleError(w, http.StatusForbidden, "workspace_forbidden", "This account is not authorized to view this workspace.")
		case errors.Is(err, services.ErrWorkspaceNotFound):
			writeWorkspacePeopleError(w, http.StatusNotFound, "workspace_not_found", "Workspace was not found.")
		default:
			writeWorkspacePeopleError(w, http.StatusInternalServerError, "workspace_people_unavailable", "Workspace people could not be loaded.")
		}
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"members": members})
}

func writeWorkspacePeopleError(w http.ResponseWriter, status int, code, message string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func (h *WorkspaceHandler) UpdateWorkspace(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r.Context())
	if !ok {
		http.Error(w, "User not found in context", http.StatusUnauthorized)
		return
	}
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid workspace ID", http.StatusBadRequest)
		return
	}

	var req models.UpdateWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	workspace, err := h.service.UpdateWorkspace(user, id, &req)
	if err != nil {
		writeWorkspaceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workspace)
}

func (h *WorkspaceHandler) DeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r.Context())
	if !ok {
		http.Error(w, "User not found in context", http.StatusUnauthorized)
		return
	}
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid workspace ID", http.StatusBadRequest)
		return
	}

	if err := h.service.DeleteWorkspace(user, id); err != nil {
		writeWorkspaceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *WorkspaceHandler) AddUserToWorkspace(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r.Context())
	if !ok {
		http.Error(w, "User not found in context", http.StatusUnauthorized)
		return
	}
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid workspace ID", http.StatusBadRequest)
		return
	}

	var req models.AddUserToWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.AddUserToWorkspace(user, id, req.UserID, req.Role); err != nil {
		writeWorkspaceError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *WorkspaceHandler) RemoveUserFromWorkspace(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r.Context())
	if !ok {
		http.Error(w, "User not found in context", http.StatusUnauthorized)
		return
	}
	vars := mux.Vars(r)
	workspaceID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid workspace ID", http.StatusBadRequest)
		return
	}

	userID, err := strconv.Atoi(vars["user_id"])
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if err := h.service.RemoveUserFromWorkspace(user, workspaceID, userID); err != nil {
		writeWorkspaceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *WorkspaceHandler) InitializeWorkspace(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r.Context())
	if !ok {
		http.Error(w, "User not found in context", http.StatusUnauthorized)
		return
	}
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid workspace ID", http.StatusBadRequest)
		return
	}

	if err := h.service.InitializeWorkspace(user, id); err != nil {
		writeWorkspaceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "initialized"})
}

func writeWorkspaceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrWorkspaceForbidden):
		http.Error(w, "Workspace access forbidden", http.StatusForbidden)
	case errors.Is(err, services.ErrWorkspaceNotFound):
		http.Error(w, "Workspace not found", http.StatusNotFound)
	default:
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}
