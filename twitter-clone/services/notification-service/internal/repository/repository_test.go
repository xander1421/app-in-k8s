package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/alexprut/twitter-clone/pkg/models"
	"github.com/alexprut/twitter-clone/pkg/testutil"
)

func setupNotificationRepo(t *testing.T) *NotificationRepository {
	pool := testutil.TestDB(t)

	repo := NewNotificationRepository(pool)
	if err := repo.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	testutil.CleanupTables(t, pool, "notifications")

	return repo
}

func TestNotificationRepository_Create(t *testing.T) {
	repo := setupNotificationRepo(t)
	ctx := context.Background()

	notif := &models.Notification{
		ID:        uuid.New().String(),
		UserID:    uuid.New().String(),
		Type:      "like",
		ActorID:   uuid.New().String(),
		TweetID:   uuid.New().String(),
		Data:      map[string]interface{}{"test": "data"},
		Read:      false,
		CreatedAt: time.Now(),
	}

	err := repo.Create(ctx, notif)
	if err != nil {
		t.Fatalf("failed to create notification: %v", err)
	}

	// Verify by getting
	notifications, err := repo.GetByUser(ctx, notif.UserID, 10, 0)
	if err != nil {
		t.Fatalf("failed to get notifications: %v", err)
	}

	if len(notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications))
	}

	if notifications[0].ID != notif.ID {
		t.Errorf("expected ID %s, got %s", notif.ID, notifications[0].ID)
	}
	if notifications[0].Type != "like" {
		t.Errorf("expected type 'like', got %s", notifications[0].Type)
	}
}

func TestNotificationRepository_GetByUser(t *testing.T) {
	repo := setupNotificationRepo(t)
	ctx := context.Background()

	userID := uuid.New().String()

	// Create multiple notifications
	for i := 0; i < 5; i++ {
		notif := &models.Notification{
			ID:        uuid.New().String(),
			UserID:    userID,
			Type:      "follow",
			ActorID:   uuid.New().String(),
			Read:      false,
			CreatedAt: time.Now().Add(time.Duration(i) * time.Minute),
		}
		if err := repo.Create(ctx, notif); err != nil {
			t.Fatalf("failed to create notification %d: %v", i, err)
		}
	}

	notifications, err := repo.GetByUser(ctx, userID, 10, 0)
	if err != nil {
		t.Fatalf("failed to get notifications: %v", err)
	}

	if len(notifications) != 5 {
		t.Errorf("expected 5 notifications, got %d", len(notifications))
	}

	// Should be ordered by created_at DESC
	for i := 1; i < len(notifications); i++ {
		if notifications[i].CreatedAt.After(notifications[i-1].CreatedAt) {
			t.Error("notifications not ordered by created_at DESC")
		}
	}
}

func TestNotificationRepository_GetUnreadByUser(t *testing.T) {
	repo := setupNotificationRepo(t)
	ctx := context.Background()

	userID := uuid.New().String()

	// Create mix of read and unread
	for i := 0; i < 5; i++ {
		notif := &models.Notification{
			ID:        uuid.New().String(),
			UserID:    userID,
			Type:      "mention",
			ActorID:   uuid.New().String(),
			Read:      i%2 == 0, // 0, 2, 4 are read; 1, 3 are unread
			CreatedAt: time.Now(),
		}
		if err := repo.Create(ctx, notif); err != nil {
			t.Fatalf("failed to create notification: %v", err)
		}
	}

	unread, err := repo.GetUnreadByUser(ctx, userID, 10, 0)
	if err != nil {
		t.Fatalf("failed to get unread: %v", err)
	}

	if len(unread) != 2 {
		t.Errorf("expected 2 unread, got %d", len(unread))
	}

	for _, n := range unread {
		if n.Read {
			t.Error("got read notification in unread list")
		}
	}
}

func TestNotificationRepository_GetUnreadCount(t *testing.T) {
	repo := setupNotificationRepo(t)
	ctx := context.Background()

	userID := uuid.New().String()

	// Create 3 unread, 2 read
	for i := 0; i < 5; i++ {
		notif := &models.Notification{
			ID:        uuid.New().String(),
			UserID:    userID,
			Type:      "retweet",
			ActorID:   uuid.New().String(),
			Read:      i >= 3,
			CreatedAt: time.Now(),
		}
		if err := repo.Create(ctx, notif); err != nil {
			t.Fatalf("failed to create notification: %v", err)
		}
	}

	count, err := repo.GetUnreadCount(ctx, userID)
	if err != nil {
		t.Fatalf("failed to get unread count: %v", err)
	}

	if count != 3 {
		t.Errorf("expected count 3, got %d", count)
	}
}

func TestNotificationRepository_MarkAsRead(t *testing.T) {
	repo := setupNotificationRepo(t)
	ctx := context.Background()

	userID := uuid.New().String()
	notifID := uuid.New().String()

	notif := &models.Notification{
		ID:        notifID,
		UserID:    userID,
		Type:      "like",
		ActorID:   uuid.New().String(),
		Read:      false,
		CreatedAt: time.Now(),
	}

	if err := repo.Create(ctx, notif); err != nil {
		t.Fatalf("failed to create notification: %v", err)
	}

	// Verify unread
	count, _ := repo.GetUnreadCount(ctx, userID)
	if count != 1 {
		t.Fatalf("expected 1 unread, got %d", count)
	}

	// Mark as read
	if err := repo.MarkAsRead(ctx, notifID, userID); err != nil {
		t.Fatalf("failed to mark as read: %v", err)
	}

	// Verify read
	count, _ = repo.GetUnreadCount(ctx, userID)
	if count != 0 {
		t.Errorf("expected 0 unread after marking, got %d", count)
	}
}

