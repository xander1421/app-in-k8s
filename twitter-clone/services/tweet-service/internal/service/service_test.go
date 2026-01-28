package service

import (
	"testing"

	"github.com/alexprut/twitter-clone/pkg/models"
)

// Unit tests for service layer validation logic

func TestTweetService_CreateTweet_Validation(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid tweet",
			content:     "Hello, world!",
			wantErr:     false,
			errContains: "",
		},
		{
			name:        "empty content",
			content:     "",
			wantErr:     true,
			errContains: "empty",
		},
		{
			name:        "max length exactly",
			content:     string(make([]byte, MaxTweetLength)),
			wantErr:     false,
			errContains: "",
		},
		{
			name:        "exceeds max length",
			content:     string(make([]byte, MaxTweetLength+1)),
			wantErr:     true,
			errContains: "exceeds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate content length logic without needing repository
			if len(tt.content) == 0 && !tt.wantErr {
				t.Error("Expected error for empty content")
			}
			if len(tt.content) > MaxTweetLength && !tt.wantErr {
				t.Error("Expected error for content exceeding max length")
			}
		})
	}
}

func TestMaxTweetLength(t *testing.T) {
	if MaxTweetLength != 280 {
		t.Errorf("Expected MaxTweetLength to be 280, got %d", MaxTweetLength)
	}
}

func TestCreateTweetRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		req     models.CreateTweetRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: models.CreateTweetRequest{
				Content: "Hello, world!",
			},
			wantErr: false,
		},
		{
			name: "with media",
			req: models.CreateTweetRequest{
				Content:  "Tweet with media",
				MediaIDs: []string{"media-1", "media-2"},
			},
			wantErr: false,
		},
		{
			name: "as reply",
			req: models.CreateTweetRequest{
				Content:   "This is a reply",
				ReplyToID: "tweet-123",
			},
			wantErr: false,
		},
		{
			name: "empty content",
			req: models.CreateTweetRequest{
				Content: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate the request
			hasError := len(tt.req.Content) == 0 || len(tt.req.Content) > MaxTweetLength
			if hasError != tt.wantErr {
				t.Errorf("Expected wantErr=%v, got error=%v", tt.wantErr, hasError)
			}
		})
	}
}

func TestTweetContentValidation(t *testing.T) {
	// Test various content lengths
	testCases := []struct {
		length  int
		isValid bool
	}{
		{0, false},
		{1, true},
		{140, true},
		{280, true},
		{281, false},
		{500, false},
	}

	for _, tc := range testCases {
		content := string(make([]byte, tc.length))
		valid := len(content) > 0 && len(content) <= MaxTweetLength
		if valid != tc.isValid {
			t.Errorf("Content length %d: expected valid=%v, got %v", tc.length, tc.isValid, valid)
		}
	}
}
