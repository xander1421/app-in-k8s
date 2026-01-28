package models

import (
	"time"
)

// User represents a Twitter user
type User struct {
	ID               string     `json:"id"`
	Username         string     `json:"username"`
	Email            string     `json:"email"`
	PasswordHash     string     `json:"-" db:"password_hash"` // Never expose in JSON
	DisplayName      string     `json:"display_name"`
	Bio              string     `json:"bio,omitempty"`
	AvatarURL        string     `json:"avatar_url,omitempty"`
	FollowerCount    int        `json:"follower_count"`
	FollowingCount   int        `json:"following_count"`
	IsVerified       bool       `json:"is_verified"`
	IsActive         bool       `json:"is_active"`
	EmailVerified    bool       `json:"email_verified"`
	EmailVerifiedAt  *time.Time `json:"email_verified_at,omitempty"`
	LastActiveAt     *time.Time `json:"last_active_at,omitempty"`
	LastLoginAt      *time.Time `json:"last_login_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
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

// Media represents uploaded media
type Media struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Type        string    `json:"type"` // image, video, gif
	URL         string    `json:"url"`
	ThumbnailURL string   `json:"thumbnail_url,omitempty"`
	Width       int       `json:"width,omitempty"`
	Height      int       `json:"height,omitempty"`
	Duration    int       `json:"duration,omitempty"` // For videos, in seconds
	Size        int64     `json:"size"` // File size in bytes
	MimeType    string    `json:"mime_type"`
	ProcessingStatus string `json:"processing_status"` // pending, processing, completed, failed
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Block represents a user block
type Block struct {
	BlockerID string    `json:"blocker_id"`
	BlockedID string    `json:"blocked_id"`
	CreatedAt time.Time `json:"created_at"`
}

// Mute represents a user mute
type Mute struct {
	MuterID   string    `json:"muter_id"`
	MutedID   string    `json:"muted_id"`
	CreatedAt time.Time `json:"created_at"`
}

// Bookmark represents a bookmarked tweet
type Bookmark struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TweetID   string    `json:"tweet_id"`
	CreatedAt time.Time `json:"created_at"`
}

// BookmarkStats represents bookmark statistics
type BookmarkStats struct {
	UserID       string `json:"user_id"`
	TotalCount   int    `json:"total_count"`
	MonthlyCount int    `json:"monthly_count"`
	WeeklyCount  int    `json:"weekly_count"`
}

// List represents a user list
type List struct {
	ID          string    `json:"id"`
	OwnerID     string    `json:"owner_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	IsPrivate   bool      `json:"is_private"`
	MemberCount int       `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ListMember represents membership in a list
type ListMember struct {
	ListID    string    `json:"list_id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

// DirectMessage represents a DM
type DirectMessage struct {
	ID           string    `json:"id"`
	SenderID     string    `json:"sender_id"`
	RecipientID  string    `json:"recipient_id"`
	Content      string    `json:"content"`
	MediaIDs     []string  `json:"media_ids,omitempty"`
	IsRead       bool      `json:"is_read"`
	ReadAt       *time.Time `json:"read_at,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// Session represents an active user session
type Session struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	RefreshToken string    `json:"refresh_token"`
	UserAgent    string    `json:"user_agent"`
	IP           string    `json:"ip"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
	LastUsedAt   *time.Time `json:"last_used_at"`
}

// PasswordReset represents a password reset request
type PasswordReset struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Token     string     `json:"token"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// EmailVerification represents an email verification
type EmailVerification struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Email      string     `json:"email"`
	Token      string     `json:"token"`
	ExpiresAt  time.Time  `json:"expires_at"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// API Request/Response types

// Authentication requests
type LoginRequest struct {
	Username  string `json:"username"` // Can be username or email
	Password  string `json:"password"`
	UserAgent string `json:"user_agent,omitempty"`
	IP        string `json:"ip,omitempty"`
}

type SignupRequest struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         *User  `json:"user"`
	ExpiresIn    int    `json:"expires_in"` // Seconds
}

type CreateUserRequest struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	Password    string `json:"password"`
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
