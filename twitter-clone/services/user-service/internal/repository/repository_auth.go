package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	
	"github.com/alexprut/twitter-clone/pkg/models"
)

// UserRepositoryAuth is the enhanced repository with authentication support
type UserRepositoryAuth struct {
	pool *pgxpool.Pool
}

// NewUserRepositoryAuth creates a new auth-enabled repository
func NewUserRepositoryAuth(pool *pgxpool.Pool) *UserRepositoryAuth {
	return &UserRepositoryAuth{pool: pool}
}

// MigrateAuth creates all auth-related tables
func (r *UserRepositoryAuth) MigrateAuth(ctx context.Context) error {
	migrations := []string{
		// Update users table with auth fields
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash VARCHAR(255)`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT true`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS last_active_at TIMESTAMPTZ`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NOW()`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN DEFAULT false`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS two_factor_enabled BOOLEAN DEFAULT false`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS locked_at TIMESTAMPTZ`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS failed_login_attempts INT DEFAULT 0`,
		
		// Create sessions table
		`CREATE TABLE IF NOT EXISTS sessions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			refresh_token TEXT UNIQUE NOT NULL,
			user_agent TEXT,
			ip VARCHAR(45),
			expires_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			last_used_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		
		// Create blocks table
		`CREATE TABLE IF NOT EXISTS blocks (
			blocker_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			blocked_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			PRIMARY KEY (blocker_id, blocked_id)
		)`,
		
		// Create mutes table
		`CREATE TABLE IF NOT EXISTS mutes (
			muter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			muted_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			PRIMARY KEY (muter_id, muted_id)
		)`,
		
		// Create user_settings table
		`CREATE TABLE IF NOT EXISTS user_settings (
			user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			notifications_enabled BOOLEAN DEFAULT true,
			email_notifications BOOLEAN DEFAULT true,
			push_notifications BOOLEAN DEFAULT false,
			private_account BOOLEAN DEFAULT false,
			show_activity_status BOOLEAN DEFAULT true,
			allow_dm_from_anyone BOOLEAN DEFAULT false,
			sensitive_content_filter BOOLEAN DEFAULT true,
			language VARCHAR(10) DEFAULT 'en',
			timezone VARCHAR(50) DEFAULT 'UTC',
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		
		// Create password_resets table
		`CREATE TABLE IF NOT EXISTS password_resets (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token VARCHAR(255) UNIQUE NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			used_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ DEFAULT NOW()
		)`,
		
		// Add indexes
		`CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_refresh_token ON sessions(refresh_token)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_blocks_blocker ON blocks(blocker_id)`,
		`CREATE INDEX IF NOT EXISTS idx_blocks_blocked ON blocks(blocked_id)`,
		`CREATE INDEX IF NOT EXISTS idx_mutes_muter ON mutes(muter_id)`,
		`CREATE INDEX IF NOT EXISTS idx_mutes_muted ON mutes(muted_id)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`CREATE INDEX IF NOT EXISTS idx_users_last_active ON users(last_active_at)`,
		`CREATE INDEX IF NOT EXISTS idx_password_resets_token ON password_resets(token)`,
		
		// Create updated_at trigger
		`CREATE OR REPLACE FUNCTION update_updated_at_column()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = NOW();
			RETURN NEW;
		END;
		$$ language 'plpgsql'`,
		
		`DROP TRIGGER IF EXISTS update_users_updated_at ON users`,
		`CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
		FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()`,
		
		`DROP TRIGGER IF EXISTS update_user_settings_updated_at ON user_settings`,
		`CREATE TRIGGER update_user_settings_updated_at BEFORE UPDATE ON user_settings
		FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()`,
	}

	for _, migration := range migrations {
		if _, err := r.pool.Exec(ctx, migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

// CreateUserWithPassword creates a new user with password
func (r *UserRepositoryAuth) CreateUserWithPassword(ctx context.Context, user *models.User, passwordHash string) error {
	query := `
		INSERT INTO users (
			id, username, email, password_hash, display_name, bio, 
			avatar_url, is_verified, is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`

	user.ID = uuid.New().String()
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now
	user.IsActive = true

	err := r.pool.QueryRow(ctx, query,
		user.ID,
		user.Username,
		user.Email,
		passwordHash,
		user.DisplayName,
		user.Bio,
		user.AvatarURL,
		user.IsVerified,
		user.IsActive,
		user.CreatedAt,
		user.UpdatedAt,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return err
	}

	// Create default user settings
	settingsQuery := `
		INSERT INTO user_settings (user_id) VALUES ($1)
	`
	_, err = r.pool.Exec(ctx, settingsQuery, user.ID)

	return err
}

// GetUserByUsernameOrEmail retrieves a user by username or email
func (r *UserRepositoryAuth) GetUserByUsernameOrEmail(ctx context.Context, identifier string) (*models.User, error) {
	query := `
		SELECT 
			id, username, email, password_hash, display_name, bio, avatar_url,
			follower_count, following_count, is_verified, is_active,
			last_active_at, last_login_at, created_at, updated_at
		FROM users
		WHERE (username = $1 OR email = $1) AND is_active = true
	`

	var user models.User
	var passwordHash sql.NullString
	var bio, avatarURL sql.NullString
	var lastActiveAt, lastLoginAt sql.NullTime

	err := r.pool.QueryRow(ctx, query, identifier).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&passwordHash,
		&user.DisplayName,
		&bio,
		&avatarURL,
		&user.FollowerCount,
		&user.FollowingCount,
		&user.IsVerified,
		&user.IsActive,
		&lastActiveAt,
		&lastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Handle nullable fields
	if passwordHash.Valid {
		user.PasswordHash = passwordHash.String
	}
	if bio.Valid {
		user.Bio = bio.String
	}
	if avatarURL.Valid {
		user.AvatarURL = avatarURL.String
	}
	if lastActiveAt.Valid {
		user.LastActiveAt = &lastActiveAt.Time
	}
	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}

	return &user, nil
}

// UpdateUserActivity updates last_active_at and last_login_at
func (r *UserRepositoryAuth) UpdateUserActivity(ctx context.Context, userID string, isLogin bool) error {
	var query string
	now := time.Now()

	if isLogin {
		query = `
			UPDATE users 
			SET last_active_at = $2, last_login_at = $2, failed_login_attempts = 0
			WHERE id = $1
		`
	} else {
		query = `
			UPDATE users 
			SET last_active_at = $2
			WHERE id = $1
		`
	}

	_, err := r.pool.Exec(ctx, query, userID, now)
	return err
}

// GetActiveFollowerIDs gets followers who were active recently
func (r *UserRepositoryAuth) GetActiveFollowerIDs(ctx context.Context, userID string, limit int, activityThreshold time.Duration) ([]string, error) {
	cutoff := time.Now().Add(-activityThreshold)
	
	query := `
		SELECT f.follower_id
		FROM follows f
		JOIN users u ON f.follower_id = u.id
		WHERE f.followee_id = $1
		AND u.is_active = true
		AND u.last_active_at > $2
		ORDER BY u.last_active_at DESC
		LIMIT $3
	`

	rows, err := r.pool.Query(ctx, query, userID, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var followerIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		followerIDs = append(followerIDs, id)
	}

	return followerIDs, nil
}

// Session management

// CreateSession creates a new session
func (r *UserRepositoryAuth) CreateSession(ctx context.Context, session *models.Session) error {
	session.ID = uuid.New().String()
	
	query := `
		INSERT INTO sessions (
			id, user_id, refresh_token, user_agent, ip, 
			expires_at, created_at, last_used_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.pool.Exec(ctx, query,
		session.ID,
		session.UserID,
		session.RefreshToken,
		session.UserAgent,
		session.IP,
		session.ExpiresAt,
		session.CreatedAt,
		session.LastUsedAt,
	)

	return err
}

// GetSessionByRefreshToken gets a session by refresh token
func (r *UserRepositoryAuth) GetSessionByRefreshToken(ctx context.Context, refreshToken string) (*models.Session, error) {
	query := `
		SELECT 
			id, user_id, refresh_token, user_agent, ip,
			expires_at, created_at, last_used_at
		FROM sessions
		WHERE refresh_token = $1
	`

	var session models.Session
	err := r.pool.QueryRow(ctx, query, refreshToken).Scan(
		&session.ID,
		&session.UserID,
		&session.RefreshToken,
		&session.UserAgent,
		&session.IP,
		&session.ExpiresAt,
		&session.CreatedAt,
		&session.LastUsedAt,
	)

	if err != nil {
		return nil, err
	}

	return &session, nil
}

// UpdateSession updates session last used time
func (r *UserRepositoryAuth) UpdateSession(ctx context.Context, sessionID string) error {
	query := `
		UPDATE sessions 
		SET last_used_at = NOW()
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query, sessionID)
	return err
}

// DeleteSession deletes a session
func (r *UserRepositoryAuth) DeleteSession(ctx context.Context, sessionID string) error {
	query := `DELETE FROM sessions WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, sessionID)
	return err
}

// DeleteSessionByRefreshToken deletes a session by refresh token
func (r *UserRepositoryAuth) DeleteSessionByRefreshToken(ctx context.Context, refreshToken string) error {
	query := `DELETE FROM sessions WHERE refresh_token = $1`
	_, err := r.pool.Exec(ctx, query, refreshToken)
	return err
}

// CleanupExpiredSessions removes expired sessions
func (r *UserRepositoryAuth) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	query := `DELETE FROM sessions WHERE expires_at < NOW()`
	result, err := r.pool.Exec(ctx, query)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// Blocking and Muting

// BlockUser blocks a user
func (r *UserRepositoryAuth) BlockUser(ctx context.Context, blockerID, blockedID string) error {
	query := `
		INSERT INTO blocks (blocker_id, blocked_id)
		VALUES ($1, $2)
		ON CONFLICT (blocker_id, blocked_id) DO NOTHING
	`
	_, err := r.pool.Exec(ctx, query, blockerID, blockedID)
	return err
}

// UnblockUser unblocks a user
func (r *UserRepositoryAuth) UnblockUser(ctx context.Context, blockerID, blockedID string) error {
	query := `DELETE FROM blocks WHERE blocker_id = $1 AND blocked_id = $2`
	_, err := r.pool.Exec(ctx, query, blockerID, blockedID)
	return err
}

// IsBlocked checks if a user is blocked
func (r *UserRepositoryAuth) IsBlocked(ctx context.Context, blockerID, blockedID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM blocks 
			WHERE blocker_id = $1 AND blocked_id = $2
		)
	`
	var exists bool
	err := r.pool.QueryRow(ctx, query, blockerID, blockedID).Scan(&exists)
	return exists, err
}

// GetBlockedUserIDs gets all blocked user IDs
func (r *UserRepositoryAuth) GetBlockedUserIDs(ctx context.Context, userID string) ([]string, error) {
	query := `SELECT blocked_id FROM blocks WHERE blocker_id = $1`
	
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blockedIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		blockedIDs = append(blockedIDs, id)
	}

	return blockedIDs, nil
}

// MuteUser mutes a user
func (r *UserRepositoryAuth) MuteUser(ctx context.Context, muterID, mutedID string) error {
	query := `
		INSERT INTO mutes (muter_id, muted_id)
		VALUES ($1, $2)
		ON CONFLICT (muter_id, muted_id) DO NOTHING
	`
	_, err := r.pool.Exec(ctx, query, muterID, mutedID)
	return err
}

// UnmuteUser unmutes a user
func (r *UserRepositoryAuth) UnmuteUser(ctx context.Context, muterID, mutedID string) error {
	query := `DELETE FROM mutes WHERE muter_id = $1 AND muted_id = $2`
	_, err := r.pool.Exec(ctx, query, muterID, mutedID)
	return err
}

// GetMutedUserIDs gets all muted user IDs
func (r *UserRepositoryAuth) GetMutedUserIDs(ctx context.Context, userID string) ([]string, error) {
	query := `SELECT muted_id FROM mutes WHERE muter_id = $1`
	
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mutedIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		mutedIDs = append(mutedIDs, id)
	}

	return mutedIDs, nil
}

// Validation helpers

// UsernameExists checks if a username is already taken
func (r *UserRepositoryAuth) UsernameExists(ctx context.Context, username string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`
	var exists bool
	err := r.pool.QueryRow(ctx, query, username).Scan(&exists)
	return exists, err
}

// EmailExists checks if an email is already registered
func (r *UserRepositoryAuth) EmailExists(ctx context.Context, email string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`
	var exists bool
	err := r.pool.QueryRow(ctx, query, email).Scan(&exists)
	return exists, err
}

// IncrementFailedLoginAttempts increments failed login counter
func (r *UserRepositoryAuth) IncrementFailedLoginAttempts(ctx context.Context, userID string) error {
	query := `
		UPDATE users 
		SET failed_login_attempts = failed_login_attempts + 1
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, userID)
	return err
}

// LockAccount locks an account after too many failed attempts
func (r *UserRepositoryAuth) LockAccount(ctx context.Context, userID string) error {
	query := `
		UPDATE users 
		SET locked_at = NOW()
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, userID)
	return err
}

// UnlockAccount unlocks a locked account
func (r *UserRepositoryAuth) UnlockAccount(ctx context.Context, userID string) error {
	query := `
		UPDATE users 
		SET locked_at = NULL, failed_login_attempts = 0
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, userID)
	return err
}

