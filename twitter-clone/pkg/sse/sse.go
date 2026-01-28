package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/alexprut/twitter-clone/pkg/auth"
	"github.com/alexprut/twitter-clone/pkg/models"
)

// MessageType defines SSE message types
type MessageType string

const (
	// Outgoing message types
	NewTweetMessage        MessageType = "new_tweet"
	LikeMessage           MessageType = "like"
	RetweetMessage        MessageType = "retweet"
	FollowMessage         MessageType = "follow"
	NotificationMessage   MessageType = "notification"
	TimelineUpdateMessage MessageType = "timeline_update"
	UserStatusMessage     MessageType = "user_status"
	SystemMessage         MessageType = "system"
)

// Message represents an SSE message
type Message struct {
	ID        string      `json:"id,omitempty"`
	Type      MessageType `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// Client represents an SSE connection
type Client struct {
	ID       string
	UserID   string
	Username string
	Email    string
	
	writer   http.ResponseWriter
	flusher  http.Flusher
	ctx      context.Context
	cancel   context.CancelFunc
	lastSeen time.Time
	
	mu sync.RWMutex
}

// Hub manages SSE connections
type Hub struct {
	clients     map[string]*Client
	userClients map[string][]*Client // userID -> []*Client
	broadcast   chan Message
	register    chan *Client
	unregister  chan *Client
	mu          sync.RWMutex
	jwtManager  *auth.JWTManager
}

// NewHub creates a new SSE hub
func NewHub(jwtManager *auth.JWTManager) *Hub {
	hub := &Hub{
		clients:     make(map[string]*Client),
		userClients: make(map[string][]*Client),
		broadcast:   make(chan Message, 1000),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		jwtManager:  jwtManager,
	}
	
	go hub.run()
	go hub.cleanup()
	
	return hub
}

// ServeSSE handles SSE connections
func (h *Hub) ServeSSE(w http.ResponseWriter, r *http.Request) {
	// Validate authentication
	token := r.URL.Query().Get("token")
	if token == "" {
		// Try Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" && len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token = authHeader[7:]
		}
	}

	// Validate token
	claims, err := h.jwtManager.ValidateToken(token)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Validate access token type
	if claims.Type != auth.AccessToken {
		http.Error(w, "Invalid token type", http.StatusUnauthorized)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	
	client := &Client{
		ID:       generateClientID(),
		UserID:   claims.UserID,
		Username: claims.Username,
		Email:    claims.Email,
		writer:   w,
		flusher:  flusher,
		ctx:      ctx,
		cancel:   cancel,
		lastSeen: time.Now(),
	}

	h.register <- client

	// Send welcome message
	welcomeMsg := Message{
		Type: SystemMessage,
		Data: map[string]string{
			"message": "Connected to Twitter Clone real-time updates",
			"user_id": claims.UserID,
		},
		Timestamp: time.Now(),
	}
	client.writeMessage(welcomeMsg)

	// Keep connection alive
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.unregister <- client
			return
		case <-ticker.C:
			// Send keep-alive ping
			pingMsg := Message{
				Type: SystemMessage,
				Data: map[string]string{"type": "ping"},
				Timestamp: time.Now(),
			}
			if err := client.writeMessage(pingMsg); err != nil {
				h.unregister <- client
				return
			}
		}
	}
}

// writeMessage sends an SSE message to the client
func (c *Client) writeMessage(msg Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	
	// Write SSE format
	if msg.ID != "" {
		fmt.Fprintf(c.writer, "id: %s\n", msg.ID)
	}
	fmt.Fprintf(c.writer, "event: %s\n", msg.Type)
	fmt.Fprintf(c.writer, "data: %s\n\n", string(data))
	
	c.flusher.Flush()
	c.lastSeen = time.Now()
	
	return nil
}

// run handles hub operations
func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			h.userClients[client.UserID] = append(h.userClients[client.UserID], client)
			h.mu.Unlock()
			
			log.Printf("SSE client connected: %s (user: %s)", client.ID, client.UserID)
			
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				
				// Remove from user clients
				userClients := h.userClients[client.UserID]
				for i, c := range userClients {
					if c.ID == client.ID {
						h.userClients[client.UserID] = append(userClients[:i], userClients[i+1:]...)
						break
					}
				}
				
				// Clean up empty user client list
				if len(h.userClients[client.UserID]) == 0 {
					delete(h.userClients, client.UserID)
				}
			}
			h.mu.Unlock()
			
			client.cancel()
			log.Printf("SSE client disconnected: %s", client.ID)
			
		case message := <-h.broadcast:
			h.broadcastMessage(message)
		}
	}
}

// broadcastMessage sends a message to all connected clients
func (h *Hub) broadcastMessage(message Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	for _, client := range h.clients {
		select {
		case <-client.ctx.Done():
			// Client disconnected
			continue
		default:
			if err := client.writeMessage(message); err != nil {
				log.Printf("Error sending message to client %s: %v", client.ID, err)
			}
		}
	}
}

// BroadcastToUser sends a message to all connections of a specific user
func (h *Hub) BroadcastToUser(userID string, message Message) {
	h.mu.RLock()
	clients := h.userClients[userID]
	h.mu.RUnlock()
	
	for _, client := range clients {
		select {
		case <-client.ctx.Done():
			continue
		default:
			if err := client.writeMessage(message); err != nil {
				log.Printf("Error sending message to user %s client %s: %v", userID, client.ID, err)
			}
		}
	}
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(message Message) {
	select {
	case h.broadcast <- message:
	default:
		log.Printf("Broadcast channel full, dropping message: %+v", message)
	}
}

// NotifyNewTweet notifies followers about a new tweet
func (h *Hub) NotifyNewTweet(tweet *models.Tweet, followerIDs []string) {
	message := Message{
		Type:      NewTweetMessage,
		Data:      tweet,
		Timestamp: time.Now(),
	}
	
	for _, followerID := range followerIDs {
		h.BroadcastToUser(followerID, message)
	}
}

// NotifyLike notifies about a like
func (h *Hub) NotifyLike(like *models.Like, tweetAuthorID string) {
	message := Message{
		Type:      LikeMessage,
		Data:      like,
		Timestamp: time.Now(),
	}
	
	h.BroadcastToUser(tweetAuthorID, message)
}

// NotifyFollow notifies about a new follow
func (h *Hub) NotifyFollow(follow *models.Follow) {
	message := Message{
		Type:      FollowMessage,
		Data:      follow,
		Timestamp: time.Now(),
	}
	
	h.BroadcastToUser(follow.FolloweeID, message)
}

// NotifyNotification sends a notification
func (h *Hub) NotifyNotification(notification *models.Notification) {
	message := Message{
		Type:      NotificationMessage,
		Data:      notification,
		Timestamp: time.Now(),
	}
	
	h.BroadcastToUser(notification.UserID, message)
}

// cleanup removes stale connections
func (h *Hub) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		h.mu.RLock()
		staleClients := make([]*Client, 0)
		for _, client := range h.clients {
			if time.Since(client.lastSeen) > 10*time.Minute {
				staleClients = append(staleClients, client)
			}
		}
		h.mu.RUnlock()
		
		for _, client := range staleClients {
			h.unregister <- client
		}
	}
}

// GetStats returns hub statistics
func (h *Hub) GetStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	return map[string]interface{}{
		"total_clients":    len(h.clients),
		"total_users":      len(h.userClients),
		"broadcast_buffer": len(h.broadcast),
	}
}

func generateClientID() string {
	return fmt.Sprintf("sse_%d", time.Now().UnixNano())
}