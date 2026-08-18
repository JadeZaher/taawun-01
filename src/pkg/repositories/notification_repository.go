package repositories

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"taawun/pkg/models"
)

type NotificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) Create(notification *models.Notification) error {
	if err := r.db.Create(notification).Error; err != nil {
		return fmt.Errorf("failed to create notification: %v", err)
	}
	return nil
}

func (r *NotificationRepository) GetByID(id int) (*models.Notification, error) {
	var notification models.Notification
	err := r.db.First(&notification, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get notification: %v", err)
	}
	return &notification, nil
}

func (r *NotificationRepository) GetByUser(userID int) ([]*models.Notification, error) {
	return r.byUser(userID, false)
}

func (r *NotificationRepository) GetUnreadByUser(userID int) ([]*models.Notification, error) {
	return r.byUser(userID, true)
}

func (r *NotificationRepository) byUser(userID int, unreadOnly bool) ([]*models.Notification, error) {
	operation := r.db.Where("user_id = ?", userID)
	message := "failed to get notifications"
	if unreadOnly {
		operation = operation.Where("read = ?", false)
		message = "failed to get unread notifications"
	}
	var notifications []*models.Notification
	if err := operation.Order("created_at DESC").Find(&notifications).Error; err != nil {
		return nil, fmt.Errorf("%s: %v", message, err)
	}
	return notifications, nil
}

func (r *NotificationRepository) MarkAsReadForUser(id, userID int) error {
	result := r.db.Model(&models.Notification{}).Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]any{"read": true, "updated_at": time.Now()})
	if result.Error != nil {
		return fmt.Errorf("failed to mark notification as read: %v", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("notification not found")
	}
	return nil
}

func (r *NotificationRepository) MarkAllAsRead(userID int) error {
	if err := r.db.Model(&models.Notification{}).Where("user_id = ? AND read = ?", userID, false).
		Updates(map[string]any{"read": true, "updated_at": time.Now()}).Error; err != nil {
		return fmt.Errorf("failed to mark all notifications as read: %v", err)
	}
	return nil
}

func (r *NotificationRepository) Delete(id int) error {
	result := r.db.Delete(&models.Notification{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete notification: %v", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("notification not found")
	}
	return nil
}

func (r *NotificationRepository) Count() (int, error) {
	return r.count(nil)
}

func (r *NotificationRepository) CountUnreadByUser(userID int) (int, error) {
	return r.count(userID)
}

func (r *NotificationRepository) count(userID any) (int, error) {
	operation := r.db.Model(&models.Notification{})
	message := "failed to count notifications"
	if userID != nil {
		operation = operation.Where("user_id = ? AND read = ?", userID, false)
		message = "failed to count unread notifications"
	}
	var count int64
	if err := operation.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("%s: %v", message, err)
	}
	return int(count), nil
}
