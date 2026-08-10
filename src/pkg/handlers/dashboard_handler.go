package handlers

import (
	"encoding/json"
	"net/http"

	"taawun/pkg/models"
	"taawun/pkg/services"
)

type DashboardHandler struct {
	userService         *services.UserService
	workspaceService    *services.WorkspaceService
	notificationService *services.NotificationService
}

func NewDashboardHandler(userService *services.UserService, workspaceService *services.WorkspaceService, notificationService *services.NotificationService) *DashboardHandler {
	return &DashboardHandler{
		userService:         userService,
		workspaceService:    workspaceService,
		notificationService: notificationService,
	}
}

func (h *DashboardHandler) GetDashboardStats(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value("user").(*models.User)
	if !ok {
		http.Error(w, "User not found in context", http.StatusUnauthorized)
		return
	}

	// Get user-specific stats
	workspaces, err := h.workspaceService.GetWorkspaces(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	notifications, err := h.notificationService.GetUserNotifications(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	unreadCount, err := h.notificationService.GetUnreadCount(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	stats := &models.DashboardStats{
		TotalUsers:      1, // Placeholder - in real app, would count from DB
		ActiveUsers:     1,
		TotalWorkspaces: len(workspaces),
		ActiveWorkspaces: countActiveWorkspaces(workspaces),
		TotalNotifications: len(notifications),
		UnreadNotifications: unreadCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *DashboardHandler) GetRecentActivity(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value("user").(*models.User)
	if !ok {
		http.Error(w, "User not found in context", http.StatusUnauthorized)
		return
	}

	// Get recent activity for the user
	// This is a placeholder - in a real app, we'd have an activity log
	activities := []models.RecentActivity{
		{
			ID:        1,
			Type:      "login",
			User:      user.Username,
			Action:    "Logged in",
			Timestamp: "2024-01-01T00:00:00Z",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(activities)
}

func countActiveWorkspaces(workspaces []*models.Workspace) int {
	count := 0
	for _, w := range workspaces {
		if w.Status == models.WorkspaceStatusActive {
			count++
		}
	}
	return count
}