package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	
	"github.com/alexprut/twitter-clone/pkg/models"
)

// MediaRepository handles media database operations
type MediaRepository struct {
	db *pgxpool.Pool
}

// NewMediaRepository creates a new media repository
func NewMediaRepository(db *pgxpool.Pool) *MediaRepository {
	return &MediaRepository{
		db: db,
	}
}

// Migrate creates the media tables
func (r *MediaRepository) Migrate(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS media (
			id VARCHAR(36) PRIMARY KEY,
			user_id VARCHAR(36) NOT NULL,
			type VARCHAR(20) NOT NULL,
			url TEXT NOT NULL,
			thumbnail_url TEXT,
			width INTEGER,
			height INTEGER,
			duration INTEGER,
			size BIGINT NOT NULL,
			mime_type VARCHAR(100) NOT NULL,
			processing_status VARCHAR(20) DEFAULT 'pending',
			metadata JSONB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		
		`CREATE INDEX IF NOT EXISTS idx_media_user_id ON media(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_media_created_at ON media(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_media_processing_status ON media(processing_status)`,
		
		// Add trigger for updated_at
		`CREATE OR REPLACE FUNCTION update_updated_at_column()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = CURRENT_TIMESTAMP;
			RETURN NEW;
		END;
		$$ language 'plpgsql'`,
		
		`DROP TRIGGER IF EXISTS update_media_updated_at ON media`,
		
		`CREATE TRIGGER update_media_updated_at BEFORE UPDATE ON media
		FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()`,
	}

	for _, query := range queries {
		if _, err := r.db.Exec(ctx, query); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}

	return nil
}

// CreateMedia creates a new media record
func (r *MediaRepository) CreateMedia(ctx context.Context, media *models.Media) error {
	metadata, err := json.Marshal(media.Metadata)
	if err != nil {
		metadata = []byte("{}")
	}

	query := `
		INSERT INTO media (
			id, user_id, type, url, thumbnail_url,
			width, height, duration, size, mime_type,
			processing_status, metadata, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err = r.db.Exec(ctx, query,
		media.ID,
		media.UserID,
		media.Type,
		media.URL,
		media.ThumbnailURL,
		media.Width,
		media.Height,
		media.Duration,
		media.Size,
		media.MimeType,
		media.ProcessingStatus,
		metadata,
		media.CreatedAt,
		media.UpdatedAt,
	)

	return err
}

// GetMedia gets a media record by ID
func (r *MediaRepository) GetMedia(ctx context.Context, mediaID string) (*models.Media, error) {
	query := `
		SELECT 
			id, user_id, type, url, thumbnail_url,
			width, height, duration, size, mime_type,
			processing_status, metadata, created_at, updated_at
		FROM media
		WHERE id = $1
	`

	var media models.Media
	var thumbnailURL sql.NullString
	var width, height, duration sql.NullInt32
	var metadataJSON []byte

	err := r.db.QueryRow(ctx, query, mediaID).Scan(
		&media.ID,
		&media.UserID,
		&media.Type,
		&media.URL,
		&thumbnailURL,
		&width,
		&height,
		&duration,
		&media.Size,
		&media.MimeType,
		&media.ProcessingStatus,
		&metadataJSON,
		&media.CreatedAt,
		&media.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Handle nullable fields
	if thumbnailURL.Valid {
		media.ThumbnailURL = thumbnailURL.String
	}
	if width.Valid {
		media.Width = int(width.Int32)
	}
	if height.Valid {
		media.Height = int(height.Int32)
	}
	if duration.Valid {
		media.Duration = int(duration.Int32)
	}

	// Unmarshal metadata
	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &media.Metadata)
	}

	return &media, nil
}

// GetMediaByUser gets all media for a user
func (r *MediaRepository) GetMediaByUser(ctx context.Context, userID string, limit, offset int) ([]*models.Media, error) {
	query := `
		SELECT 
			id, user_id, type, url, thumbnail_url,
			width, height, duration, size, mime_type,
			processing_status, metadata, created_at, updated_at
		FROM media
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mediaList []*models.Media

	for rows.Next() {
		var media models.Media
		var thumbnailURL sql.NullString
		var width, height, duration sql.NullInt32
		var metadataJSON []byte

		err := rows.Scan(
			&media.ID,
			&media.UserID,
			&media.Type,
			&media.URL,
			&thumbnailURL,
			&width,
			&height,
			&duration,
			&media.Size,
			&media.MimeType,
			&media.ProcessingStatus,
			&metadataJSON,
			&media.CreatedAt,
			&media.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Handle nullable fields
		if thumbnailURL.Valid {
			media.ThumbnailURL = thumbnailURL.String
		}
		if width.Valid {
			media.Width = int(width.Int32)
		}
		if height.Valid {
			media.Height = int(height.Int32)
		}
		if duration.Valid {
			media.Duration = int(duration.Int32)
		}

		// Unmarshal metadata
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &media.Metadata)
		}

		mediaList = append(mediaList, &media)
	}

	return mediaList, nil
}

