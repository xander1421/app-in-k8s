package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexprut/twitter-clone/pkg/models"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Migrate(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		username VARCHAR(50) UNIQUE NOT NULL,
		email VARCHAR(255) UNIQUE NOT NULL,
		display_name VARCHAR(100),
		bio TEXT,
		avatar_url TEXT,
		follower_count INT DEFAULT 0,
		following_count INT DEFAULT 0,
		is_verified BOOLEAN DEFAULT FALSE,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS follows (
		follower_id UUID NOT NULL,
		followee_id UUID NOT NULL,
		created_at TIMESTAMPTZ DEFAULT NOW(),
		PRIMARY KEY (follower_id, followee_id)
	);

	CREATE INDEX IF NOT EXISTS idx_follows_follower ON follows(follower_id);
	CREATE INDEX IF NOT EXISTS idx_follows_followee ON follows(followee_id);
	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	`

	_, err := r.pool.Exec(ctx, schema)
	return err
}

// User operations

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (id, username, email, display_name, bio, avatar_url, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.pool.Exec(ctx, query,
		user.ID, user.Username, user.Email, user.DisplayName, user.Bio, user.AvatarURL, user.CreatedAt)
	return err
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	query := `
		SELECT id, username, email, display_name, bio, avatar_url, follower_count, following_count, is_verified, created_at
		FROM users WHERE id = $1
	`
	var user models.User
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.Username, &user.Email, &user.DisplayName, &user.Bio,
		&user.AvatarURL, &user.FollowerCount, &user.FollowingCount, &user.IsVerified, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	query := `
		SELECT id, username, email, display_name, bio, avatar_url, follower_count, following_count, is_verified, created_at
		FROM users WHERE username = $1
	`
	var user models.User
	err := r.pool.QueryRow(ctx, query, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.DisplayName, &user.Bio,
		&user.AvatarURL, &user.FollowerCount, &user.FollowingCount, &user.IsVerified, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	query := `
		UPDATE users SET display_name = $2, bio = $3, avatar_url = $4
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, user.ID, user.DisplayName, user.Bio, user.AvatarURL)
	return err
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM users WHERE id = $1", id)
	return err
}

// Follow operations

func (r *UserRepository) Follow(ctx context.Context, followerID, followeeID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Insert follow relationship
	_, err = tx.Exec(ctx,
		"INSERT INTO follows (follower_id, followee_id, created_at) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
		followerID, followeeID, time.Now())
	if err != nil {
		return err
	}

	// Update follower count
	_, err = tx.Exec(ctx, "UPDATE users SET follower_count = follower_count + 1 WHERE id = $1", followeeID)
	if err != nil {
		return err
	}

	// Update following count
	_, err = tx.Exec(ctx, "UPDATE users SET following_count = following_count + 1 WHERE id = $1", followerID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *UserRepository) Unfollow(ctx context.Context, followerID, followeeID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Delete follow relationship
	result, err := tx.Exec(ctx,
		"DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2",
		followerID, followeeID)
	if err != nil {
		return err
	}

	// Only update counts if a row was actually deleted
	if result.RowsAffected() > 0 {
		_, err = tx.Exec(ctx, "UPDATE users SET follower_count = GREATEST(0, follower_count - 1) WHERE id = $1", followeeID)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, "UPDATE users SET following_count = GREATEST(0, following_count - 1) WHERE id = $1", followerID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *UserRepository) IsFollowing(ctx context.Context, followerID, followeeID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id = $1 AND followee_id = $2)",
		followerID, followeeID).Scan(&exists)
	return exists, err
}

func (r *UserRepository) GetFollowers(ctx context.Context, userID string, limit, offset int) ([]models.User, error) {
	query := `
		SELECT u.id, u.username, u.email, u.display_name, u.bio, u.avatar_url,
		       u.follower_count, u.following_count, u.is_verified, u.created_at
		FROM users u
		INNER JOIN follows f ON u.id = f.follower_id
		WHERE f.followee_id = $1
		ORDER BY f.created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(
			&user.ID, &user.Username, &user.Email, &user.DisplayName, &user.Bio,
			&user.AvatarURL, &user.FollowerCount, &user.FollowingCount, &user.IsVerified, &user.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (r *UserRepository) GetFollowing(ctx context.Context, userID string, limit, offset int) ([]models.User, error) {
	query := `
		SELECT u.id, u.username, u.email, u.display_name, u.bio, u.avatar_url,
		       u.follower_count, u.following_count, u.is_verified, u.created_at
		FROM users u
		INNER JOIN follows f ON u.id = f.followee_id
		WHERE f.follower_id = $1
		ORDER BY f.created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(
			&user.ID, &user.Username, &user.Email, &user.DisplayName, &user.Bio,
			&user.AvatarURL, &user.FollowerCount, &user.FollowingCount, &user.IsVerified, &user.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (r *UserRepository) GetFollowerIDs(ctx context.Context, userID string) ([]string, error) {
	query := `SELECT follower_id FROM follows WHERE followee_id = $1`
	rows, err := r.pool.Query(ctx, query, userID)
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

func (r *UserRepository) GetFollowerCount(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, "SELECT follower_count FROM users WHERE id = $1", userID).Scan(&count)
	return count, err
}

func (r *UserRepository) GetFollowingIDs(ctx context.Context, userID string) ([]string, error) {
	query := `SELECT followee_id FROM follows WHERE follower_id = $1`
	rows, err := r.pool.Query(ctx, query, userID)
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

// GetActiveFollowerIDs returns followers who were active in the last N days
func (r *UserRepository) GetActiveFollowerIDs(ctx context.Context, userID string, activeDays int) ([]string, error) {
	// In a real implementation, you'd track last_active timestamp
	// For now, return all followers
	return r.GetFollowerIDs(ctx, userID)
}

func (r *UserRepository) BatchGetUsers(ctx context.Context, ids []string) ([]models.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	query := `
		SELECT id, username, email, display_name, bio, avatar_url, follower_count, following_count, is_verified, created_at
		FROM users WHERE id = ANY($1)
	`
	rows, err := r.pool.Query(ctx, query, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(
			&user.ID, &user.Username, &user.Email, &user.DisplayName, &user.Bio,
			&user.AvatarURL, &user.FollowerCount, &user.FollowingCount, &user.IsVerified, &user.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}
