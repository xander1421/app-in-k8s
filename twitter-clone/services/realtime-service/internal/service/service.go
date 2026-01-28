package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/alexprut/twitter-clone/pkg/cache"
	"github.com/alexprut/twitter-clone/pkg/models"
	"github.com/alexprut/twitter-clone/pkg/queue"
	"github.com/alexprut/twitter-clone/pkg/sse"
)

// RealtimeService handles real-time event processing and SSE broadcasting
type RealtimeService struct {
	hub   *sse.Hub
	redis *cache.RedisCache
	rmq   *queue.RabbitMQ
}

// NewRealtimeService creates a new realtime service
func NewRealtimeService(hub *sse.Hub, redis *cache.RedisCache, rmq *queue.RabbitMQ) *RealtimeService {
	return &RealtimeService{
		hub:   hub,
		redis: redis,
		rmq:   rmq,
	}
}

// StartEventConsumers starts consuming events from message queues
func (s *RealtimeService) StartEventConsumers(ctx context.Context) {
	// Register handlers for different job types
	s.rmq.RegisterHandler("notifications.realtime", s.handleNotificationJob)
	s.rmq.RegisterHandler("timeline.updates", s.handleTimelineJob)
	s.rmq.RegisterHandler("presence.updates", s.handlePresenceJob)
	s.rmq.RegisterHandler("direct_messages.new", s.handleDirectMessageJob)
	
	// Start consumers
	go s.rmq.StartConsumer(ctx, "notifications.realtime")
	go s.rmq.StartConsumer(ctx, "timeline.updates")
	go s.rmq.StartConsumer(ctx, "presence.updates")
	go s.rmq.StartConsumer(ctx, "direct_messages.new")
	
	// Start Redis pub/sub for cross-instance communication
	go s.startRedisPubSub(ctx)
	
	log.Println("Realtime event consumers started")
}

// handleNotificationJob processes notification events
func (s *RealtimeService) handleNotificationJob(job models.FanoutJob) error {
	if job.Type != "notify" {
		return nil // Not a notification job
	}
	
	// Extract notification data from job payload
	notificationData, ok := job.Payload["notification"]
	if !ok {
		return fmt.Errorf("no notification data in job payload")
	}
	
	notificationBytes, err := json.Marshal(notificationData)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}
	
	var notification models.Notification
	if err := json.Unmarshal(notificationBytes, &notification); err != nil {
		return fmt.Errorf("failed to unmarshal notification: %w", err)
	}
	
	// Broadcast to user via SSE
	s.hub.NotifyNotification(&notification)
	
	// Also publish to Redis for other instances
	ctx := context.Background()
	s.publishToRedis(ctx, fmt.Sprintf("user:%s:notifications", notification.UserID), notificationBytes)
	
	return nil
}

// handleTimelineJob processes timeline update events
func (s *RealtimeService) handleTimelineJob(job models.FanoutJob) error {
	if job.Type != "timeline" {
		return nil // Not a timeline job
	}
	
	// Extract user ID from job payload or use TweetID and AuthorID
	userID, ok := job.Payload["user_id"].(string)
	if !ok {
		userID = job.AuthorID // Fallback to author ID
	}
	
	// For timeline updates, we notify about new tweets
	if job.TweetID != "" {
		// Send timeline update to user via SSE
		msg := sse.Message{
			Type: sse.NewTweetMessage,
			Data: map[string]string{
				"tweet_id": job.TweetID,
				"author_id": job.AuthorID,
				"action": "new",
			},
			Timestamp: time.Now(),
		}
		s.hub.BroadcastToUser(userID, msg)
		
		// Publish to Redis for cross-instance communication
		ctx := context.Background()
		msgBytes, _ := json.Marshal(msg)
		s.publishToRedis(ctx, fmt.Sprintf("user:%s:timeline", userID), msgBytes)
	}
	
	return nil
}

// handlePresenceJob processes user presence events
func (s *RealtimeService) handlePresenceJob(job models.FanoutJob) error {
	if job.Type != "presence" {
		return nil // Not a presence job
	}
	
	// Extract presence data from job payload
	userID, ok := job.Payload["user_id"].(string)
	if !ok {
		userID = job.AuthorID // Fallback
	}
	
	status, ok := job.Payload["status"].(string)
	if !ok {
		status = "online" // Default
	}
	
	// Update presence in Redis
	ctx := context.Background()
	presenceKey := fmt.Sprintf("presence:%s", userID)
	s.redis.Set(ctx, presenceKey, status, 5*time.Minute)
	
	// Get user's followers to notify
	followers := s.getUserFollowers(ctx, userID)
	
	// Broadcast to followers
	message := sse.Message{
		Type: sse.UserStatusMessage,
		Data: map[string]string{
			"user_id": userID,
			"status":  status,
		},
		Timestamp: time.Now(),
	}
	
	for _, followerID := range followers {
		s.hub.BroadcastToUser(followerID, message)
	}
	
	return nil
}

