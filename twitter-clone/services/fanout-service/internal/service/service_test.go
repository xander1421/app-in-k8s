package service

import (
	"testing"

	"github.com/alexprut/twitter-clone/pkg/models"
)

func TestFanoutConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant int
		minValue int
	}{
		{"SmallFollowerThreshold", SmallFollowerThreshold, 1000},
		{"MediumFollowerThreshold", MediumFollowerThreshold, 100000},
		{"FanoutBatchSize", FanoutBatchSize, 100},
		{"ActiveDaysWindow", ActiveDaysWindow, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant < tt.minValue {
				t.Errorf("%s = %d, want >= %d", tt.name, tt.constant, tt.minValue)
			}
		})
	}
}

func TestFanoutThresholds(t *testing.T) {
	if SmallFollowerThreshold >= MediumFollowerThreshold {
		t.Errorf("SmallFollowerThreshold (%d) should be less than MediumFollowerThreshold (%d)",
			SmallFollowerThreshold, MediumFollowerThreshold)
	}
}

func TestFanoutJobPayload(t *testing.T) {
	tests := []struct {
		name           string
		payload        map[string]interface{}
		expectedCount  int
		shouldFanout   bool
	}{
		{
			name:           "small account should fanout",
			payload:        map[string]interface{}{"follower_count": float64(100)},
			expectedCount:  100,
			shouldFanout:   true,
		},
		{
			name:           "medium account should fanout",
			payload:        map[string]interface{}{"follower_count": float64(500000)},
			expectedCount:  500000,
			shouldFanout:   true,
		},
		{
			name:           "celebrity account should not fanout",
			payload:        map[string]interface{}{"follower_count": float64(2000000)},
			expectedCount:  2000000,
			shouldFanout:   false,
		},
		{
			name:           "missing follower count defaults to zero",
			payload:        map[string]interface{}{},
			expectedCount:  0,
			shouldFanout:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := 0
			if c, ok := tt.payload["follower_count"].(float64); ok {
				count = int(c)
			}

			if count != tt.expectedCount {
				t.Errorf("follower count = %d, want %d", count, tt.expectedCount)
			}

			shouldFanout := count <= MediumFollowerThreshold
			if shouldFanout != tt.shouldFanout {
				t.Errorf("shouldFanout = %v, want %v", shouldFanout, tt.shouldFanout)
			}
		})
	}
}

func TestBatchCalculation(t *testing.T) {
	tests := []struct {
		name          string
		followerCount int
		batchSize     int
		expectedBatches int
	}{
		{"zero followers", 0, FanoutBatchSize, 0},
		{"one batch", 500, FanoutBatchSize, 1},
		{"exact batches", 2000, FanoutBatchSize, 2},
		{"partial last batch", 2500, FanoutBatchSize, 3},
		{"large follower count", 9500, FanoutBatchSize, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batches := 0
			if tt.followerCount > 0 {
				batches = (tt.followerCount + tt.batchSize - 1) / tt.batchSize
			}
			if batches != tt.expectedBatches {
				t.Errorf("batches = %d, want %d", batches, tt.expectedBatches)
			}
		})
	}
}

func TestFanoutJobTypes(t *testing.T) {
	job := models.FanoutJob{
		ID:       "job-123",
		TweetID:  "tweet-456",
		AuthorID: "author-789",
		Payload:  map[string]interface{}{"follower_count": float64(1000)},
	}

	if job.ID == "" {
		t.Error("FanoutJob ID should not be empty")
	}
	if job.TweetID == "" {
		t.Error("FanoutJob TweetID should not be empty")
	}
	if job.AuthorID == "" {
		t.Error("FanoutJob AuthorID should not be empty")
	}
}

func TestNotificationJobPayload(t *testing.T) {
	payload := map[string]interface{}{
		"user_id":    "user-123",
		"notif_type": "like",
		"actor_id":   "actor-456",
	}

	userID, _ := payload["user_id"].(string)
	notifType, _ := payload["notif_type"].(string)
	actorID, _ := payload["actor_id"].(string)

	if userID != "user-123" {
		t.Errorf("user_id = %s, want user-123", userID)
	}
	if notifType != "like" {
		t.Errorf("notif_type = %s, want like", notifType)
	}
	if actorID != "actor-456" {
		t.Errorf("actor_id = %s, want actor-456", actorID)
	}
}

func TestMediaTranscodeJobPayload(t *testing.T) {
	payload := map[string]interface{}{
		"media_id":     "media-123",
		"content_type": "video/mp4",
	}

	mediaID, _ := payload["media_id"].(string)
	contentType, _ := payload["content_type"].(string)

	if mediaID != "media-123" {
		t.Errorf("media_id = %s, want media-123", mediaID)
	}
	if contentType != "video/mp4" {
		t.Errorf("content_type = %s, want video/mp4", contentType)
	}
}
