package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

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
	user, ok := CurrentUser(r.Context())
	if !ok {
		http.Error(w, "User not found in context", http.StatusUnauthorized)
		return
	}

	workspaces, members, err := h.workspaceEvidence(user)
	if err != nil {
		http.Error(w, "Could not load dashboard evidence", http.StatusInternalServerError)
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
		Scope:               "accessible_workspaces",
		EvidenceState:       "ready",
		TotalUsers:          len(members),
		ActiveUsers:         countActiveUsers(members),
		TotalWorkspaces:     len(workspaces),
		ActiveWorkspaces:    countActiveWorkspaces(workspaces),
		TotalNotifications:  len(notifications),
		UnreadNotifications: unreadCount,
	}
	if len(workspaces) == 0 {
		stats.EvidenceState = "empty"
		stats.EmptyState = "No workspace evidence yet. Create or join a workspace to begin."
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *DashboardHandler) GetRecentActivity(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUser(r.Context())
	if !ok {
		http.Error(w, "User not found in context", http.StatusUnauthorized)
		return
	}

	workspaces, members, err := h.workspaceEvidence(user)
	if err != nil {
		http.Error(w, "Could not load dashboard activity", http.StatusInternalServerError)
		return
	}
	activities := workspaceActivities(workspaces, members)
	feed := &models.RecentActivityFeed{
		Scope:      "accessible_workspaces",
		Activities: activities,
	}
	if len(activities) == 0 {
		feed.EmptyState = "No recorded workspace activity yet."
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(feed)
}

func (h *DashboardHandler) workspaceEvidence(user *models.User) ([]*models.Workspace, map[int]*models.User, error) {
	workspaces, err := h.workspaceService.GetWorkspaces(user)
	if err != nil {
		return nil, nil, err
	}
	members := make(map[int]*models.User)
	for _, workspace := range workspaces {
		workspaceUsers, err := h.workspaceService.GetWorkspaceUsers(user, workspace.ID)
		if err != nil {
			return nil, nil, err
		}
		for _, member := range workspaceUsers {
			members[member.ID] = member
		}
	}
	return workspaces, members, nil
}

func workspaceActivities(workspaces []*models.Workspace, members map[int]*models.User) []models.RecentActivity {
	activities := make([]models.RecentActivity, 0, len(workspaces))
	for _, workspace := range workspaces {
		if workspace == nil || workspace.CreatedAt.IsZero() {
			continue
		}
		owner := ""
		if member := members[workspace.OwnerID]; member != nil {
			owner = member.Username
		}
		activities = append(activities, models.RecentActivity{
			ID:          workspace.ID,
			WorkspaceID: workspace.ID,
			Workspace:   workspace.Name,
			Type:        "workspace_created",
			User:        owner,
			Action:      "Workspace created",
			Timestamp:   workspace.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	sort.SliceStable(activities, func(i, j int) bool {
		if activities[i].Timestamp == activities[j].Timestamp {
			return activities[i].WorkspaceID > activities[j].WorkspaceID
		}
		return activities[i].Timestamp > activities[j].Timestamp
	})
	return activities
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

func countActiveUsers(users map[int]*models.User) int {
	count := 0
	for _, user := range users {
		if user.Status == models.StatusActive {
			count++
		}
	}
	return count
}