// GetSessionByToken gets a session by refresh token (alias for GetSessionByRefreshToken)
func (r *UserRepositoryAuth) GetSessionByToken(ctx context.Context, refreshToken string) (*models.Session, error) {
	return r.GetSessionByRefreshToken(ctx, refreshToken)
}

// GetUserByID gets a user by ID
func (r *UserRepositoryAuth) GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	query := `
		SELECT 
			id, username, email, password_hash, display_name, bio, avatar_url,
			follower_count, following_count, is_verified, is_active,
			last_active_at, last_login_at, created_at, updated_at
		FROM users
		WHERE id = $1 AND is_active = true
	`

	var user models.User
	var passwordHash sql.NullString
	var bio, avatarURL sql.NullString
	var lastActiveAt, lastLoginAt sql.NullTime

	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&passwordHash,
		&user.DisplayName,
		&bio,
		&avatarURL,
		&user.FollowerCount,
		&user.FollowingCount,
		&user.IsVerified,
		&user.IsActive,
		&lastActiveAt,
		&lastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Handle nullable fields
	if passwordHash.Valid {
		user.PasswordHash = passwordHash.String
	}
	if bio.Valid {
		user.Bio = bio.String
	}
	if avatarURL.Valid {
		user.AvatarURL = avatarURL.String
	}
	if lastActiveAt.Valid {
		user.LastActiveAt = &lastActiveAt.Time
	}
	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}

	return &user, nil
}

