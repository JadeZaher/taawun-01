package repositories

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"taawun/pkg/models"
)

type WorkspaceRepository struct {
	db *gorm.DB
}

func NewWorkspaceRepository(db *gorm.DB) *WorkspaceRepository {
	return &WorkspaceRepository{db: db}
}

func (r *WorkspaceRepository) Create(workspace *models.Workspace) error {
	if err := r.db.Create(workspace).Error; err != nil {
		return fmt.Errorf("failed to create workspace: %v", err)
	}
	return nil
}

func (r *WorkspaceRepository) GetByID(id int) (*models.Workspace, error) {
	var workspace models.Workspace
	err := r.db.First(&workspace, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace: %v", err)
	}
	return &workspace, nil
}

func (r *WorkspaceRepository) GetAll() ([]*models.Workspace, error) {
	return r.find(r.db, "failed to get workspaces")
}

func (r *WorkspaceRepository) GetByOwner(ownerID int) ([]*models.Workspace, error) {
	return r.find(r.db.Where("owner_id = ?", ownerID), "failed to get workspaces by owner")
}

func (r *WorkspaceRepository) GetByUser(userID int) ([]*models.Workspace, error) {
	var workspaces []*models.Workspace
	err := r.db.Table("workspaces AS w").Select("w.*").
		Joins("INNER JOIN workspace_users AS wu ON w.id = wu.workspace_id").
		Where("wu.user_id = ?", userID).Order("w.id DESC").Scan(&workspaces).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get workspaces by user: %v", err)
	}
	return workspaces, nil
}

func (r *WorkspaceRepository) find(operation *gorm.DB, message string) ([]*models.Workspace, error) {
	var workspaces []*models.Workspace
	if err := operation.Order("id DESC").Find(&workspaces).Error; err != nil {
		return nil, fmt.Errorf("%s: %v", message, err)
	}
	return workspaces, nil
}

func (r *WorkspaceRepository) Update(workspace *models.Workspace) error {
	result := r.db.Model(&models.Workspace{}).Where("id = ?", workspace.ID).Updates(map[string]any{
		"name": workspace.Name, "description": workspace.Description,
		"status": workspace.Status, "updated_at": time.Now(),
	})
	if result.Error != nil {
		return fmt.Errorf("failed to update workspace: %v", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("workspace not found")
	}
	return nil
}

func (r *WorkspaceRepository) Delete(id int) error {
	result := r.db.Delete(&models.Workspace{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete workspace: %v", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("workspace not found")
	}
	return nil
}

func (r *WorkspaceRepository) AddUser(workspaceID, userID int, role string) error {
	membership := models.WorkspaceUser{WorkspaceID: workspaceID, UserID: userID, Role: role}
	if err := r.db.Create(&membership).Error; err != nil {
		return fmt.Errorf("failed to add user to workspace: %v", err)
	}
	return nil
}

func (r *WorkspaceRepository) RemoveUser(workspaceID, userID int) error {
	result := r.db.Where("workspace_id = ? AND user_id = ?", workspaceID, userID).Delete(&models.WorkspaceUser{})
	if result.Error != nil {
		return fmt.Errorf("failed to remove user from workspace: %v", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found in workspace")
	}
	return nil
}

func (r *WorkspaceRepository) GetUserRole(workspaceID, userID int) (string, error) {
	var membership models.WorkspaceUser
	err := r.db.Select("role").Where("workspace_id = ? AND user_id = ?", workspaceID, userID).First(&membership).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get user role: %v", err)
	}
	return membership.Role, nil
}

func (r *WorkspaceRepository) GetMembers(workspaceID int) ([]models.WorkspaceMember, error) {
	var members []models.WorkspaceMember
	err := r.db.Table("workspace_users AS wu").
		Select("wu.user_id, u.username, wu.role, wu.joined_at").
		Joins("INNER JOIN users AS u ON u.id = wu.user_id").
		Where("wu.workspace_id = ? AND u.status = ?", workspaceID, models.StatusActive).
		Order("wu.joined_at ASC, wu.user_id ASC").Scan(&members).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace members: %v", err)
	}
	return members, nil
}

func (r *WorkspaceRepository) Count() (int, error) {
	return r.count(nil, nil, "failed to count workspaces")
}

func (r *WorkspaceRepository) CountActive() (int, error) {
	return r.count("status = ?", "active", "failed to count active workspaces")
}

func (r *WorkspaceRepository) CountByStatus(status string) (int, error) {
	return r.count("status = ?", status, "failed to count workspaces by status")
}

func (r *WorkspaceRepository) count(query any, value any, message string) (int, error) {
	operation := r.db.Model(&models.Workspace{})
	if query != nil {
		operation = operation.Where(query, value)
	}
	var count int64
	if err := operation.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("%s: %v", message, err)
	}
	return int(count), nil
}
