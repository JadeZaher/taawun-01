package services

import (
	"context"
	"errors"
	"fmt"

	"taawun/pkg/models"
	"taawun/pkg/repositories"
)

var (
	ErrWorkspaceForbidden = errors.New("workspace access forbidden")
	ErrWorkspaceNotFound  = errors.New("workspace not found")
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

func (s *WorkspaceService) CreateWorkspace(actor *models.User, req *models.CreateWorkspaceRequest) (*models.Workspace, error) {
	if actor == nil || actor.ID <= 0 {
		return nil, ErrWorkspaceForbidden
	}
	if req.Name == "" {
		return nil, fmt.Errorf("workspace name is required")
	}

	workspace := &models.Workspace{
		Name:        req.Name,
		Description: req.Description,
		OwnerID:     actor.ID,
		Status:      models.WorkspaceStatusActive,
	}

	if err := s.workspaceRepo.Create(workspace); err != nil {
		return nil, fmt.Errorf("failed to create workspace: %v", err)
	}

	// Add owner as member with owner role
	if err := s.workspaceRepo.AddUser(workspace.ID, actor.ID, models.WorkspaceRoleOwner); err != nil {
		return nil, fmt.Errorf("failed to add owner to workspace: %v", err)
	}

	return workspace, nil
}

func (s *WorkspaceService) GetWorkspace(actor *models.User, id int) (*models.Workspace, error) {
	return s.authorize(actor, id, models.WorkspaceRoleOwner, models.WorkspaceRoleAdmin, models.WorkspaceRoleMember, models.WorkspaceRoleViewer)
}

// AuthorizeWorkspaceCapability maps persisted workspace roles to product capabilities.
func (s *WorkspaceService) AuthorizeWorkspaceCapability(actor *models.User, id int, capability models.WorkspaceCapability) (*models.Workspace, error) {
	var allowedRoles []string
	switch capability {
	case models.WorkspaceCapabilityView, models.WorkspaceCapabilityAudit:
		allowedRoles = []string{models.WorkspaceRoleOwner, models.WorkspaceRoleAdmin, models.WorkspaceRoleMember, models.WorkspaceRoleViewer}
	case models.WorkspaceCapabilityBuild:
		allowedRoles = []string{models.WorkspaceRoleOwner, models.WorkspaceRoleAdmin, models.WorkspaceRoleMember}
	case models.WorkspaceCapabilityPublish:
		allowedRoles = []string{models.WorkspaceRoleOwner, models.WorkspaceRoleAdmin}
	default:
		return nil, ErrWorkspaceForbidden
	}
	workspace, err := s.authorize(actor, id, allowedRoles...)
	if err != nil {
		return nil, err
	}
	if (capability == models.WorkspaceCapabilityBuild || capability == models.WorkspaceCapabilityPublish) && workspace.Status != models.WorkspaceStatusActive {
		return nil, ErrWorkspaceForbidden
	}
	return workspace, nil
}

func (s *WorkspaceService) GetWorkspaces(actor *models.User) ([]*models.Workspace, error) {
	if actor == nil || actor.ID <= 0 {
		return nil, ErrWorkspaceForbidden
	}
	if actor.Role == models.RoleAdmin {
		workspaces, err := s.workspaceRepo.GetAll()
		if err != nil {
			return nil, fmt.Errorf("failed to get workspaces: %v", err)
		}
		return workspaces, nil
	}
	workspaces, err := s.workspaceRepo.GetByUser(actor.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspaces: %v", err)
	}
	return workspaces, nil
}

// GetWorkspaceMembers exposes the authoritative membership view to workspace members.
func (s *WorkspaceService) GetWorkspaceMembers(actor *models.User, id int) ([]models.WorkspaceMember, error) {
	if _, err := s.AuthorizeWorkspaceCapability(actor, id, models.WorkspaceCapabilityView); err != nil {
		return nil, err
	}
	return s.workspaceRepo.GetMembers(id)
}

func (s *WorkspaceService) UpdateWorkspace(actor *models.User, id int, req *models.UpdateWorkspaceRequest) (*models.Workspace, error) {
	workspace, err := s.authorize(actor, id, models.WorkspaceRoleOwner, models.WorkspaceRoleAdmin)
	if err != nil {
		return nil, err
	}

	if req.Name != "" {
		workspace.Name = req.Name
	}
	if req.Description != "" {
		workspace.Description = req.Description
	}
	if req.Status != "" {
		if !validWorkspaceStatus(req.Status) {
			return nil, fmt.Errorf("invalid workspace status")
		}
		workspace.Status = req.Status
	}

	if err := s.workspaceRepo.Update(workspace); err != nil {
		return nil, fmt.Errorf("failed to update workspace: %v", err)
	}

	return workspace, nil
}

func (s *WorkspaceService) DeleteWorkspace(actor *models.User, id int) error {
	if _, err := s.authorize(actor, id, models.WorkspaceRoleOwner); err != nil {
		return err
	}
	if err := s.workspaceRepo.Delete(id); err != nil {
		return fmt.Errorf("failed to delete workspace: %v", err)
	}
	return nil
}

func (s *WorkspaceService) AddUserToWorkspace(actor *models.User, workspaceID, userID int, role string) error {
	if _, err := s.authorize(actor, workspaceID, models.WorkspaceRoleOwner, models.WorkspaceRoleAdmin); err != nil {
		return err
	}
	if role == "" {
		role = models.WorkspaceRoleMember
	}
	if !validAssignableWorkspaceRole(role) {
		return fmt.Errorf("invalid workspace role")
	}
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

	if err := s.workspaceRepo.AddUser(workspaceID, userID, role); err != nil {
		return fmt.Errorf("failed to add user to workspace: %v", err)
	}

	return nil
}

// AcceptWorkspaceInvitation adds the authenticated invitee without granting them self-service authority.
func (s *WorkspaceService) AcceptWorkspaceInvitation(_ context.Context, actor *models.User, workspaceID int, role string) error {
	if actor == nil || actor.ID <= 0 || !validAssignableWorkspaceRole(role) {
		return ErrWorkspaceForbidden
	}
	workspace, err := s.workspaceRepo.GetByID(workspaceID)
	if err != nil {
		return fmt.Errorf("failed to get workspace: %v", err)
	}
	if workspace == nil {
		return ErrWorkspaceNotFound
	}
	if workspace.Status != models.WorkspaceStatusActive {
		return ErrWorkspaceForbidden
	}
	existingRole, err := s.workspaceRepo.GetUserRole(workspaceID, actor.ID)
	if err != nil {
		return fmt.Errorf("failed to check user role: %v", err)
	}
	if existingRole != "" {
		return nil
	}
	if err := s.workspaceRepo.AddUser(workspaceID, actor.ID, role); err != nil {
		return fmt.Errorf("failed to accept workspace invitation: %v", err)
	}
	return nil
}

func (s *WorkspaceService) RemoveUserFromWorkspace(actor *models.User, workspaceID, userID int) error {
	if _, err := s.authorize(actor, workspaceID, models.WorkspaceRoleOwner, models.WorkspaceRoleAdmin); err != nil {
		return err
	}
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

func (s *WorkspaceService) InitializeWorkspace(actor *models.User, workspaceID int) error {
	if _, err := s.authorize(actor, workspaceID, models.WorkspaceRoleOwner, models.WorkspaceRoleAdmin); err != nil {
		return err
	}

	// Perform initialization tasks
	// This is a placeholder for actual initialization logic
	// In a real application, this might create default settings,
	// initialize resources, etc.

	return nil
}

func (s *WorkspaceService) GetWorkspaceUsers(actor *models.User, workspaceID int) ([]*models.User, error) {
	if _, err := s.authorize(actor, workspaceID, models.WorkspaceRoleOwner, models.WorkspaceRoleAdmin, models.WorkspaceRoleMember, models.WorkspaceRoleViewer); err != nil {
		return nil, err
	}
	users, err := s.userRepo.GetWorkspaceUsers(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace users: %v", err)
	}
	return users, nil
}

func (s *WorkspaceService) authorize(actor *models.User, workspaceID int, allowedRoles ...string) (*models.Workspace, error) {
	if actor == nil || actor.ID <= 0 {
		return nil, ErrWorkspaceForbidden
	}
	persistedActor, err := s.userRepo.GetByID(actor.ID)
	if err != nil || persistedActor == nil || persistedActor.Status != models.StatusActive {
		return nil, ErrWorkspaceForbidden
	}
	actor = persistedActor
	workspace, err := s.workspaceRepo.GetByID(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace: %v", err)
	}
	if workspace == nil {
		return nil, ErrWorkspaceNotFound
	}
	if actor.Role == models.RoleAdmin {
		return workspace, nil
	}

	role := ""
	if workspace.OwnerID == actor.ID {
		role = models.WorkspaceRoleOwner
	} else {
		role, err = s.workspaceRepo.GetUserRole(workspaceID, actor.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get workspace role: %v", err)
		}
	}
	for _, allowedRole := range allowedRoles {
		if role == allowedRole {
			return workspace, nil
		}
	}
	return nil, ErrWorkspaceForbidden
}

func validAssignableWorkspaceRole(role string) bool {
	switch role {
	case models.WorkspaceRoleAdmin, models.WorkspaceRoleMember, models.WorkspaceRoleViewer:
		return true
	default:
		return false
	}
}

func validWorkspaceStatus(status string) bool {
	switch status {
	case models.WorkspaceStatusActive, models.WorkspaceStatusInactive, models.WorkspaceStatusArchived:
		return true
	default:
		return false
	}
}
