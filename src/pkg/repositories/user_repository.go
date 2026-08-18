package repositories

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"taawun/pkg/models"
)

var ErrUserNotFound = errors.New("user not found")

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
	result := r.db.Model(&models.User{}).Where("id = ? AND status <> ?", user.ID, models.StatusDeleted).Updates(updates)
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

// DeactivateAndAnonymize revokes account authority while retaining referenced audit records.
func (r *UserRepository) DeactivateAndAnonymize(id int, username, email, password string, deletedAt time.Time) error {
	if id <= 0 || username == "" || email == "" || password == "" {
		return fmt.Errorf("invalid account deletion request")
	}

	hasTable := func(name string) bool { return r.db.Migrator().HasTable(name) }
	hasWorkspaceMemberships := hasTable("workspace_users")
	hasNotifications := hasTable("notifications")
	oauthTables := map[string]bool{
		"oauth_sessions":            hasTable("oauth_sessions"),
		"oauth_consents":            hasTable("oauth_consents"),
		"oauth_authorization_codes": hasTable("oauth_authorization_codes"),
		"oauth_access_tokens":       hasTable("oauth_access_tokens"),
		"oauth_refresh_tokens":      hasTable("oauth_refresh_tokens"),
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		var account models.User
		if err := tx.Select("id", "status").First(&account, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserNotFound
			}
			return fmt.Errorf("load account lifecycle state: %w", err)
		}
		if account.Status == models.StatusDeleted {
			return nil
		}

		result := tx.Model(&models.User{}).Where("id = ?", id).Updates(map[string]any{
			"username":        username,
			"email":           email,
			"password":        password,
			"status":          models.StatusDeleted,
			"session_version": gorm.Expr("session_version + 1"),
			"updated_at":      deletedAt.UTC(),
		})
		if result.Error != nil {
			return fmt.Errorf("anonymize account: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrUserNotFound
		}

		if hasWorkspaceMemberships {
			if err := tx.Where("user_id = ?", id).Delete(&models.WorkspaceUser{}).Error; err != nil {
				return fmt.Errorf("revoke workspace memberships: %w", err)
			}
		}
		if hasNotifications {
			if err := tx.Where("user_id = ?", id).Delete(&models.Notification{}).Error; err != nil {
				return fmt.Errorf("remove account notifications: %w", err)
			}
		}

		revokedAt := deletedAt.UTC().Unix()
		for _, table := range []string{"oauth_sessions", "oauth_consents", "oauth_access_tokens", "oauth_refresh_tokens"} {
			if !oauthTables[table] {
				continue
			}
			if err := tx.Table(table).Where("user_id = ? AND revoked_at IS NULL", id).Update("revoked_at", revokedAt).Error; err != nil {
				return fmt.Errorf("revoke account OAuth credentials: %w", err)
			}
		}
		if oauthTables["oauth_authorization_codes"] {
			if err := tx.Table("oauth_authorization_codes").Where("user_id = ? AND consumed_at IS NULL", id).Update("consumed_at", revokedAt).Error; err != nil {
				return fmt.Errorf("revoke account OAuth authorization codes: %w", err)
			}
		}
		return nil
	})
}

func (r *UserRepository) UpdateRole(id int, role string) error {
	return r.updateFields(id, map[string]any{"role": role, "updated_at": time.Now()}, "failed to update user role")
}

func (r *UserRepository) UpdateStatus(id int, status string) error {
	return r.updateFields(id, map[string]any{"status": status, "updated_at": time.Now()}, "failed to update user status")
}

func (r *UserRepository) updateFields(id int, fields map[string]any, message string) error {
	result := r.db.Model(&models.User{}).Where("id = ? AND status <> ?", id, models.StatusDeleted).Updates(fields)
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
