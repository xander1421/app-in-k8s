package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexprut/fileshare/internal/models"
)

type PostgresDB struct {
	pool *pgxpool.Pool
}

func NewPostgresDB(ctx context.Context, databaseURL string) (*PostgresDB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	config.MaxConns = 20
	config.MinConns = 5
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}

	db := &PostgresDB{pool: pool}
	if err := db.migrate(ctx); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

func (db *PostgresDB) migrate(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		username VARCHAR(255) UNIQUE NOT NULL,
		email VARCHAR(255) UNIQUE NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS files (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name VARCHAR(255) NOT NULL,
		size BIGINT NOT NULL,
		content_type VARCHAR(255),
		checksum VARCHAR(64),
		owner_id UUID REFERENCES users(id),
		path VARCHAR(512) NOT NULL,
		tags TEXT[],
		downloads BIGINT DEFAULT 0,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS share_links (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		file_id UUID REFERENCES files(id) ON DELETE CASCADE,
		token VARCHAR(64) UNIQUE NOT NULL,
		expires_at TIMESTAMP WITH TIME ZONE,
		max_uses INT,
		uses INT DEFAULT 0,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_files_owner ON files(owner_id);
	CREATE INDEX IF NOT EXISTS idx_files_created ON files(created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_share_links_token ON share_links(token);
	`

	_, err := db.pool.Exec(ctx, schema)
	return err
}

func (db *PostgresDB) Close() {
	db.pool.Close()
}

func (db *PostgresDB) Health(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

// File operations
func (db *PostgresDB) CreateFile(ctx context.Context, f *models.File) error {
	query := `
		INSERT INTO files (id, name, size, content_type, checksum, owner_id, path, tags, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := db.pool.Exec(ctx, query,
		f.ID, f.Name, f.Size, f.ContentType, f.Checksum, f.OwnerID, f.Path, f.Tags, f.CreatedAt, f.UpdatedAt)
	return err
}

func (db *PostgresDB) GetFile(ctx context.Context, id string) (*models.File, error) {
	query := `SELECT id, name, size, content_type, checksum, owner_id, path, tags, downloads, created_at, updated_at FROM files WHERE id = $1`
	var f models.File
	err := db.pool.QueryRow(ctx, query, id).Scan(
		&f.ID, &f.Name, &f.Size, &f.ContentType, &f.Checksum, &f.OwnerID, &f.Path, &f.Tags, &f.Downloads, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (db *PostgresDB) ListFiles(ctx context.Context, ownerID string, limit, offset int) ([]models.File, error) {
	query := `SELECT id, name, size, content_type, checksum, owner_id, path, tags, downloads, created_at, updated_at
	          FROM files WHERE owner_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := db.pool.Query(ctx, query, ownerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []models.File
	for rows.Next() {
		var f models.File
		if err := rows.Scan(&f.ID, &f.Name, &f.Size, &f.ContentType, &f.Checksum, &f.OwnerID, &f.Path, &f.Tags, &f.Downloads, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

func (db *PostgresDB) DeleteFile(ctx context.Context, id string) error {
	_, err := db.pool.Exec(ctx, "DELETE FROM files WHERE id = $1", id)
	return err
}

func (db *PostgresDB) IncrementDownloads(ctx context.Context, id string) error {
	_, err := db.pool.Exec(ctx, "UPDATE files SET downloads = downloads + 1 WHERE id = $1", id)
	return err
}

// Share link operations
func (db *PostgresDB) CreateShareLink(ctx context.Context, s *models.ShareLink) error {
	query := `INSERT INTO share_links (id, file_id, token, expires_at, max_uses, created_at) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := db.pool.Exec(ctx, query, s.ID, s.FileID, s.Token, s.ExpiresAt, s.MaxUses, s.CreatedAt)
	return err
}

func (db *PostgresDB) GetShareLinkByToken(ctx context.Context, token string) (*models.ShareLink, error) {
	query := `SELECT id, file_id, token, expires_at, max_uses, uses, created_at FROM share_links WHERE token = $1`
	var s models.ShareLink
	err := db.pool.QueryRow(ctx, query, token).Scan(&s.ID, &s.FileID, &s.Token, &s.ExpiresAt, &s.MaxUses, &s.Uses, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (db *PostgresDB) IncrementShareLinkUses(ctx context.Context, id string) error {
	_, err := db.pool.Exec(ctx, "UPDATE share_links SET uses = uses + 1 WHERE id = $1", id)
	return err
}

// User operations (simplified - no auth for demo)
func (db *PostgresDB) GetOrCreateUser(ctx context.Context, username, email string) (*models.User, error) {
	// Try to get existing user
	query := `SELECT id, username, email, created_at FROM users WHERE username = $1`
	var u models.User
	err := db.pool.QueryRow(ctx, query, username).Scan(&u.ID, &u.Username, &u.Email, &u.CreatedAt)
	if err == nil {
		return &u, nil
	}

	// Create new user
	insertQuery := `INSERT INTO users (username, email) VALUES ($1, $2)
	                RETURNING id, username, email, created_at`
	err = db.pool.QueryRow(ctx, insertQuery, username, email).Scan(&u.ID, &u.Username, &u.Email, &u.CreatedAt)
	return &u, err
}
