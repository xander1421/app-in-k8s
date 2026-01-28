package service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	
	"github.com/alexprut/twitter-clone/pkg/models"
	"github.com/alexprut/twitter-clone/pkg/queue"
	"github.com/alexprut/twitter-clone/pkg/storage"
	"github.com/alexprut/twitter-clone/services/media-service/internal/repository"
)


// Job represents a processing job (placeholder for undefined Job)
type Job struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Payload  map[string]interface{} `json:"payload"`
	Priority string                 `json:"priority"`
}

// MediaServiceComplete is the complete implementation with database
type MediaServiceComplete struct {
	storage  *storage.MinIOClient
	queue    *queue.RabbitMQ
	repo     *repository.MediaRepository
	endpoint string
}

// NewMediaServiceComplete creates a new media service with database
func NewMediaServiceComplete(storage *storage.MinIOClient, queue *queue.RabbitMQ, repo *repository.MediaRepository, endpoint string) *MediaServiceComplete {
	return &MediaServiceComplete{
		storage:  storage,
		queue:    queue,
		repo:     repo,
		endpoint: endpoint,
	}
}

// Upload handles direct file upload with database persistence
func (s *MediaServiceComplete) Upload(ctx context.Context, uploaderID, contentType string, reader io.Reader, size int64) (*models.Media, error) {
	if !AllowedContentTypes[contentType] {
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}

	if size > MaxUploadSize {
		return nil, fmt.Errorf("file size %d exceeds maximum %d", size, MaxUploadSize)
	}

	// Generate unique media ID
	mediaID := uuid.New().String()
	ext := getExtension(contentType)
	objectName := fmt.Sprintf("%s/%s%s", uploaderID, mediaID, ext)

	// Buffer the content for analysis
	buf := &bytes.Buffer{}
	tee := io.TeeReader(reader, buf)

	// Analyze media (dimensions for images)
	var width, height int
	mediaType := getMediaType(contentType)
	
	if mediaType == "image" {
		img, _, err := image.DecodeConfig(tee)
		if err == nil {
			width = img.Width
			height = img.Height
		}
		// Reset reader
		reader = io.MultiReader(buf, reader)
	}

	// Upload to MinIO
	url, err := s.storage.Upload(ctx, objectName, contentType, reader, size)
	if err != nil {
		return nil, fmt.Errorf("upload to storage: %w", err)
	}

	// Create media record
	media := &models.Media{
		ID:               mediaID,
		UserID:           uploaderID,
		Type:             mediaType,
		URL:              url,
		Width:            width,
		Height:           height,
		Size:             size,
		MimeType:         contentType,
		ProcessingStatus: "pending",
		Metadata:         make(map[string]interface{}),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Save to database
	if err := s.repo.CreateMedia(ctx, media); err != nil {
		// Clean up uploaded file if database save fails
		s.storage.Delete(ctx, objectName)
		return nil, fmt.Errorf("save to database: %w", err)
	}

	// Queue processing job
	if s.queue != nil {
		if err := s.queue.PublishMediaProcess(ctx, mediaID, media.UserID, contentType); err != nil {
			// Log but don't fail upload
			fmt.Printf("Failed to queue processing job: %v\n", err)
		}
	}

	return media, nil
}

// GetPresignedURL generates a presigned URL for direct upload
func (s *MediaServiceComplete) GetPresignedURL(ctx context.Context, uploaderID, contentType string) (map[string]interface{}, error) {
	if !AllowedContentTypes[contentType] {
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}

	mediaID := uuid.New().String()
	ext := getExtension(contentType)
	objectName := fmt.Sprintf("%s/%s%s", uploaderID, mediaID, ext)

	url, err := s.storage.GetPresignedUploadURL(ctx, objectName, PresignedURLExpiry)
	if err != nil {
		return nil, fmt.Errorf("generate presigned URL: %w", err)
	}

	// Pre-create media record with pending status
	media := &models.Media{
		ID:               mediaID,
		UserID:           uploaderID,
		Type:             getMediaType(contentType),
		URL:              fmt.Sprintf("http://%s/%s/%s", s.endpoint, storage.BucketMedia, objectName),
		MimeType:         contentType,
		ProcessingStatus: "pending",
		Metadata:         make(map[string]interface{}),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := s.repo.CreateMedia(ctx, media); err != nil {
		return nil, fmt.Errorf("save to database: %w", err)
	}

	return map[string]interface{}{
		"upload_url": url,
		"media_id":   mediaID,
		"expires_at": time.Now().Add(PresignedURLExpiry),
	}, nil
}

// GetMedia retrieves media information from database
func (s *MediaServiceComplete) GetMedia(ctx context.Context, mediaID string) (*models.Media, error) {
	media, err := s.repo.GetMedia(ctx, mediaID)
	if err != nil {
		return nil, fmt.Errorf("get from database: %w", err)
	}
	
	return media, nil
}

// GetMediaBatch retrieves multiple media records
func (s *MediaServiceComplete) GetMediaBatch(ctx context.Context, mediaIDs []string) ([]*models.Media, error) {
	if len(mediaIDs) == 0 {
		return []*models.Media{}, nil
	}
	
	return s.repo.GetMediaBatch(ctx, mediaIDs)
}

// DeleteMedia deletes media and its database record
func (s *MediaServiceComplete) DeleteMedia(ctx context.Context, mediaID, userID string) error {
	// Get media to verify ownership
	media, err := s.repo.GetMedia(ctx, mediaID)
	if err != nil {
		return fmt.Errorf("media not found: %w", err)
	}

	if media.UserID != userID {
		return fmt.Errorf("unauthorized: user does not own this media")
	}

	// Delete from storage
	objectName := extractObjectName(media.URL)
	if err := s.storage.Delete(ctx, objectName); err != nil {
		// Log but continue with database deletion
		fmt.Printf("Failed to delete from storage: %v\n", err)
	}

	// Delete from database
	if err := s.repo.DeleteMedia(ctx, mediaID); err != nil {
		return fmt.Errorf("delete from database: %w", err)
	}

	return nil
}

// ProcessMedia processes uploaded media (thumbnails, metadata extraction)
func (s *MediaServiceComplete) ProcessMedia(ctx context.Context, mediaID string) error {
	media, err := s.repo.GetMedia(ctx, mediaID)
	if err != nil {
		return fmt.Errorf("media not found: %w", err)
	}

	// Update status to processing
	media.ProcessingStatus = "processing"
	if err := s.repo.UpdateMedia(ctx, media); err != nil {
		return err
	}

	// Process based on media type
	switch media.Type {
	case "image":
		if err := s.processImage(ctx, media); err != nil {
			media.ProcessingStatus = "failed"
			s.repo.UpdateMedia(ctx, media)
			return err
		}
	case "video":
		if err := s.processVideo(ctx, media); err != nil {
			media.ProcessingStatus = "failed"
			s.repo.UpdateMedia(ctx, media)
			return err
		}
	}

	// Update status to completed
	media.ProcessingStatus = "completed"
	return s.repo.UpdateMedia(ctx, media)
}

// processImage generates thumbnails for images
func (s *MediaServiceComplete) processImage(ctx context.Context, media *models.Media) error {
	// TODO: Implement image processing
	log.Printf("Would process image for media: %s", media.ID)
	media.ThumbnailURL = media.URL
	return nil
}

// processVideo extracts video metadata and generates thumbnail
func (s *MediaServiceComplete) processVideo(ctx context.Context, media *models.Media) error {
	// In production, use ffmpeg to:
	// 1. Extract video metadata (dimensions, duration, codec)
	// 2. Generate thumbnail from first frame
	// 3. Optionally transcode to web-friendly format
	
	// For now, just mark as processed
	media.Metadata["processed"] = true
	media.ThumbnailURL = media.URL // Placeholder
	
	return nil
}

// GetUserMedia gets all media for a user
func (s *MediaServiceComplete) GetUserMedia(ctx context.Context, userID string, limit, offset int) ([]*models.Media, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	
	return s.repo.GetMediaByUser(ctx, userID, limit, offset)
}

// CleanupOldMedia removes old processed media
func (s *MediaServiceComplete) CleanupOldMedia(ctx context.Context, olderThan time.Duration) error {
	deleted, err := s.repo.CleanupOldMedia(ctx, olderThan)
	if err != nil {
		return fmt.Errorf("cleanup database: %w", err)
	}
	
	log.Printf("Cleaned up %d old media records", deleted)
	return nil
}

// StartProcessor starts the background media processor
func (s *MediaServiceComplete) StartProcessor(ctx context.Context) {
	// TODO: Implement background processing
	log.Printf("Media processor started")
	<-ctx.Done()
	log.Printf("Media processor stopped")
}

// Helper functions


func getMediaType(contentType string) string {
	if strings.HasPrefix(contentType, "image/") {
		return "image"
	}
	if strings.HasPrefix(contentType, "video/") {
		return "video"
	}
	return "unknown"
}

func extractObjectName(url string) string {
	// Extract object name from URL
	parts := strings.Split(url, "/")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	return ""
}