// handleDirectMessageJob processes direct message events
func (s *RealtimeService) handleDirectMessageJob(job models.FanoutJob) error {
	if job.Type != "dm" {
		return nil // Not a direct message job
	}
	
	// Extract DM data from job payload
	dmData, ok := job.Payload["message"]
	if !ok {
		return fmt.Errorf("no message data in job payload")
	}
	
	dmBytes, err := json.Marshal(dmData)
	if err != nil {
		return fmt.Errorf("failed to marshal DM: %w", err)
	}
	
	var dm struct {
		ID         string    `json:"id"`
		SenderID   string    `json:"sender_id"`
		ReceiverID string    `json:"receiver_id"`
		Content    string    `json:"content"`
		CreatedAt  time.Time `json:"created_at"`
	}
	
	if err := json.Unmarshal(dmBytes, &dm); err != nil {
		return fmt.Errorf("failed to unmarshal DM: %w", err)
	}
	
	// Create SSE message
	message := sse.Message{
		Type: "direct_message",
		Data: dm,
		Timestamp: time.Now(),
	}
	
	// Send to receiver
	s.hub.BroadcastToUser(dm.ReceiverID, message)
	
	// Send delivery confirmation to sender
	confirmation := sse.Message{
		Type: "dm_delivered",
		Data: map[string]string{"message_id": dm.ID},
		Timestamp: time.Now(),
	}
	s.hub.BroadcastToUser(dm.SenderID, confirmation)
	
	return nil
}

// startRedisPubSub starts Redis pub/sub for cross-instance communication
func (s *RealtimeService) startRedisPubSub(ctx context.Context) {
	// Subscribe to global events channel
	pubsub := s.redis.Subscribe(ctx, "realtime:global")
	defer pubsub.Close()
	
	ch := pubsub.Channel()
	for msg := range ch {
		// Parse and broadcast message
		var message sse.Message
		if err := json.Unmarshal([]byte(msg.Payload), &message); err != nil {
			log.Printf("Error parsing Redis pub/sub message: %v", err)
			continue
		}
		
		// Broadcast to local clients
		s.hub.Broadcast(message)
	}
}

// publishToRedis publishes an event to Redis pub/sub
func (s *RealtimeService) publishToRedis(ctx context.Context, channel string, data []byte) {
	if err := s.redis.Publish(ctx, channel, string(data)); err != nil {
		log.Printf("Error publishing to Redis channel %s: %v", channel, err)
	}
}

// getUserFollowers gets list of user's followers (for presence updates)
func (s *RealtimeService) getUserFollowers(ctx context.Context, userID string) []string {
	// This would typically query the user service or cache
	// For now, return from Redis cache if available
	key := fmt.Sprintf("user:%s:followers", userID)
	
	followers, err := s.redis.GetList(ctx, key)
	if err != nil {
		return []string{}
	}
	
	return followers
}

// GetOnlineUsers returns list of online users
func (s *RealtimeService) GetOnlineUsers(ctx context.Context) ([]string, error) {
	// Get stats from hub (simplified for now)
	_ = s.hub.GetStats() // For now, not using local stats
	localUsers := make([]string, 0)
	
	// Also get from Redis (other instances)
	pattern := "presence:*"
	keys, err := s.redis.Keys(ctx, pattern)
	if err != nil {
		return localUsers, nil
	}
	
	userMap := make(map[string]bool)
	for _, user := range localUsers {
		userMap[user] = true
	}
	
	for _, key := range keys {
		userID := key[9:] // Remove "presence:" prefix
		userMap[userID] = true
	}
	
	result := make([]string, 0, len(userMap))
	for userID := range userMap {
		result = append(result, userID)
	}
	
	return result, nil
}

