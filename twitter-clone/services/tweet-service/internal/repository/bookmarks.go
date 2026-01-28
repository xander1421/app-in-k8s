package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/alexprut/twitter-clone/pkg/models"
)

// CreateBookmarksTable creates the bookmarks table
func (r *TweetRepository) CreateBookmarksTable(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS bookmarks (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL,
		tweet_id UUID NOT NULL,
		created_at TIMESTAMPTZ DEFAULT NOW(),
		UNIQUE(user_id, tweet_id)
	);

	CREATE INDEX IF NOT EXISTS idx_bookmarks_user ON bookmarks(user_id);
	CREATE INDEX IF NOT EXISTS idx_bookmarks_tweet ON bookmarks(tweet_id);
	CREATE INDEX IF NOT EXISTS idx_bookmarks_created ON bookmarks(created_at DESC);
	`

	_, err := r.pool.Exec(ctx, schema)
	return err
}

// AddBookmark adds a tweet to user's bookmarks
func (r *TweetRepository) AddBookmark(ctx context.Context, bookmark *models.Bookmark) error {
	query := `
		INSERT INTO bookmarks (user_id, tweet_id, created_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, tweet_id) DO NOTHING
	`

	_, err := r.pool.Exec(ctx, query, bookmark.UserID, bookmark.TweetID, bookmark.CreatedAt)
	return err
}

// RemoveBookmark removes a tweet from user's bookmarks
func (r *TweetRepository) RemoveBookmark(ctx context.Context, userID, tweetID string) error {
	query := `DELETE FROM bookmarks WHERE user_id = $1 AND tweet_id = $2`
	_, err := r.pool.Exec(ctx, query, userID, tweetID)
	return err
}

// GetUserBookmarks returns paginated bookmarks for a user
func (r *TweetRepository) GetUserBookmarks(ctx context.Context, userID string, limit int, cursor string) ([]*models.Bookmark, error) {
	var query string
	var args []interface{}

	if cursor == "" {
		query = `
			SELECT id, user_id, tweet_id, created_at
			FROM bookmarks
			WHERE user_id = $1
			ORDER BY created_at DESC
			LIMIT $2
		`
		args = []interface{}{userID, limit}
	} else {
		query = `
			SELECT id, user_id, tweet_id, created_at
			FROM bookmarks
			WHERE user_id = $1 AND id < $2
			ORDER BY created_at DESC
			LIMIT $3
		`
		args = []interface{}{userID, cursor, limit}
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookmarks []*models.Bookmark
	for rows.Next() {
		var bookmark models.Bookmark
		if err := rows.Scan(&bookmark.ID, &bookmark.UserID, &bookmark.TweetID, &bookmark.CreatedAt); err != nil {
			return nil, err
		}
		bookmarks = append(bookmarks, &bookmark)
	}

	return bookmarks, nil
}

// IsBookmarked checks if a tweet is bookmarked by user
func (r *TweetRepository) IsBookmarked(ctx context.Context, userID, tweetID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM bookmarks WHERE user_id = $1 AND tweet_id = $2)`
	
	var exists bool
	err := r.pool.QueryRow(ctx, query, userID, tweetID).Scan(&exists)
	return exists, err
}

// GetBookmarkCount returns the number of bookmarks for a tweet
func (r *TweetRepository) GetBookmarkCount(ctx context.Context, tweetID string) (int, error) {
	query := `SELECT COUNT(*) FROM bookmarks WHERE tweet_id = $1`
	
	var count int
	err := r.pool.QueryRow(ctx, query, tweetID).Scan(&count)
	return count, err
}

