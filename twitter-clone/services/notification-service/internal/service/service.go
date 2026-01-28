package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/alexprut/twitter-clone/pkg/models"
	"github.com/alexprut/twitter-clone/services/notification-service/internal/repository"
)

type NotificationService struct {
	repo *repository.NotificationRepository
}

func NewNotificationService(repo *repository.NotificationRepository) *NotificationService {
	return &NotificationService{repo: repo}
}

func (s *NotificationService) CreateNotification(ctx context.Context, userID, notifType, actorID, tweetID string, data map[string]interface{}) (*models.Notification, error) {
	// Don't notify yourself
	if userID == actorID {
		return nil, nil
	}

	notif := &models.Notification{
		ID:        uuid.New().String(),
		UserID:    userID,
		Type:      notifType,
		ActorID:   actorID,
		TweetID:   tweetID,
		Data:      data,
		Read:      false,
		CreatedAt: time.Now(),
	}

	if err := s.repo.Create(ctx, notif); err != nil {
		return nil, fmt.Errorf("create notification: %w", err)
	}

	return notif, nil
}

func (s *NotificationService) GetNotifications(ctx context.Context, userID string, limit, offset int) ([]models.Notification, bool, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	notifications, err := s.repo.GetByUser(ctx, userID, limit+1, offset)
	if err != nil {
		return nil, false, fmt.Errorf("get notifications: %w", err)
	}

	hasMore := len(notifications) > limit
	if hasMore {
		notifications = notifications[:limit]
	}

	return notifications, hasMore, nil
}

func (s *NotificationService) GetUnreadNotifications(ctx context.Context, userID string, limit, offset int) ([]models.Notification, bool, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	notifications, err := s.repo.GetUnreadByUser(ctx, userID, limit+1, offset)
	if err != nil {
		return nil, false, fmt.Errorf("get unread notifications: %w", err)
	}

	hasMore := len(notifications) > limit
	if hasMore {
		notifications = notifications[:limit]
	}

	return notifications, hasMore, nil
}

func (s *NotificationService) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	return s.repo.GetUnreadCount(ctx, userID)
}

func (s *NotificationService) MarkAsRead(ctx context.Context, notifID, userID string) error {
	return s.repo.MarkAsRead(ctx, notifID, userID)
}

func (s *NotificationService) MarkAllAsRead(ctx context.Context, userID string) error {
	return s.repo.MarkAllAsRead(ctx, userID)
}

func (s *NotificationService) Delete(ctx context.Context, notifID, userID string) error {
	return s.repo.Delete(ctx, notifID, userID)
}

// CleanupOldNotifications removes notifications older than the specified duration
func (s *NotificationService) CleanupOldNotifications(ctx context.Context, maxAge time.Duration) (int64, error) {
	before := time.Now().Add(-maxAge)
	return s.repo.DeleteOlderThan(ctx, before)
}
