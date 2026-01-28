package models

import (
	"time"
)

// User represents a Twitter user
type User struct {
	ID             string    `json:"id"`
	Username       string    `json:"username"`
	Email          string    `json:"email"`
	DisplayName    string    `json:"display_name"`
	Bio            string    `json:"bio,omitempty"`
	AvatarURL      string    `json:"avatar_url,omitempty"`
	FollowerCount  int       `json:"follower_count"`
	FollowingCount int       `json:"following_count"`
	IsVerified     bool      `json:"is_verified"`
	CreatedAt      time.Time `json:"created_at"`
}

// Follow represents a follow relationship
type Follow struct {
	FollowerID string    `json:"follower_id"`
	FolloweeID string    `json:"followee_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// Tweet represents a tweet
type Tweet struct {
	ID           string    `json:"id"`
	AuthorID     string    `json:"author_id"`
	Content      string    `json:"content"`
	MediaIDs     []string  `json:"media_ids,omitempty"`
	ReplyToID    string    `json:"reply_to_id,omitempty"`
	RetweetOfID  string    `json:"retweet_of_id,omitempty"`
	LikeCount    int       `json:"like_count"`
	RetweetCount int       `json:"retweet_count"`
	ReplyCount   int       `json:"reply_count"`
	CreatedAt    time.Time `json:"created_at"`
	// Populated fields (not stored)
	Author *User `json:"author,omitempty"`
}

// Like represents a like on a tweet
type Like struct {
	UserID    string    `json:"user_id"`
	TweetID   string    `json:"tweet_id"`
	CreatedAt time.Time `json:"created_at"`
}

// Media represents uploaded media
type Media struct {
	ID          string    `json:"id"`
	UploaderID  string    `json:"uploader_id"`
	URL         string    `json:"url"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	Width       int       `json:"width,omitempty"`
	Height      int       `json:"height,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Notification represents a user notification
type Notification struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"user_id"`
	Type      string                 `json:"type"` // like, retweet, follow, mention, reply
	ActorID   string                 `json:"actor_id"`
	TweetID   string                 `json:"tweet_id,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Read      bool                   `json:"read"`
	CreatedAt time.Time              `json:"created_at"`
}

// Timeline represents a user's timeline entry
type TimelineEntry struct {
	TweetID   string    `json:"tweet_id"`
	Score     float64   `json:"score"` // timestamp as score for ZSET
	CreatedAt time.Time `json:"created_at"`
}

// FanoutJob represents a job to fan out a tweet to followers
type FanoutJob struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"` // fanout, index, notify, media_process
	TweetID   string                 `json:"tweet_id,omitempty"`
	AuthorID  string                 `json:"author_id,omitempty"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Priority  string                 `json:"priority"` // high, normal, low
	CreatedAt time.Time              `json:"created_at"`
}

// SearchResult represents search results
type SearchResult struct {
	Tweets []Tweet `json:"tweets,omitempty"`
	Users  []User  `json:"users,omitempty"`
	Total  int64   `json:"total"`
	TookMs int64   `json:"took_ms"`
	Query  string  `json:"query"`
}

// TrendingTopic represents a trending hashtag
type TrendingTopic struct {
	Tag        string `json:"tag"`
	TweetCount int64  `json:"tweet_count"`
	Rank       int    `json:"rank"`
}

// API Request/Response types

type CreateUserRequest struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type UpdateUserRequest struct {
	DisplayName string `json:"display_name,omitempty"`
	Bio         string `json:"bio,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

type CreateTweetRequest struct {
	Content   string   `json:"content"`
	MediaIDs  []string `json:"media_ids,omitempty"`
	ReplyToID string   `json:"reply_to_id,omitempty"`
}

type TimelineResponse struct {
	Tweets     []Tweet `json:"tweets"`
	NextCursor string  `json:"next_cursor,omitempty"`
	HasMore    bool    `json:"has_more"`
}

type FollowersResponse struct {
	Users      []User `json:"users"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}
