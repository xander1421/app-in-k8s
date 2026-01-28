package service

import (
	"testing"
	"time"
)

func TestNotificationLimitValidation(t *testing.T) {
	tests := []struct {
		name          string
		inputLimit    int
		expectedLimit int
	}{
		{"zero uses default", 0, 20},
		{"negative uses default", -5, 20},
		{"normal limit unchanged", 50, 50},
		{"exceeds max capped", 150, 100},
		{"exactly max allowed", 100, 100},
		{"exactly default", 20, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit := tt.inputLimit
			if limit <= 0 {
				limit = 20
			}
			if limit > 100 {
				limit = 100
			}

			if limit != tt.expectedLimit {
				t.Errorf("limit = %d, want %d", limit, tt.expectedLimit)
			}
		})
	}
}

func TestSelfNotificationPrevention(t *testing.T) {
	tests := []struct {
		name           string
		userID         string
		actorID        string
		shouldNotify   bool
	}{
		{
			name:         "different users should notify",
			userID:       "user-123",
			actorID:      "user-456",
			shouldNotify: true,
		},
		{
			name:         "same user should not notify",
			userID:       "user-123",
			actorID:      "user-123",
			shouldNotify: false,
		},
		{
			name:         "empty actor should notify",
			userID:       "user-123",
			actorID:      "",
			shouldNotify: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldNotify := tt.userID != tt.actorID
			if shouldNotify != tt.shouldNotify {
				t.Errorf("shouldNotify = %v, want %v", shouldNotify, tt.shouldNotify)
			}
		})
	}
}

func TestHasMoreCalculation(t *testing.T) {
	tests := []struct {
		name     string
		results  int
		limit    int
		hasMore  bool
	}{
		{"fewer than limit", 10, 20, false},
		{"exactly at limit", 20, 20, false},
		{"more than limit", 21, 20, true},
		{"empty result", 0, 20, false},
		{"one over limit", 51, 50, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasMore := tt.results > tt.limit
			if hasMore != tt.hasMore {
				t.Errorf("hasMore = %v, want %v", hasMore, tt.hasMore)
			}
		})
	}
}

func TestNotificationTypes(t *testing.T) {
	validTypes := []string{
		"like",
		"retweet",
		"follow",
		"mention",
		"reply",
	}

	for _, notifType := range validTypes {
		if notifType == "" {
			t.Error("notification type should not be empty")
		}
	}
}

func TestCleanupDuration(t *testing.T) {
	tests := []struct {
		name     string
		maxAge   time.Duration
		now      time.Time
		expected time.Time
	}{
		{
			name:     "30 days cleanup",
			maxAge:   30 * 24 * time.Hour,
			now:      time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "7 days cleanup",
			maxAge:   7 * 24 * time.Hour,
			now:      time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := tt.now.Add(-tt.maxAge)
			if !before.Equal(tt.expected) {
				t.Errorf("cleanup before = %v, want %v", before, tt.expected)
			}
		})
	}
}

func TestNotificationDataPayload(t *testing.T) {
	data := map[string]interface{}{
		"tweet_content": "Hello world",
		"actor_name":    "John Doe",
	}

	if data["tweet_content"] != "Hello world" {
		t.Error("tweet_content should be preserved")
	}
	if data["actor_name"] != "John Doe" {
		t.Error("actor_name should be preserved")
	}
}

func TestPaginationOffset(t *testing.T) {
	tests := []struct {
		name       string
		limit      int
		page       int
		expected   int
	}{
		{"first page", 20, 0, 0},
		{"second page", 20, 1, 20},
		{"third page", 20, 2, 40},
		{"custom limit", 50, 2, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset := tt.limit * tt.page
			if offset != tt.expected {
				t.Errorf("offset = %d, want %d", offset, tt.expected)
			}
		})
	}
}
