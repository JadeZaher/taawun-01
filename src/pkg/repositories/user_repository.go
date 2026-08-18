package repositories

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"taawun/pkg/models"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *models.User) error {
	if err := r.db.Create(user).Error; err != nil {
		return fmt.Errorf("failed to create user: %v", err)
	}
	return nil
}

func (r *UserRepository) GetByID(id int) (*models.User, error) {
	return r.first("id = ?", id, "failed to get user")
}

func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	return r.first("email = ?", email, "failed to get user by email")
}

func (r *UserRepository) GetByUsername(username string) (*models.User, error) {
	return r.first("username = ?", username, "failed to get user by username")
}

func (r *UserRepository) first(query string, value any, message string) (*models.User, error) {
	var user models.User
	err := r.db.Where(query, value).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %v", message, err)
	}
	return &user, nil
}

func (r *UserRepository) GetAll() ([]*models.User, error) {
	var users []*models.User
	if err := r.db.Order("id DESC").Find(&users).Error; err != nil {
		return nil, fmt.Errorf("failed to get users: %v", err)
	}
	return users, nil
}

func (r *UserRepository) Update(user *models.User) error {
	return r.update(user, false)
}

// UpdateAndRotateSessions persists a password replacement and revokes older access tokens.
func (r *UserRepository) UpdateAndRotateSessions(user *models.User) error {
	return r.update(user, true)
}

func (r *UserRepository) update(user *models.User, rotateSessions bool) error {
	updates := map[string]any{
		"username":   user.Username,
		"email":      user.Email,
		"password":   user.Password,
		"role":       user.Role,
		"status":     user.Status,
		"updated_at": time.Now(),
	}
	if rotateSessions {
		updates["session_version"] = gorm.Expr("session_version + 1")
	}
	result := r.db.Model(&models.User{}).Where("id = ?", user.ID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update user: %v", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}
	if rotateSessions {
		user.SessionVersion++
	}
	return nil
}

func (r *UserRepository) Delete(id int) error {
	result := r.db.Delete(&models.User{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete user: %v", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *UserRepository) UpdateRole(id int, role string) error {
	return r.updateFields(id, map[string]any{"role": role, "updated_at": time.Now()}, "failed to update user role")
}

func (r *UserRepository) UpdateStatus(id int, status string) error {
	return r.updateFields(id, map[string]any{"status": status, "updated_at": time.Now()}, "failed to update user status")
}

func (r *UserRepository) updateFields(id int, fields map[string]any, message string) error {
	result := r.db.Model(&models.User{}).Where("id = ?", id).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("%s: %v", message, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *UserRepository) Count() (int, error) {
	return r.count(nil, nil, "failed to count users")
}

func (r *UserRepository) CountActive() (int, error) {
	return r.count("status = ?", "active", "failed to count active users")
}

func (r *UserRepository) CountByRole(role string) (int, error) {
	return r.count("role = ?", role, "failed to count users by role")
}

func (r *UserRepository) count(query any, value any, message string) (int, error) {
	operation := r.db.Model(&models.User{})
	if query != nil {
		operation = operation.Where(query, value)
	}
	var count int64
	if err := operation.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("%s: %v", message, err)
	}
	return int(count), nil
}

func (r *UserRepository) GetWorkspaceUsers(workspaceID int) ([]*models.User, error) {
	var users []*models.User
	err := r.db.Table("users AS u").Select("u.*").
		Joins("INNER JOIN workspace_users AS wu ON u.id = wu.user_id").
		Where("wu.workspace_id = ?", workspaceID).Scan(&users).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace users: %v", err)
	}
	return users, nil
}
