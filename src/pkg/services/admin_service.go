package services

import (
	"fmt"
	"taawun/pkg/models"
	"taawun/pkg/repositories"
)

type AdminService struct {
	userRepo         *repositories.UserRepository
	workspaceRepo    *repositories.WorkspaceRepository
	notificationRepo *repositories.NotificationRepository
}

func NewAdminService(userRepo *repositories.UserRepository, workspaceRepo *repositories.WorkspaceRepository, notificationRepo *repositories.NotificationRepository) *AdminService {
	return &AdminService{
		userRepo:         userRepo,
		workspaceRepo:    workspaceRepo,
		notificationRepo: notificationRepo,
	}
}

func (s *AdminService) GetAllUsers() ([]*models.User, error) {
	users, err := s.userRepo.GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %v", err)
	}
	return users, nil
}

func (s *AdminService) UpdateUserRole(userID int, role string) error {
	if role != models.RoleAdmin && role != models.RoleUser {
		return fmt.Errorf("invalid role: %s", role)
	}

	if err := s.userRepo.UpdateRole(userID, role); err != nil {
		return fmt.Errorf("failed to update user role: %v", err)
	}
	return nil
}

func (s *AdminService) UpdateUserStatus(userID int, status string) error {
	if status != models.StatusActive && status != models.StatusInactive && status != models.StatusSuspended {
		return fmt.Errorf("invalid status: %s", status)
	}

	if err := s.userRepo.UpdateStatus(userID, status); err != nil {
		return fmt.Errorf("failed to update user status: %v", err)
	}
	return nil
}

func (s *AdminService) GetAllWorkspaces() ([]*models.Workspace, error) {
	workspaces, err := s.workspaceRepo.GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get workspaces: %v", err)
	}
	return workspaces, nil
}

func (s *AdminService) DeleteWorkspace(workspaceID int) error {
	if err := s.workspaceRepo.Delete(workspaceID); err != nil {
		return fmt.Errorf("failed to delete workspace: %v", err)
	}
	return nil
}

func (s *AdminService) GetStatistics() (*models.Statistics, error) {
	totalUsers, err := s.userRepo.Count()
	if err != nil {
		return nil, fmt.Errorf("failed to count users: %v", err)
	}

	activeUsers, err := s.userRepo.CountActive()
	if err != nil {
		return nil, fmt.Errorf("failed to count active users: %v", err)
	}

	totalWorkspaces, err := s.workspaceRepo.Count()
	if err != nil {
		return nil, fmt.Errorf("failed to count workspaces: %v", err)
	}

	activeWorkspaces, err := s.workspaceRepo.CountActive()
	if err != nil {
		return nil, fmt.Errorf("failed to count active workspaces: %v", err)
	}

	// Count users by role
	usersByRole := make(map[string]int)
	for _, role := range []string{models.RoleAdmin, models.RoleUser} {
		count, err := s.userRepo.CountByRole(role)
		if err != nil {
			return nil, fmt.Errorf("failed to count users by role: %v", err)
		}
		usersByRole[role] = count
	}

	// Count workspaces by status
	workspacesByStatus := make(map[string]int)
	for _, status := range []string{models.WorkspaceStatusActive, models.WorkspaceStatusInactive, models.WorkspaceStatusArchived} {
		count, err := s.workspaceRepo.CountByStatus(status)
		if err != nil {
			return nil, fmt.Errorf("failed to count workspaces by status: %v", err)
		}
		workspacesByStatus[status] = count
	}

	return &models.Statistics{
		TotalUsers:       totalUsers,
		ActiveUsers:      activeUsers,
		TotalWorkspaces:  totalWorkspaces,
		ActiveWorkspaces: activeWorkspaces,
		UsersByRole:      usersByRole,
		WorkspacesByStatus: workspacesByStatus,
		RecentActivities: []models.RecentActivity{},
	}, nil
}