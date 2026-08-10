package models

type DashboardStats struct {
	TotalUsers      int `json:"total_users"`
	ActiveUsers     int `json:"active_users"`
	TotalWorkspaces int `json:"total_workspaces"`
	ActiveWorkspaces int `json:"active_workspaces"`
	TotalNotifications int `json:"total_notifications"`
	UnreadNotifications int `json:"unread_notifications"`
}

type RecentActivity struct {
	ID        int    `json:"id"`
	Type      string `json:"type"`
	User      string `json:"user"`
	Action    string `json:"action"`
	Timestamp string `json:"timestamp"`
}

type Statistics struct {
	TotalUsers      int                     `json:"total_users"`
	ActiveUsers     int                     `json:"active_users"`
	TotalWorkspaces int                     `json:"total_workspaces"`
	ActiveWorkspaces int                    `json:"active_workspaces"`
	UsersByRole     map[string]int          `json:"users_by_role"`
	WorkspacesByStatus map[string]int       `json:"workspaces_by_status"`
	RecentActivities []RecentActivity       `json:"recent_activities"`
}