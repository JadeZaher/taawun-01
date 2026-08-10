package services

import (
	"fmt"
	"taawun/pkg/models"
	"taawun/pkg/repositories"
)

type NotificationService struct {
	notificationRepo *repositories.NotificationRepository
	userRepo         *repositories.UserRepository
}

func NewNotificationService(notificationRepo *repositories.NotificationRepository, userRepo *repositories.UserRepository) *NotificationService {
	return &NotificationService{
		notificationRepo: notificationRepo,
		userRepo:         userRepo,
	}
}

func (s *NotificationService) CreateNotification(req *models.CreateNotificationRequest) (*models.Notification, error) {
	// Check if user exists
	user, err := s.userRepo.GetByID(req.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %v", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	notification := &models.Notification{
		UserID:  req.UserID,
		Type:    req.Type,
		Title:   req.Title,
		Message: req.Message,
		Read:    false,
	}

	if err := s.notificationRepo.Create(notification); err != nil {
		return nil, fmt.Errorf("failed to create notification: %v", err)
	}

	return notification, nil
}

func (s *NotificationService) GetUserNotifications(userID int) ([]*models.Notification, error) {
	notifications, err := s.notificationRepo.GetByUser(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get notifications: %v", err)
	}
	return notifications, nil
}

func (s *NotificationService) GetUnreadNotifications(userID int) ([]*models.Notification, error) {
	notifications, err := s.notificationRepo.GetUnreadByUser(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get unread notifications: %v", err)
	}
	return notifications, nil
}

func (s *NotificationService) MarkAsRead(notificationID int) error {
	if err := s.notificationRepo.MarkAsRead(notificationID); err != nil {
		return fmt.Errorf("failed to mark notification as read: %v", err)
	}
	return nil
}

func (s *NotificationService) MarkAllAsRead(userID int) error {
	if err := s.notificationRepo.MarkAllAsRead(userID); err != nil {
		return fmt.Errorf("failed to mark all notifications as read: %v", err)
	}
	return nil
}

func (s *NotificationService) GetUnreadCount(userID int) (int, error) {
	count, err := s.notificationRepo.CountUnreadByUser(userID)
	if err != nil {
		return 0, fmt.Errorf("failed to get unread count: %v", err)
	}
	return count, nil
}