func TestNotificationRepository_MarkAllAsRead(t *testing.T) {
	repo := setupNotificationRepo(t)
	ctx := context.Background()

	userID := uuid.New().String()

	// Create 5 unread
	for i := 0; i < 5; i++ {
		notif := &models.Notification{
			ID:        uuid.New().String(),
			UserID:    userID,
			Type:      "follow",
			ActorID:   uuid.New().String(),
			Read:      false,
			CreatedAt: time.Now(),
		}
		if err := repo.Create(ctx, notif); err != nil {
			t.Fatalf("failed to create notification: %v", err)
		}
	}

	// Verify 5 unread
	count, _ := repo.GetUnreadCount(ctx, userID)
	if count != 5 {
		t.Fatalf("expected 5 unread, got %d", count)
	}

	// Mark all as read
	if err := repo.MarkAllAsRead(ctx, userID); err != nil {
		t.Fatalf("failed to mark all as read: %v", err)
	}

	// Verify 0 unread
	count, _ = repo.GetUnreadCount(ctx, userID)
	if count != 0 {
		t.Errorf("expected 0 unread after marking all, got %d", count)
	}
}

func TestNotificationRepository_Delete(t *testing.T) {
	repo := setupNotificationRepo(t)
	ctx := context.Background()

	userID := uuid.New().String()
	notifID := uuid.New().String()

	notif := &models.Notification{
		ID:        notifID,
		UserID:    userID,
		Type:      "mention",
		ActorID:   uuid.New().String(),
		Read:      false,
		CreatedAt: time.Now(),
	}

	if err := repo.Create(ctx, notif); err != nil {
		t.Fatalf("failed to create notification: %v", err)
	}

	// Delete
	if err := repo.Delete(ctx, notifID, userID); err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	// Verify deleted
	notifications, _ := repo.GetByUser(ctx, userID, 10, 0)
	if len(notifications) != 0 {
		t.Errorf("expected 0 notifications after delete, got %d", len(notifications))
	}
}

func TestNotificationRepository_DeleteOlderThan(t *testing.T) {
	repo := setupNotificationRepo(t)
	ctx := context.Background()

	userID := uuid.New().String()

	// Create old notifications (before cutoff)
	for i := 0; i < 3; i++ {
		notif := &models.Notification{
			ID:        uuid.New().String(),
			UserID:    userID,
			Type:      "like",
			ActorID:   uuid.New().String(),
			Read:      true,
			CreatedAt: time.Now().Add(-48 * time.Hour), // 2 days old
		}
		if err := repo.Create(ctx, notif); err != nil {
			t.Fatalf("failed to create old notification: %v", err)
		}
	}

	// Create new notifications
	for i := 0; i < 2; i++ {
		notif := &models.Notification{
			ID:        uuid.New().String(),
			UserID:    userID,
			Type:      "follow",
			ActorID:   uuid.New().String(),
			Read:      false,
			CreatedAt: time.Now(),
		}
		if err := repo.Create(ctx, notif); err != nil {
			t.Fatalf("failed to create new notification: %v", err)
		}
	}

	// Delete older than 1 day
	before := time.Now().Add(-24 * time.Hour)
	deleted, err := repo.DeleteOlderThan(ctx, before)
	if err != nil {
		t.Fatalf("failed to delete old: %v", err)
	}

	if deleted != 3 {
		t.Errorf("expected 3 deleted, got %d", deleted)
	}

	// Verify remaining
	notifications, _ := repo.GetByUser(ctx, userID, 10, 0)
	if len(notifications) != 2 {
		t.Errorf("expected 2 remaining, got %d", len(notifications))
	}
}

func TestNotificationRepository_Pagination(t *testing.T) {
	repo := setupNotificationRepo(t)
	ctx := context.Background()

	userID := uuid.New().String()

	// Create 10 notifications
	for i := 0; i < 10; i++ {
		notif := &models.Notification{
			ID:        uuid.New().String(),
			UserID:    userID,
			Type:      "like",
			ActorID:   uuid.New().String(),
			Read:      false,
			CreatedAt: time.Now().Add(time.Duration(i) * time.Second),
		}
		if err := repo.Create(ctx, notif); err != nil {
			t.Fatalf("failed to create: %v", err)
		}
	}

	// First page
	page1, _ := repo.GetByUser(ctx, userID, 5, 0)
	if len(page1) != 5 {
		t.Errorf("expected 5 in page 1, got %d", len(page1))
	}

	// Second page
	page2, _ := repo.GetByUser(ctx, userID, 5, 5)
	if len(page2) != 5 {
		t.Errorf("expected 5 in page 2, got %d", len(page2))
	}

	// No overlap
	for _, p1 := range page1 {
		for _, p2 := range page2 {
			if p1.ID == p2.ID {
				t.Error("page 1 and page 2 have overlapping notifications")
			}
		}
	}
}
