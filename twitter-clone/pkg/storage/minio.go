package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	BucketMedia      = "twitter-media"
	BucketThumbnails = "twitter-thumbnails"
)

type MinIOClient struct {
	client *minio.Client
	bucket string
}

// NewMinIOClient creates a new MinIO client
func NewMinIOClient(ctx context.Context, endpoint, accessKey, secretKey string, useSSL bool) (*MinIOClient, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	mc := &MinIOClient{
		client: client,
		bucket: BucketMedia,
	}

	// Ensure buckets exist
	if err := mc.ensureBuckets(ctx); err != nil {
		return nil, fmt.Errorf("ensure buckets: %w", err)
	}

	return mc, nil
}

func (mc *MinIOClient) ensureBuckets(ctx context.Context) error {
	buckets := []string{BucketMedia, BucketThumbnails}

	for _, bucket := range buckets {
		exists, err := mc.client.BucketExists(ctx, bucket)
		if err != nil {
			return fmt.Errorf("check bucket %s: %w", bucket, err)
		}

		if !exists {
			if err := mc.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
				return fmt.Errorf("create bucket %s: %w", bucket, err)
			}
			log.Printf("Created bucket: %s", bucket)

			// Set public read policy for media bucket
			if bucket == BucketMedia {
				policy := `{
					"Version": "2012-10-17",
					"Statement": [{
						"Effect": "Allow",
						"Principal": {"AWS": ["*"]},
						"Action": ["s3:GetObject"],
						"Resource": ["arn:aws:s3:::` + bucket + `/*"]
					}]
				}`
				if err := mc.client.SetBucketPolicy(ctx, bucket, policy); err != nil {
					log.Printf("Warning: failed to set bucket policy: %v", err)
				}
			}
		}
	}

	return nil
}

func (mc *MinIOClient) Health(ctx context.Context) error {
	_, err := mc.client.BucketExists(ctx, mc.bucket)
	return err
}

// Upload uploads a file to MinIO
func (mc *MinIOClient) Upload(ctx context.Context, objectName, contentType string, reader io.Reader, size int64) (string, error) {
	info, err := mc.client.PutObject(ctx, mc.bucket, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("put object: %w", err)
	}

	log.Printf("Uploaded %s (%d bytes)", objectName, info.Size)
	return objectName, nil
}

// UploadThumbnail uploads a thumbnail to the thumbnails bucket
func (mc *MinIOClient) UploadThumbnail(ctx context.Context, objectName, contentType string, reader io.Reader, size int64) (string, error) {
	info, err := mc.client.PutObject(ctx, BucketThumbnails, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("put thumbnail: %w", err)
	}

	log.Printf("Uploaded thumbnail %s (%d bytes)", objectName, info.Size)
	return objectName, nil
}

// Download downloads a file from MinIO
func (mc *MinIOClient) Download(ctx context.Context, objectName string) (io.ReadCloser, error) {
	obj, err := mc.client.GetObject(ctx, mc.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get object: %w", err)
	}
	return obj, nil
}

// Delete removes a file from MinIO
func (mc *MinIOClient) Delete(ctx context.Context, objectName string) error {
	return mc.client.RemoveObject(ctx, mc.bucket, objectName, minio.RemoveObjectOptions{})
}

// GetPresignedURL generates a presigned URL for downloading
func (mc *MinIOClient) GetPresignedURL(ctx context.Context, objectName string, expires time.Duration) (string, error) {
	reqParams := make(url.Values)
	presignedURL, err := mc.client.PresignedGetObject(ctx, mc.bucket, objectName, expires, reqParams)
	if err != nil {
		return "", fmt.Errorf("presign: %w", err)
	}
	return presignedURL.String(), nil
}

// GetPresignedUploadURL generates a presigned URL for uploading
func (mc *MinIOClient) GetPresignedUploadURL(ctx context.Context, objectName string, expires time.Duration) (string, error) {
	presignedURL, err := mc.client.PresignedPutObject(ctx, mc.bucket, objectName, expires)
	if err != nil {
		return "", fmt.Errorf("presign upload: %w", err)
	}
	return presignedURL.String(), nil
}

// GetPublicURL returns the public URL for an object
func (mc *MinIOClient) GetPublicURL(endpoint, objectName string) string {
	return fmt.Sprintf("http://%s/%s/%s", endpoint, mc.bucket, objectName)
}

// Stat returns object info
func (mc *MinIOClient) Stat(ctx context.Context, objectName string) (*minio.ObjectInfo, error) {
	info, err := mc.client.StatObject(ctx, mc.bucket, objectName, minio.StatObjectOptions{})
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// ListObjects lists objects with a prefix
func (mc *MinIOClient) ListObjects(ctx context.Context, prefix string) ([]minio.ObjectInfo, error) {
	var objects []minio.ObjectInfo

	opts := minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}

	for obj := range mc.client.ListObjects(ctx, mc.bucket, opts) {
		if obj.Err != nil {
			return nil, obj.Err
		}
		objects = append(objects, obj)
	}

	return objects, nil
}