// GetTweetsByIDs returns tweets by their IDs
func (r *TweetRepository) GetTweetsByIDs(ctx context.Context, ids []string) ([]*models.Tweet, error) {
	if len(ids) == 0 {
		return []*models.Tweet{}, nil
	}

	query := `
		SELECT id, author_id, content, media_ids, reply_to_id, retweet_of_id,
		       like_count, retweet_count, reply_count, created_at
		FROM tweets 
		WHERE id = ANY($1)
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tweets []*models.Tweet
	for rows.Next() {
		var tweet models.Tweet
		var replyToID, retweetOfID *string

		err := rows.Scan(
			&tweet.ID, &tweet.AuthorID, &tweet.Content, &tweet.MediaIDs,
			&replyToID, &retweetOfID,
			&tweet.LikeCount, &tweet.RetweetCount, &tweet.ReplyCount, &tweet.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if replyToID != nil {
			tweet.ReplyToID = *replyToID
		}
		if retweetOfID != nil {
			tweet.RetweetOfID = *retweetOfID
		}

		tweets = append(tweets, &tweet)
	}

	return tweets, nil
}

// GetMostBookmarkedTweets returns IDs of most bookmarked tweets
func (r *TweetRepository) GetMostBookmarkedTweets(ctx context.Context, limit int, since time.Time) ([]string, error) {
	query := `
		SELECT tweet_id, COUNT(*) as bookmark_count
		FROM bookmarks
		WHERE created_at > $1
		GROUP BY tweet_id
		ORDER BY bookmark_count DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tweetIDs []string
	for rows.Next() {
		var tweetID string
		var count int
		if err := rows.Scan(&tweetID, &count); err != nil {
			return nil, err
		}
		tweetIDs = append(tweetIDs, tweetID)
	}

	return tweetIDs, nil
}

// GetUserBookmarkStats returns bookmark statistics for a user
func (r *TweetRepository) GetUserBookmarkStats(ctx context.Context, userID string) (*models.BookmarkStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_bookmarks,
			MIN(created_at) as first_bookmark,
			MAX(created_at) as last_bookmark,
			COUNT(DISTINCT DATE(created_at)) as active_days
		FROM bookmarks
		WHERE user_id = $1
	`

	var stats models.BookmarkStats
	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&stats.TotalCount,
		&stats.MonthlyCount,
		&stats.WeeklyCount,
		&stats.UserID,
	)

	if err == pgx.ErrNoRows {
		return &models.BookmarkStats{}, nil
	}

	return &stats, err
}

// BatchAddBookmarks adds multiple bookmarks in a single transaction
func (r *TweetRepository) BatchAddBookmarks(ctx context.Context, bookmarks []*models.Bookmark) error {
	if len(bookmarks) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO bookmarks (user_id, tweet_id, created_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, tweet_id) DO NOTHING
	`

	batch := &pgx.Batch{}
	for _, bookmark := range bookmarks {
		batch.Queue(query, bookmark.UserID, bookmark.TweetID, bookmark.CreatedAt)
	}

	br := tx.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < len(bookmarks); i++ {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("failed to insert bookmark %d: %w", i, err)
		}
	}

	return tx.Commit(ctx)
}

// CleanupOldBookmarks removes bookmarks older than specified duration
func (r *TweetRepository) CleanupOldBookmarks(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	query := `DELETE FROM bookmarks WHERE created_at < $1`
	
	result, err := r.pool.Exec(ctx, query, cutoff)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected(), nil
}

// GetBookmarksByDateRange returns bookmarks within a date range
func (r *TweetRepository) GetBookmarksByDateRange(ctx context.Context, userID string, start, end time.Time) ([]*models.Bookmark, error) {
	query := `
		SELECT id, user_id, tweet_id, created_at
		FROM bookmarks
		WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, userID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookmarks []*models.Bookmark
	for rows.Next() {
		var bookmark models.Bookmark
		if err := rows.Scan(&bookmark.ID, &bookmark.UserID, &bookmark.TweetID, &bookmark.CreatedAt); err != nil {
			return nil, err
		}
		bookmarks = append(bookmarks, &bookmark)
	}

	return bookmarks, nil
}