// SendSystemMessage sends a system message to all users
func (s *RealtimeService) SendSystemMessage(ctx context.Context, message string) {
	msg := sse.Message{
		Type: sse.SystemMessage,
		Data: map[string]string{"message": message},
		Timestamp: time.Now(),
	}
	
	// Broadcast locally
	s.hub.Broadcast(msg)
	
	// Publish to Redis for other instances
	data, _ := json.Marshal(msg)
	s.publishToRedis(ctx, "realtime:global", data)
}

// TrackUserActivity tracks user activity for presence
func (s *RealtimeService) TrackUserActivity(ctx context.Context, userID string) {
	// Update last seen
	lastSeenKey := fmt.Sprintf("user:%s:last_seen", userID)
	s.redis.Set(ctx, lastSeenKey, time.Now().Unix(), 24*time.Hour)
	
	// Update presence
	presenceKey := fmt.Sprintf("presence:%s", userID)
	s.redis.Set(ctx, presenceKey, "online", 5*time.Minute)
	
	// Publish presence update as FanoutJob
	job := models.FanoutJob{
		ID:       fmt.Sprintf("presence_%d", time.Now().UnixNano()),
		Type:     "presence",
		AuthorID: userID,
		Payload: map[string]interface{}{
			"user_id": userID,
			"status":  "online",
		},
		Priority:  "normal",
		CreatedAt: time.Now(),
	}
	s.rmq.Publish(ctx, "presence.updates", job)
}

// GetUserPresence gets user's current presence status
func (s *RealtimeService) GetUserPresence(ctx context.Context, userID string) (string, error) {
	// Check if user has active SSE connection (simplified check)
	// Note: SSE hub doesn't expose user-specific online status
	// We rely primarily on Redis presence for now
	
	// Check Redis presence
	presenceKey := fmt.Sprintf("presence:%s", userID)
	var status string
	err := s.redis.Get(ctx, presenceKey, &status)
	if err != nil {
		// Check last seen
		lastSeenKey := fmt.Sprintf("user:%s:last_seen", userID)
		var lastSeenStr string
		err := s.redis.Get(ctx, lastSeenKey, &lastSeenStr)
		if err != nil {
			return "offline", nil
		}
		
		var lastSeen int64
		fmt.Sscanf(lastSeenStr, "%d", &lastSeen)
		lastSeenTime := time.Unix(lastSeen, 0)
		
		if time.Since(lastSeenTime) < 5*time.Minute {
			return "away", nil
		}
		
		return "offline", nil
	}
	
	return status, nil
}

// CreateRoom creates a chat room for multiple users
func (s *RealtimeService) CreateRoom(ctx context.Context, roomID string, userIDs []string) error {
	// Store room membership in Redis
	roomKey := fmt.Sprintf("room:%s:members", roomID)
	
	for _, userID := range userIDs {
		if err := s.redis.AddToSet(ctx, roomKey, userID); err != nil {
			return err
		}
		
		// Add room to user's room list
		userRoomKey := fmt.Sprintf("user:%s:rooms", userID)
		s.redis.AddToSet(ctx, userRoomKey, roomID)
	}
	
	// Notify users about new room
	notification := map[string]interface{}{
		"type":    "room_created",
		"room_id": roomID,
		"members": userIDs,
	}
	
	_ = notification // notification data is used directly in SSE message
	for _, userID := range userIDs {
		s.hub.BroadcastToUser(userID, sse.Message{
			Type:      "room_notification",
			Data:      notification,
			Timestamp: time.Now(),
		})
	}
	
	return nil
}

// BroadcastToRoom sends a message to all users in a room
func (s *RealtimeService) BroadcastToRoom(ctx context.Context, roomID string, senderID string, message string) error {
	// Get room members
	roomKey := fmt.Sprintf("room:%s:members", roomID)
	members, err := s.redis.GetSetMembers(ctx, roomKey)
	if err != nil {
		return err
	}
	
	// Create message
	msg := map[string]interface{}{
		"room_id":   roomID,
		"sender_id": senderID,
		"message":   message,
		"timestamp": time.Now().Unix(),
	}
	
	sseMessage := sse.Message{
		Type:      "room_message",
		Data:      msg,
		Timestamp: time.Now(),
	}
	
	// Send to all members
	for _, memberID := range members {
		s.hub.BroadcastToUser(memberID, sseMessage)
	}
	
	// Store message in Redis for history
	historyKey := fmt.Sprintf("room:%s:history", roomID)
	data, _ := json.Marshal(msg)
	s.redis.AddToList(ctx, historyKey, string(data))
	
	return nil
}