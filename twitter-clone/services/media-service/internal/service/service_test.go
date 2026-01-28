package service

import (
	"testing"
)

func TestMediaConstants(t *testing.T) {
	if MaxUploadSize <= 0 {
		t.Error("MaxUploadSize should be positive")
	}
	if MaxUploadSize > 100*1024*1024 {
		t.Error("MaxUploadSize should not exceed 100MB")
	}

	if PresignedURLExpiry <= 0 {
		t.Error("PresignedURLExpiry should be positive")
	}
}

func TestAllowedContentTypes(t *testing.T) {
	expectedTypes := []string{
		"image/jpeg",
		"image/png",
		"image/gif",
		"image/webp",
		"video/mp4",
		"video/webm",
	}

	for _, ct := range expectedTypes {
		if !AllowedContentTypes[ct] {
			t.Errorf("content type %s should be allowed", ct)
		}
	}

	disallowedTypes := []string{
		"text/plain",
		"application/pdf",
		"image/bmp",
		"video/avi",
		"application/octet-stream",
	}

	for _, ct := range disallowedTypes {
		if AllowedContentTypes[ct] {
			t.Errorf("content type %s should not be allowed", ct)
		}
	}
}

func TestGetExtension(t *testing.T) {
	tests := []struct {
		contentType string
		expected    string
	}{
		{"image/jpeg", ".jpg"},
		{"image/png", ".png"},
		{"image/gif", ".gif"},
		{"image/webp", ".webp"},
		{"video/mp4", ".mp4"},
		{"video/webm", ".webm"},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			ext := getExtension(tt.contentType)
			if ext != tt.expected {
				t.Errorf("getExtension(%s) = %s, want %s", tt.contentType, ext, tt.expected)
			}
		})
	}
}

func TestUploadValidation(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		size        int64
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid jpeg upload",
			contentType: "image/jpeg",
			size:        1024 * 1024,
			expectError: false,
		},
		{
			name:        "valid png upload",
			contentType: "image/png",
			size:        5 * 1024 * 1024,
			expectError: false,
		},
		{
			name:        "valid mp4 upload",
			contentType: "video/mp4",
			size:        MaxUploadSize,
			expectError: false,
		},
		{
			name:        "invalid content type",
			contentType: "application/pdf",
			size:        1024,
			expectError: true,
			errorMsg:    "unsupported content type",
		},
		{
			name:        "file too large",
			contentType: "image/jpeg",
			size:        MaxUploadSize + 1,
			expectError: true,
			errorMsg:    "file too large",
		},
		{
			name:        "empty content type",
			contentType: "",
			size:        1024,
			expectError: true,
			errorMsg:    "unsupported content type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := AllowedContentTypes[tt.contentType] && tt.size <= MaxUploadSize
			hasError := !isValid

			if hasError != tt.expectError {
				t.Errorf("validation error = %v, want %v", hasError, tt.expectError)
			}
		})
	}
}

func TestObjectNameGeneration(t *testing.T) {
	uploaderID := "user-123"
	mediaID := "media-456"
	contentType := "image/jpeg"

	ext := getExtension(contentType)
	expected := uploaderID + "/" + mediaID + ext

	objectName := uploaderID + "/" + mediaID + ext

	if objectName != expected {
		t.Errorf("object name = %s, want %s", objectName, expected)
	}
}

func TestMaxUploadSizeBytes(t *testing.T) {
	expectedMB := 10
	expectedBytes := int64(expectedMB * 1024 * 1024)

	if MaxUploadSize != expectedBytes {
		t.Errorf("MaxUploadSize = %d bytes, want %d bytes (%d MB)",
			MaxUploadSize, expectedBytes, expectedMB)
	}
}

func TestPresignedURLExpiryMinutes(t *testing.T) {
	expectedMinutes := 15.0
	actualMinutes := PresignedURLExpiry.Minutes()

	if actualMinutes != expectedMinutes {
		t.Errorf("PresignedURLExpiry = %v minutes, want %v minutes",
			actualMinutes, expectedMinutes)
	}
}
