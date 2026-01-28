package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/alexprut/twitter-clone/pkg/models"
)

// Password reset methods

// CreatePasswordReset creates a password reset token
func (r *UserRepositoryAuth) CreatePasswordReset(ctx context.Context, reset *models.PasswordReset) error {
	reset.ID = uuid.New().String()
	reset.CreatedAt = time.Now()

	query := `
		INSERT INTO password_resets (id, user_id, token, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := r.pool.Exec(ctx, query,
		reset.ID,
		reset.UserID,
		reset.Token,
		reset.ExpiresAt,
		reset.CreatedAt,
	)

	return err
}

// GetPasswordResetByToken retrieves a password reset by token
func (r *UserRepositoryAuth) GetPasswordResetByToken(ctx context.Context, token string) (*models.PasswordReset, error) {
	query := `
		SELECT id, user_id, token, expires_at, used_at, created_at
		FROM password_resets
		WHERE token = $1
	`

	var reset models.PasswordReset
	var usedAt sql.NullTime

	err := r.pool.QueryRow(ctx, query, token).Scan(
		&reset.ID,
		&reset.UserID,
		&reset.Token,
		&reset.ExpiresAt,
		&usedAt,
		&reset.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	if usedAt.Valid {
		reset.UsedAt = &usedAt.Time
	}

	return &reset, nil
}

// UpdatePasswordReset updates a password reset
func (r *UserRepositoryAuth) UpdatePasswordReset(ctx context.Context, reset *models.PasswordReset) error {
	query := `
		UPDATE password_resets 
		SET used_at = $2
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query, reset.ID, reset.UsedAt)
	return err
}

// Session management extensions

// DeleteAllUserSessions deletes all sessions for a user
func (r *UserRepositoryAuth) DeleteAllUserSessions(ctx context.Context, userID string) error {
	query := `DELETE FROM sessions WHERE user_id = $1`
	_, err := r.pool.Exec(ctx, query, userID)
	return err
}

// GetUserSessions gets all sessions for a user
func (r *UserRepositoryAuth) GetUserSessions(ctx context.Context, userID string) ([]*models.Session, error) {
	query := `
		SELECT 
			id, user_id, refresh_token, user_agent, ip,
			expires_at, created_at, last_used_at
		FROM sessions
		WHERE user_id = $1
		ORDER BY last_used_at DESC
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*models.Session
	for rows.Next() {
		var session models.Session
		var lastUsedAt sql.NullTime

		err := rows.Scan(
			&session.ID,
			&session.UserID,
			&session.RefreshToken,
			&session.UserAgent,
			&session.IP,
			&session.ExpiresAt,
			&session.CreatedAt,
			&lastUsedAt,
		)
		if err != nil {
			return nil, err
		}

		if lastUsedAt.Valid {
			session.LastUsedAt = &lastUsedAt.Time
		}

		sessions = append(sessions, &session)
	}

	return sessions, nil
}

// GetSessionByID gets a session by ID
func (r *UserRepositoryAuth) GetSessionByID(ctx context.Context, sessionID string) (*models.Session, error) {
	query := `
		SELECT 
			id, user_id, refresh_token, user_agent, ip,
			expires_at, created_at, last_used_at
		FROM sessions
		WHERE id = $1
	`

	var session models.Session
	var lastUsedAt sql.NullTime

	err := r.pool.QueryRow(ctx, query, sessionID).Scan(
		&session.ID,
		&session.UserID,
		&session.RefreshToken,
		&session.UserAgent,
		&session.IP,
		&session.ExpiresAt,
		&session.CreatedAt,
		&lastUsedAt,
	)

	if err != nil {
		return nil, err
	}

	if lastUsedAt.Valid {
		session.LastUsedAt = &lastUsedAt.Time
	}

	return &session, nil
}

// DeleteSessionByID deletes a session by ID
func (r *UserRepositoryAuth) DeleteSessionByID(ctx context.Context, sessionID string) error {
	query := `DELETE FROM sessions WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, sessionID)
	return err
}

// Email verification methods

// GetEmailVerificationByToken retrieves an email verification by token
func (r *UserRepositoryAuth) GetEmailVerificationByToken(ctx context.Context, token string) (*models.EmailVerification, error) {
	query := `
		SELECT id, user_id, email, token, expires_at, verified_at, created_at
		FROM email_verifications
		WHERE token = $1
	`

	var verification models.EmailVerification
	var verifiedAt sql.NullTime

	err := r.pool.QueryRow(ctx, query, token).Scan(
		&verification.ID,
		&verification.UserID,
		&verification.Email,
		&verification.Token,
		&verification.ExpiresAt,
		&verifiedAt,
		&verification.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	if verifiedAt.Valid {
		verification.VerifiedAt = &verifiedAt.Time
	}

	return &verification, nil
}

// UpdateEmailVerification updates an email verification
func (r *UserRepositoryAuth) UpdateEmailVerification(ctx context.Context, verification *models.EmailVerification) error {
	query := `
		UPDATE email_verifications 
		SET verified_at = $2
		WHERE id = $1
	`

	_, err := r.pool.Exec(ctx, query, verification.ID, verification.VerifiedAt)
	return err
}