// UpdateMedia updates a media record
func (r *MediaRepository) UpdateMedia(ctx context.Context, media *models.Media) error {
	metadata, err := json.Marshal(media.Metadata)
	if err != nil {
		metadata = []byte("{}")
	}

	query := `
		UPDATE media SET
			thumbnail_url = $2,
			width = $3,
			height = $4,
			duration = $5,
			processing_status = $6,
			metadata = $7,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	_, err = r.db.Exec(ctx, query,
		media.ID,
		media.ThumbnailURL,
		media.Width,
		media.Height,
		media.Duration,
		media.ProcessingStatus,
		metadata,
	)

	return err
}

// UpdateProcessingStatus updates the processing status of media
func (r *MediaRepository) UpdateProcessingStatus(ctx context.Context, mediaID string, status string) error {
	query := `
		UPDATE media 
		SET processing_status = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query, mediaID, status)
	return err
}

// DeleteMedia deletes a media record
func (r *MediaRepository) DeleteMedia(ctx context.Context, mediaID string) error {
	query := `DELETE FROM media WHERE id = $1`
	_, err := r.db.Exec(ctx, query, mediaID)
	return err
}

// GetMediaBatch gets multiple media records by IDs
func (r *MediaRepository) GetMediaBatch(ctx context.Context, mediaIDs []string) ([]*models.Media, error) {
	if len(mediaIDs) == 0 {
		return []*models.Media{}, nil
	}

	// Build placeholder string
	placeholders := make([]string, len(mediaIDs))
	args := make([]interface{}, len(mediaIDs))
	for i, id := range mediaIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT 
			id, user_id, type, url, thumbnail_url,
			width, height, duration, size, mime_type,
			processing_status, metadata, created_at, updated_at
		FROM media
		WHERE id IN (%s)
		ORDER BY created_at DESC
	`, strings.Join(placeholders, ","))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mediaList []*models.Media

	for rows.Next() {
		var media models.Media
		var thumbnailURL sql.NullString
		var width, height, duration sql.NullInt32
		var metadataJSON []byte

		err := rows.Scan(
			&media.ID,
			&media.UserID,
			&media.Type,
			&media.URL,
			&thumbnailURL,
			&width,
			&height,
			&duration,
			&media.Size,
			&media.MimeType,
			&media.ProcessingStatus,
			&metadataJSON,
			&media.CreatedAt,
			&media.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Handle nullable fields
		if thumbnailURL.Valid {
			media.ThumbnailURL = thumbnailURL.String
		}
		if width.Valid {
			media.Width = int(width.Int32)
		}
		if height.Valid {
			media.Height = int(height.Int32)
		}
		if duration.Valid {
			media.Duration = int(duration.Int32)
		}

		// Unmarshal metadata
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &media.Metadata)
		}

		mediaList = append(mediaList, &media)
	}

	return mediaList, nil
}

// GetPendingMedia gets media that needs processing
func (r *MediaRepository) GetPendingMedia(ctx context.Context, limit int) ([]*models.Media, error) {
	query := `
		SELECT 
			id, user_id, type, url, thumbnail_url,
			width, height, duration, size, mime_type,
			processing_status, metadata, created_at, updated_at
		FROM media
		WHERE processing_status = 'pending'
		ORDER BY created_at ASC
		LIMIT $1
	`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mediaList []*models.Media

	for rows.Next() {
		var media models.Media
		var thumbnailURL sql.NullString
		var width, height, duration sql.NullInt32
		var metadataJSON []byte

		err := rows.Scan(
			&media.ID,
			&media.UserID,
			&media.Type,
			&media.URL,
			&thumbnailURL,
			&width,
			&height,
			&duration,
			&media.Size,
			&media.MimeType,
			&media.ProcessingStatus,
			&metadataJSON,
			&media.CreatedAt,
			&media.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Handle nullable fields
		if thumbnailURL.Valid {
			media.ThumbnailURL = thumbnailURL.String
		}
		if width.Valid {
			media.Width = int(width.Int32)
		}
		if height.Valid {
			media.Height = int(height.Int32)
		}
		if duration.Valid {
			media.Duration = int(duration.Int32)
		}

		// Unmarshal metadata
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &media.Metadata)
		}

		mediaList = append(mediaList, &media)
	}

	return mediaList, nil
}

// CleanupOldMedia deletes media older than the specified duration
func (r *MediaRepository) CleanupOldMedia(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	
	query := `
		DELETE FROM media 
		WHERE created_at < $1 
		AND processing_status = 'completed'
	`
	
	result, err := r.db.Exec(ctx, query, cutoff)
	if err != nil {
		return 0, err
	}
	
	return result.RowsAffected(), nil
}