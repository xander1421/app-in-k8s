package models

import (
	"time"
)

// File represents a file in the system
type File struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
	Checksum    string    `json:"checksum"`
	OwnerID     string    `json:"owner_id"`
	Path        string    `json:"-"` // Internal path, not exposed
	Tags        []string  `json:"tags,omitempty"`
	Downloads   int64     `json:"downloads"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ShareLink represents a shareable link to a file
type ShareLink struct {
	ID        string     `json:"id"`
	FileID    string     `json:"file_id"`
	Token     string     `json:"token"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	MaxUses   *int       `json:"max_uses,omitempty"`
	Uses      int        `json:"uses"`
	CreatedAt time.Time  `json:"created_at"`
}

// User represents a user in the system
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// FileEvent represents a real-time event for WebSocket broadcast
type FileEvent struct {
	Type       string      `json:"type"` // upload, download, delete, share
	FileID     string      `json:"file_id"`
	FileName   string      `json:"file_name"`
	UserID     string      `json:"user_id"`
	InstanceID string      `json:"instance_id"` // Which pod generated this event
	Payload    interface{} `json:"payload,omitempty"`
	Timestamp  time.Time   `json:"timestamp"`
}

// Job represents a background job for RabbitMQ
type Job struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"` // thumbnail, scan, notify
	FileID    string                 `json:"file_id"`
	Payload   map[string]interface{} `json:"payload"`
	CreatedAt time.Time              `json:"created_at"`
}

// SearchResult represents an Elasticsearch search result
type SearchResult struct {
	Files      []File `json:"files"`
	Total      int64  `json:"total"`
	TookMs     int64  `json:"took_ms"`
	Query      string `json:"query"`
}

// ClusterInfo represents cluster state info
type ClusterInfo struct {
	InstanceID    string   `json:"instance_id"`
	ActivePeers   []string `json:"active_peers"`
	ConnectedWS   int      `json:"connected_websockets"`
	UptimeSeconds int64    `json:"uptime_seconds"`
}
