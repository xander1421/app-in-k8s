package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexprut/twitter-clone/pkg/models"
)

type TweetRepository struct {
	pool *pgxpool.Pool
}

func NewTweetRepository(pool *pgxpool.Pool) *TweetRepository {
	return &TweetRepository{pool: pool}
}

func (r *TweetRepository) Migrate(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS tweets (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		author_id UUID NOT NULL,
		content TEXT NOT NULL,
		media_ids UUID[],
		reply_to_id UUID,
		retweet_of_id UUID,
		like_count INT DEFAULT 0,
		retweet_count INT DEFAULT 0,
		reply_count INT DEFAULT 0,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS likes (
		user_id UUID NOT NULL,
		tweet_id UUID NOT NULL,
		created_at TIMESTAMPTZ DEFAULT NOW(),
		PRIMARY KEY (user_id, tweet_id)
	);

	CREATE TABLE IF NOT EXISTS retweets (
		user_id UUID NOT NULL,
		tweet_id UUID NOT NULL,
		created_at TIMESTAMPTZ DEFAULT NOW(),
		PRIMARY KEY (user_id, tweet_id)
	);

	CREATE INDEX IF NOT EXISTS idx_tweets_author ON tweets(author_id);
	CREATE INDEX IF NOT EXISTS idx_tweets_created ON tweets(created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_tweets_reply_to ON tweets(reply_to_id) WHERE reply_to_id IS NOT NULL;
	CREATE INDEX IF NOT EXISTS idx_likes_tweet ON likes(tweet_id);
	CREATE INDEX IF NOT EXISTS idx_retweets_tweet ON retweets(tweet_id);
	`

	_, err := r.pool.Exec(ctx, schema)
	return err
}

// Tweet operations

func (r *TweetRepository) Create(ctx context.Context, tweet *models.Tweet) error {
	query := `
		INSERT INTO tweets (id, author_id, content, media_ids, reply_to_id, retweet_of_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	var replyToID, retweetOfID *string
	if tweet.ReplyToID != "" {
		replyToID = &tweet.ReplyToID
	}
	if tweet.RetweetOfID != "" {
		retweetOfID = &tweet.RetweetOfID
	}

	_, err := r.pool.Exec(ctx, query,
		tweet.ID, tweet.AuthorID, tweet.Content, tweet.MediaIDs,
		replyToID, retweetOfID, tweet.CreatedAt)
	return err
}

func (r *TweetRepository) GetByID(ctx context.Context, id string) (*models.Tweet, error) {
	query := `
		SELECT id, author_id, content, media_ids, reply_to_id, retweet_of_id,
		       like_count, retweet_count, reply_count, created_at
		FROM tweets WHERE id = $1
	`
	var tweet models.Tweet
	var replyToID, retweetOfID *string

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&tweet.ID, &tweet.AuthorID, &tweet.Content, &tweet.MediaIDs,
		&replyToID, &retweetOfID,
		&tweet.LikeCount, &tweet.RetweetCount, &tweet.ReplyCount, &tweet.CreatedAt)
	if err != nil {
		return nil, err
	}

	if replyToID != nil {
		tweet.ReplyToID = *replyToID
	}
	if retweetOfID != nil {
		tweet.RetweetOfID = *retweetOfID
	}

	return &tweet, nil
}

func (r *TweetRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM tweets WHERE id = $1", id)
	return err
}

func (r *TweetRepository) GetByAuthor(ctx context.Context, authorID string, limit, offset int) ([]models.Tweet, error) {
	query := `
		SELECT id, author_id, content, media_ids, reply_to_id, retweet_of_id,
		       like_count, retweet_count, reply_count, created_at
		FROM tweets WHERE author_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	return r.queryTweets(ctx, query, authorID, limit, offset)
}

func (r *TweetRepository) GetReplies(ctx context.Context, tweetID string, limit, offset int) ([]models.Tweet, error) {
	query := `
		SELECT id, author_id, content, media_ids, reply_to_id, retweet_of_id,
		       like_count, retweet_count, reply_count, created_at
		FROM tweets WHERE reply_to_id = $1
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3
	`
	return r.queryTweets(ctx, query, tweetID, limit, offset)
}

func (r *TweetRepository) BatchGet(ctx context.Context, ids []string) ([]models.Tweet, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	query := `
		SELECT id, author_id, content, media_ids, reply_to_id, retweet_of_id,
		       like_count, retweet_count, reply_count, created_at
		FROM tweets WHERE id = ANY($1)
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTweets(rows)
}

func (r *TweetRepository) queryTweets(ctx context.Context, query string, args ...interface{}) ([]models.Tweet, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanTweets(rows)
}

func (r *TweetRepository) scanTweets(rows pgx.Rows) ([]models.Tweet, error) {
	var tweets []models.Tweet
	for rows.Next() {
		var tweet models.Tweet
		var replyToID, retweetOfID *string

		if err := rows.Scan(
			&tweet.ID, &tweet.AuthorID, &tweet.Content, &tweet.MediaIDs,
			&replyToID, &retweetOfID,
			&tweet.LikeCount, &tweet.RetweetCount, &tweet.ReplyCount, &tweet.CreatedAt); err != nil {
			return nil, err
		}

		if replyToID != nil {
			tweet.ReplyToID = *replyToID
		}
		if retweetOfID != nil {
			tweet.RetweetOfID = *retweetOfID
		}

		tweets = append(tweets, tweet)
	}
	return tweets, nil
}

// Like operations

func (r *TweetRepository) Like(ctx context.Context, userID, tweetID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Insert like
	_, err = tx.Exec(ctx,
		"INSERT INTO likes (user_id, tweet_id, created_at) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
		userID, tweetID, time.Now())
	if err != nil {
		return err
	}

	// Update like count
	_, err = tx.Exec(ctx, "UPDATE tweets SET like_count = like_count + 1 WHERE id = $1", tweetID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *TweetRepository) Unlike(ctx context.Context, userID, tweetID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Delete like
	result, err := tx.Exec(ctx, "DELETE FROM likes WHERE user_id = $1 AND tweet_id = $2", userID, tweetID)
	if err != nil {
		return err
	}

	// Only update count if a row was deleted
	if result.RowsAffected() > 0 {
		_, err = tx.Exec(ctx, "UPDATE tweets SET like_count = GREATEST(0, like_count - 1) WHERE id = $1", tweetID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *TweetRepository) IsLiked(ctx context.Context, userID, tweetID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM likes WHERE user_id = $1 AND tweet_id = $2)",
		userID, tweetID).Scan(&exists)
	return exists, err
}

func (r *TweetRepository) GetLikedTweetIDs(ctx context.Context, userID string, limit, offset int) ([]string, error) {
	query := `SELECT tweet_id FROM likes WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// Retweet operations

func (r *TweetRepository) Retweet(ctx context.Context, userID, tweetID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Insert retweet record
	_, err = tx.Exec(ctx,
		"INSERT INTO retweets (user_id, tweet_id, created_at) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
		userID, tweetID, time.Now())
	if err != nil {
		return err
	}

	// Update retweet count
	_, err = tx.Exec(ctx, "UPDATE tweets SET retweet_count = retweet_count + 1 WHERE id = $1", tweetID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *TweetRepository) Unretweet(ctx context.Context, userID, tweetID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	result, err := tx.Exec(ctx, "DELETE FROM retweets WHERE user_id = $1 AND tweet_id = $2", userID, tweetID)
	if err != nil {
		return err
	}

	if result.RowsAffected() > 0 {
		_, err = tx.Exec(ctx, "UPDATE tweets SET retweet_count = GREATEST(0, retweet_count - 1) WHERE id = $1", tweetID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *TweetRepository) IsRetweeted(ctx context.Context, userID, tweetID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM retweets WHERE user_id = $1 AND tweet_id = $2)",
		userID, tweetID).Scan(&exists)
	return exists, err
}

// IncrementReplyCount increments the reply count for a tweet
func (r *TweetRepository) IncrementReplyCount(ctx context.Context, tweetID string) error {
	_, err := r.pool.Exec(ctx, "UPDATE tweets SET reply_count = reply_count + 1 WHERE id = $1", tweetID)
	return err
}
