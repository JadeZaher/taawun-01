package repositories

import (
	"database/sql"
	"fmt"
	"time"

	"taawun/pkg/models"
)

type NotificationRepository struct {
	db *sql.DB
}

func NewNotificationRepository(db *sql.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) Create(notification *models.Notification) error {
	query := `INSERT INTO notifications (user_id, type, title, message, read) 
		VALUES (?, ?, ?, ?, ?) RETURNING id, created_at, updated_at`
	
	err := r.db.QueryRow(query, notification.UserID, notification.Type, notification.Title, notification.Message, notification.Read).
		Scan(&notification.ID, &notification.CreatedAt, &notification.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create notification: %v", err)
	}
	return nil
}

func (r *NotificationRepository) GetByID(id int) (*models.Notification, error) {
	query := `SELECT id, user_id, type, title, message, read, created_at, updated_at 
		FROM notifications WHERE id = ?`
	
	var notification models.Notification
	err := r.db.QueryRow(query, id).Scan(
		&notification.ID, &notification.UserID, &notification.Type,
		&notification.Title, &notification.Message, &notification.Read,
		&notification.CreatedAt, &notification.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get notification: %v", err)
	}
	return &notification, nil
}

func (r *NotificationRepository) GetByUser(userID int) ([]*models.Notification, error) {
	query := `SELECT id, user_id, type, title, message, read, created_at, updated_at 
		FROM notifications WHERE user_id = ? ORDER BY created_at DESC`
	
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get notifications: %v", err)
	}
	defer rows.Close()

	var notifications []*models.Notification
	for rows.Next() {
		var notification models.Notification
		err := rows.Scan(
			&notification.ID, &notification.UserID, &notification.Type,
			&notification.Title, &notification.Message, &notification.Read,
			&notification.CreatedAt, &notification.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan notification: %v", err)
		}
		notifications = append(notifications, &notification)
	}
	return notifications, nil
}

func (r *NotificationRepository) GetUnreadByUser(userID int) ([]*models.Notification, error) {
	query := `SELECT id, user_id, type, title, message, read, created_at, updated_at 
		FROM notifications WHERE user_id = ? AND read = 0 ORDER BY created_at DESC`
	
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get unread notifications: %v", err)
	}
	defer rows.Close()

	var notifications []*models.Notification
	for rows.Next() {
		var notification models.Notification
		err := rows.Scan(
			&notification.ID, &notification.UserID, &notification.Type,
			&notification.Title, &notification.Message, &notification.Read,
			&notification.CreatedAt, &notification.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan notification: %v", err)
		}
		notifications = append(notifications, &notification)
	}
	return notifications, nil
}

func (r *NotificationRepository) MarkAsRead(id int) error {
	query := `UPDATE notifications SET read = 1, updated_at = ? WHERE id = ?`
	
	result, err := r.db.Exec(query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to mark notification as read: %v", err)
	}
	
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}
	if rows == 0 {
		return fmt.Errorf("notification not found")
	}
	return nil
}

func (r *NotificationRepository) MarkAllAsRead(userID int) error {
	query := `UPDATE notifications SET read = 1, updated_at = ? WHERE user_id = ? AND read = 0`
	
	_, err := r.db.Exec(query, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to mark all notifications as read: %v", err)
	}
	return nil
}

func (r *NotificationRepository) Delete(id int) error {
	query := `DELETE FROM notifications WHERE id = ?`
	
	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete notification: %v", err)
	}
	
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %v", err)
	}
	if rows == 0 {
		return fmt.Errorf("notification not found")
	}
	return nil
}

func (r *NotificationRepository) Count() (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM notifications").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count notifications: %v", err)
	}
	return count, nil
}

func (r *NotificationRepository) CountUnreadByUser(userID int) (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = ? AND read = 0", userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count unread notifications: %v", err)
	}
	return count, nil
}