package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexprut/twitter-clone/pkg/models"
)

type NotificationRepository struct {
	pool *pgxpool.Pool
}

func NewNotificationRepository(pool *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{pool: pool}
}

func (r *NotificationRepository) Migrate(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS notifications (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL,
		type VARCHAR(50) NOT NULL,
		actor_id UUID NOT NULL,
		tweet_id UUID,
		data JSONB,
		read BOOLEAN DEFAULT FALSE,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications(user_id, created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_notifications_unread ON notifications(user_id, read) WHERE read = FALSE;
	`

	_, err := r.pool.Exec(ctx, schema)
	return err
}

func (r *NotificationRepository) Create(ctx context.Context, notif *models.Notification) error {
	query := `
		INSERT INTO notifications (id, user_id, type, actor_id, tweet_id, data, read, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	var tweetID *string
	if notif.TweetID != "" {
		tweetID = &notif.TweetID
	}

	_, err := r.pool.Exec(ctx, query,
		notif.ID, notif.UserID, notif.Type, notif.ActorID, tweetID, notif.Data, notif.Read, notif.CreatedAt)
	return err
}

func (r *NotificationRepository) GetByUser(ctx context.Context, userID string, limit, offset int) ([]models.Notification, error) {
	query := `
		SELECT id, user_id, type, actor_id, tweet_id, data, read, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []models.Notification
	for rows.Next() {
		var notif models.Notification
		var tweetID *string

		if err := rows.Scan(&notif.ID, &notif.UserID, &notif.Type, &notif.ActorID, &tweetID, &notif.Data, &notif.Read, &notif.CreatedAt); err != nil {
			return nil, err
		}

		if tweetID != nil {
			notif.TweetID = *tweetID
		}

		notifications = append(notifications, notif)
	}

	return notifications, nil
}

func (r *NotificationRepository) GetUnreadByUser(ctx context.Context, userID string, limit, offset int) ([]models.Notification, error) {
	query := `
		SELECT id, user_id, type, actor_id, tweet_id, data, read, created_at
		FROM notifications
		WHERE user_id = $1 AND read = FALSE
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []models.Notification
	for rows.Next() {
		var notif models.Notification
		var tweetID *string

		if err := rows.Scan(&notif.ID, &notif.UserID, &notif.Type, &notif.ActorID, &tweetID, &notif.Data, &notif.Read, &notif.CreatedAt); err != nil {
			return nil, err
		}

		if tweetID != nil {
			notif.TweetID = *tweetID
		}

		notifications = append(notifications, notif)
	}

	return notifications, nil
}

func (r *NotificationRepository) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read = FALSE",
		userID).Scan(&count)
	return count, err
}

func (r *NotificationRepository) MarkAsRead(ctx context.Context, notifID, userID string) error {
	_, err := r.pool.Exec(ctx,
		"UPDATE notifications SET read = TRUE WHERE id = $1 AND user_id = $2",
		notifID, userID)
	return err
}

func (r *NotificationRepository) MarkAllAsRead(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx,
		"UPDATE notifications SET read = TRUE WHERE user_id = $1 AND read = FALSE",
		userID)
	return err
}

func (r *NotificationRepository) Delete(ctx context.Context, notifID, userID string) error {
	_, err := r.pool.Exec(ctx,
		"DELETE FROM notifications WHERE id = $1 AND user_id = $2",
		notifID, userID)
	return err
}

func (r *NotificationRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	result, err := r.pool.Exec(ctx,
		"DELETE FROM notifications WHERE created_at < $1",
		before)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
