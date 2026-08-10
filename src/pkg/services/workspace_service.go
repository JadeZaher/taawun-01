package services

import (
	"fmt"
	"taawun/pkg/models"
	"taawun/pkg/repositories"
)

type WorkspaceService struct {
	workspaceRepo *repositories.WorkspaceRepository
	userRepo      *repositories.UserRepository
}

func NewWorkspaceService(workspaceRepo *repositories.WorkspaceRepository, userRepo *repositories.UserRepository) *WorkspaceService {
	return &WorkspaceService{
		workspaceRepo: workspaceRepo,
		userRepo:      userRepo,
	}
}

func (s *WorkspaceService) CreateWorkspace(userID int, req *models.CreateWorkspaceRequest) (*models.Workspace, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("workspace name is required")
	}

	workspace := &models.Workspace{
		Name:        req.Name,
		Description: req.Description,
		OwnerID:     userID,
		Status:      models.WorkspaceStatusActive,
	}

	if err := s.workspaceRepo.Create(workspace); err != nil {
		return nil, fmt.Errorf("failed to create workspace: %v", err)
	}

	// Add owner as member with owner role
	if err := s.workspaceRepo.AddUser(workspace.ID, userID, models.WorkspaceRoleOwner); err != nil {
		return nil, fmt.Errorf("failed to add owner to workspace: %v", err)
	}

	return workspace, nil
}

func (s *WorkspaceService) GetWorkspace(id int) (*models.Workspace, error) {
	workspace, err := s.workspaceRepo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace: %v", err)
	}
	if workspace == nil {
		return nil, fmt.Errorf("workspace not found")
	}
	return workspace, nil
}

func (s *WorkspaceService) GetWorkspaces(userID int) ([]*models.Workspace, error) {
	workspaces, err := s.workspaceRepo.GetByUser(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspaces: %v", err)
	}
	return workspaces, nil
}

func (s *WorkspaceService) UpdateWorkspace(id int, req *models.UpdateWorkspaceRequest) (*models.Workspace, error) {
	workspace, err := s.workspaceRepo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace: %v", err)
	}
	if workspace == nil {
		return nil, fmt.Errorf("workspace not found")
	}

	if req.Name != "" {
		workspace.Name = req.Name
	}
	if req.Description != "" {
		workspace.Description = req.Description
	}
	if req.Status != "" {
		workspace.Status = req.Status
	}

	if err := s.workspaceRepo.Update(workspace); err != nil {
		return nil, fmt.Errorf("failed to update workspace: %v", err)
	}

	return workspace, nil
}

func (s *WorkspaceService) DeleteWorkspace(id int) error {
	if err := s.workspaceRepo.Delete(id); err != nil {
		return fmt.Errorf("failed to delete workspace: %v", err)
	}
	return nil
}

func (s *WorkspaceService) AddUserToWorkspace(workspaceID, userID int, role string) error {
	// Check if user exists
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %v", err)
	}
	if user == nil {
		return fmt.Errorf("user not found")
	}

	// Check if workspace exists
	workspace, err := s.workspaceRepo.GetByID(workspaceID)
	if err != nil {
		return fmt.Errorf("failed to get workspace: %v", err)
	}
	if workspace == nil {
		return fmt.Errorf("workspace not found")
	}

	// Check if user already in workspace
	existingRole, err := s.workspaceRepo.GetUserRole(workspaceID, userID)
	if err != nil {
		return fmt.Errorf("failed to check user role: %v", err)
	}
	if existingRole != "" {
		return fmt.Errorf("user already in workspace")
	}

	if role == "" {
		role = models.WorkspaceRoleMember
	}

	if err := s.workspaceRepo.AddUser(workspaceID, userID, role); err != nil {
		return fmt.Errorf("failed to add user to workspace: %v", err)
	}

	return nil
}

func (s *WorkspaceService) RemoveUserFromWorkspace(workspaceID, userID int) error {
	// Check if user exists
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %v", err)
	}
	if user == nil {
		return fmt.Errorf("user not found")
	}

	// Check if workspace exists
	workspace, err := s.workspaceRepo.GetByID(workspaceID)
	if err != nil {
		return fmt.Errorf("failed to get workspace: %v", err)
	}
	if workspace == nil {
		return fmt.Errorf("workspace not found")
	}

	// Cannot remove owner
	role, err := s.workspaceRepo.GetUserRole(workspaceID, userID)
	if err != nil {
		return fmt.Errorf("failed to get user role: %v", err)
	}
	if role == models.WorkspaceRoleOwner {
		return fmt.Errorf("cannot remove workspace owner")
	}

	if err := s.workspaceRepo.RemoveUser(workspaceID, userID); err != nil {
		return fmt.Errorf("failed to remove user from workspace: %v", err)
	}

	return nil
}

func (s *WorkspaceService) InitializeWorkspace(workspaceID int) error {
	workspace, err := s.workspaceRepo.GetByID(workspaceID)
	if err != nil {
		return fmt.Errorf("failed to get workspace: %v", err)
	}
	if workspace == nil {
		return fmt.Errorf("workspace not found")
	}

	// Perform initialization tasks
	// This is a placeholder for actual initialization logic
	// In a real application, this might create default settings,
	// initialize resources, etc.

	return nil
}

func (s *WorkspaceService) GetWorkspaceUsers(workspaceID int) ([]*models.User, error) {
	users, err := s.userRepo.GetWorkspaceUsers(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace users: %v", err)
	}
	return users, nil
}