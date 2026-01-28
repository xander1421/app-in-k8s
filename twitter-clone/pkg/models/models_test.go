package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestUserJSON(t *testing.T) {
	user := User{
		ID:             "user-123",
		Username:       "testuser",
		Email:          "test@example.com",
		DisplayName:    "Test User",
		Bio:            "This is a test bio",
		FollowerCount:  100,
		FollowingCount: 50,
		IsVerified:     true,
		CreatedAt:      time.Now(),
	}

	// Test serialization
	data, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("Failed to marshal user: %v", err)
	}

	// Test deserialization
	var decoded User
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal user: %v", err)
	}

	if decoded.ID != user.ID {
		t.Errorf("Expected ID %s, got %s", user.ID, decoded.ID)
	}
	if decoded.Username != user.Username {
		t.Errorf("Expected Username %s, got %s", user.Username, decoded.Username)
	}
	if decoded.IsVerified != user.IsVerified {
		t.Errorf("Expected IsVerified %v, got %v", user.IsVerified, decoded.IsVerified)
	}
}

func TestTweetJSON(t *testing.T) {
	tweet := Tweet{
		ID:           "tweet-123",
		AuthorID:     "user-123",
		Content:      "Hello, world! #test @user",
		MediaIDs:     []string{"media-1", "media-2"},
		LikeCount:    10,
		RetweetCount: 5,
		ReplyCount:   2,
		CreatedAt:    time.Now(),
	}

	data, err := json.Marshal(tweet)
	if err != nil {
		t.Fatalf("Failed to marshal tweet: %v", err)
	}

	var decoded Tweet
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal tweet: %v", err)
	}

	if decoded.ID != tweet.ID {
		t.Errorf("Expected ID %s, got %s", tweet.ID, decoded.ID)
	}
	if decoded.Content != tweet.Content {
		t.Errorf("Expected Content %s, got %s", tweet.Content, decoded.Content)
	}
	if len(decoded.MediaIDs) != len(tweet.MediaIDs) {
		t.Errorf("Expected %d media IDs, got %d", len(tweet.MediaIDs), len(decoded.MediaIDs))
	}
}

func TestTweetWithReply(t *testing.T) {
	tweet := Tweet{
		ID:        "reply-123",
		AuthorID:  "user-456",
		Content:   "This is a reply",
		ReplyToID: "tweet-123",
		CreatedAt: time.Now(),
	}

	if tweet.ReplyToID == "" {
		t.Error("Expected ReplyToID to be set")
	}
}

func TestTweetWithRetweet(t *testing.T) {
	tweet := Tweet{
		ID:          "retweet-123",
		AuthorID:    "user-789",
		Content:     "",
		RetweetOfID: "tweet-123",
		CreatedAt:   time.Now(),
	}

	if tweet.RetweetOfID == "" {
		t.Error("Expected RetweetOfID to be set")
	}
}

func TestCreateUserRequest(t *testing.T) {
	req := CreateUserRequest{
		Username:    "newuser",
		Email:       "new@example.com",
		DisplayName: "New User",
	}

	if req.Username == "" {
		t.Error("Username should not be empty")
	}
	if req.Email == "" {
		t.Error("Email should not be empty")
	}
}

func TestCreateTweetRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     CreateTweetRequest
		wantErr bool
	}{
		{
			name: "valid tweet",
			req: CreateTweetRequest{
				Content: "This is a valid tweet",
			},
			wantErr: false,
		},
		{
			name: "tweet with media",
			req: CreateTweetRequest{
				Content:  "Tweet with media",
				MediaIDs: []string{"media-1"},
			},
			wantErr: false,
		},
		{
			name: "reply tweet",
			req: CreateTweetRequest{
				Content:   "This is a reply",
				ReplyToID: "tweet-123",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just validate the struct can be created
			if tt.req.Content == "" && !tt.wantErr {
				t.Error("Content should not be empty for valid requests")
			}
		})
	}
}

func TestTimelineResponse(t *testing.T) {
	resp := TimelineResponse{
		Tweets: []Tweet{
			{ID: "tweet-1", Content: "First tweet"},
			{ID: "tweet-2", Content: "Second tweet"},
		},
		NextCursor: "cursor-123",
		HasMore:    true,
	}

	if len(resp.Tweets) != 2 {
		t.Errorf("Expected 2 tweets, got %d", len(resp.Tweets))
	}
	if !resp.HasMore {
		t.Error("Expected HasMore to be true")
	}
}

func TestErrorResponse(t *testing.T) {
	resp := ErrorResponse{
		Error:   "Something went wrong",
		Code:    "internal_error",
		Details: "Additional details here",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal error response: %v", err)
	}

	var decoded ErrorResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal error response: %v", err)
	}

	if decoded.Error != resp.Error {
		t.Errorf("Expected Error %s, got %s", resp.Error, decoded.Error)
	}
	if decoded.Code != resp.Code {
		t.Errorf("Expected Code %s, got %s", resp.Code, decoded.Code)
	}
}

func TestTrendingTopic(t *testing.T) {
	topic := TrendingTopic{
		Tag:        "golang",
		TweetCount: 1000,
		Rank:       1,
	}

	if topic.Tag == "" {
		t.Error("Tag should not be empty")
	}
	if topic.TweetCount <= 0 {
		t.Error("TweetCount should be positive")
	}
}

func TestNotification(t *testing.T) {
	notif := Notification{
		ID:        "notif-123",
		UserID:    "user-123",
		Type:      "like",
		ActorID:   "user-456",
		TweetID:   "tweet-789",
		Read:      false,
		CreatedAt: time.Now(),
	}

	if notif.Type == "" {
		t.Error("Type should not be empty")
	}
	if notif.Read {
		t.Error("New notification should be unread")
	}
}
