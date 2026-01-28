package service

import (
	"context"
	"fmt"
	"io"
	"path"
	"time"

	"github.com/google/uuid"

	"github.com/alexprut/twitter-clone/pkg/models"
	"github.com/alexprut/twitter-clone/pkg/queue"
	"github.com/alexprut/twitter-clone/pkg/storage"
)

const (
	MaxUploadSize     = 10 * 1024 * 1024 // 10MB
	PresignedURLExpiry = 15 * time.Minute
)

var AllowedContentTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
	"video/mp4":  true,
	"video/webm": true,
}

type MediaService struct {
	storage  *storage.MinIOClient
	queue    *queue.RabbitMQ
	endpoint string
}

func NewMediaService(storage *storage.MinIOClient, queue *queue.RabbitMQ, endpoint string) *MediaService {
	return &MediaService{
		storage:  storage,
		queue:    queue,
		endpoint: endpoint,
	}
}

// Upload handles direct file upload
func (s *MediaService) Upload(ctx context.Context, uploaderID, contentType string, reader io.Reader, size int64) (*models.Media, error) {
	if !AllowedContentTypes[contentType] {
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}

	if size > MaxUploadSize {
		return nil, fmt.Errorf("file too large: max size is %d bytes", MaxUploadSize)
	}

	mediaID := uuid.New().String()
	ext := getExtension(contentType)
	objectName := fmt.Sprintf("%s/%s%s", uploaderID, mediaID, ext)

	_, err := s.storage.Upload(ctx, objectName, contentType, reader, size)
	if err != nil {
		return nil, fmt.Errorf("upload: %w", err)
	}

	media := &models.Media{
		ID:          mediaID,
		UploaderID:  uploaderID,
		URL:         s.storage.GetPublicURL(s.endpoint, objectName),
		ContentType: contentType,
		Size:        size,
		CreatedAt:   time.Now(),
	}

	// Queue media processing job
	if s.queue != nil {
		s.queue.PublishMediaProcess(ctx, mediaID, uploaderID, contentType)
	}

	return media, nil
}

// GetPresignedUploadURL returns a presigned URL for direct client upload
func (s *MediaService) GetPresignedUploadURL(ctx context.Context, uploaderID, contentType string) (string, string, error) {
	if !AllowedContentTypes[contentType] {
		return "", "", fmt.Errorf("unsupported content type: %s", contentType)
	}

	mediaID := uuid.New().String()
	ext := getExtension(contentType)
	objectName := fmt.Sprintf("%s/%s%s", uploaderID, mediaID, ext)

	uploadURL, err := s.storage.GetPresignedUploadURL(ctx, objectName, PresignedURLExpiry)
	if err != nil {
		return "", "", fmt.Errorf("get presigned URL: %w", err)
	}

	return uploadURL, mediaID, nil
}

// GetMedia returns media metadata
func (s *MediaService) GetMedia(ctx context.Context, mediaID, uploaderID string) (*models.Media, error) {
	// In a real implementation, we'd store media metadata in a database
	// For now, we'll just check if the object exists
	prefix := fmt.Sprintf("%s/%s", uploaderID, mediaID)
	objects, err := s.storage.ListObjects(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("list objects: %w", err)
	}

	if len(objects) == 0 {
		return nil, fmt.Errorf("media not found")
	}

	obj := objects[0]
	return &models.Media{
		ID:          mediaID,
		UploaderID:  uploaderID,
		URL:         s.storage.GetPublicURL(s.endpoint, obj.Key),
		ContentType: obj.ContentType,
		Size:        obj.Size,
		CreatedAt:   obj.LastModified,
	}, nil
}

// GetPresignedDownloadURL returns a presigned URL for downloading
func (s *MediaService) GetPresignedDownloadURL(ctx context.Context, objectName string) (string, error) {
	return s.storage.GetPresignedURL(ctx, objectName, PresignedURLExpiry)
}

// Delete removes media
func (s *MediaService) Delete(ctx context.Context, uploaderID, mediaID string) error {
	prefix := fmt.Sprintf("%s/%s", uploaderID, mediaID)
	objects, err := s.storage.ListObjects(ctx, prefix)
	if err != nil {
		return fmt.Errorf("list objects: %w", err)
	}

	for _, obj := range objects {
		if err := s.storage.Delete(ctx, obj.Key); err != nil {
			return fmt.Errorf("delete %s: %w", obj.Key, err)
		}
	}

	return nil
}

func getExtension(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	default:
		return path.Ext(contentType)
	}
}