// UpdateUser updates user information
func (r *UserRepositoryAuth) UpdateUser(ctx context.Context, user *models.User) error {
	query := `
		UPDATE users 
		SET display_name = $2, bio = $3, avatar_url = $4, password_hash = $5,
			email_verified = $6, is_active = $7, last_active_at = $8, 
			last_login_at = $9, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, user.ID, user.DisplayName, user.Bio, 
		user.AvatarURL, user.PasswordHash, user.EmailVerified, user.IsActive,
		user.LastActiveAt, user.LastLoginAt)
	return err
}

// UpdateUserProfile updates user profile information only
func (r *UserRepositoryAuth) UpdateUserProfile(ctx context.Context, userID string, req *models.UpdateUserRequest) error {
	query := `
		UPDATE users 
		SET display_name = $2, bio = $3, avatar_url = $4, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, userID, req.DisplayName, req.Bio, req.AvatarURL)
	return err
}

// GetUserByEmail gets a user by email
func (r *UserRepositoryAuth) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT 
			id, username, email, password_hash, display_name, bio, avatar_url,
			follower_count, following_count, is_verified, is_active,
			last_active_at, last_login_at, created_at, updated_at
		FROM users
		WHERE email = $1 AND is_active = true
	`

	var user models.User
	var passwordHash sql.NullString
	var bio, avatarURL sql.NullString
	var lastActiveAt, lastLoginAt sql.NullTime

	err := r.pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&passwordHash,
		&user.DisplayName,
		&bio,
		&avatarURL,
		&user.FollowerCount,
		&user.FollowingCount,
		&user.IsVerified,
		&user.IsActive,
		&lastActiveAt,
		&lastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Handle nullable fields
	if passwordHash.Valid {
		user.PasswordHash = passwordHash.String
	}
	if bio.Valid {
		user.Bio = bio.String
	}
	if avatarURL.Valid {
		user.AvatarURL = avatarURL.String
	}
	if lastActiveAt.Valid {
		user.LastActiveAt = &lastActiveAt.Time
	}
	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}

	return &user, nil
}