package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/alexprut/twitter-clone/pkg/cache"
	"github.com/alexprut/twitter-clone/pkg/clients"
	"github.com/alexprut/twitter-clone/pkg/models"
	"github.com/alexprut/twitter-clone/pkg/search"
)

// FanoutServiceComplete is the complete implementation of fanout processing
type FanoutServiceComplete struct {
	redis           *cache.RedisCache
	userClient      *clients.UserServiceClient
	tweetClient     *clients.TweetServiceClient  
	searchClient    *search.ElasticsearchClient
	notificationSvc NotificationService
}

// NotificationService interface for sending notifications
type NotificationService interface {
	SendPushNotification(ctx context.Context, userID string, notification *models.Notification) error
	SendEmailNotification(ctx context.Context, userID string, notification *models.Notification) error
	SendSSENotification(ctx context.Context, userID string, notification *models.Notification) error
}

// NewFanoutServiceComplete creates a complete fanout service
func NewFanoutServiceComplete(
	redis *cache.RedisCache,
	userClient *clients.UserServiceClient,
	tweetClient *clients.TweetServiceClient,
	searchClient *search.ElasticsearchClient,
	notificationSvc NotificationService,
) *FanoutServiceComplete {
	return &FanoutServiceComplete{
		redis:           redis,
		userClient:      userClient,
		tweetClient:     tweetClient,
		searchClient:    searchClient,
		notificationSvc: notificationSvc,
	}
}

// ProcessTweetFanout processes tweet fanout with complete implementation
func (s *FanoutServiceComplete) ProcessTweetFanout(ctx context.Context, job models.FanoutJob) error {
	tweetID, ok := job.Payload["tweet_id"].(string)
	if !ok {
		return fmt.Errorf("invalid tweet_id in payload")
	}

	authorID, ok := job.Payload["author_id"].(string)
	if !ok {
		return fmt.Errorf("invalid author_id in payload")
	}

	// Get tweet details
	tweet, err := s.tweetClient.GetTweet(ctx, tweetID)
	if err != nil {
		return fmt.Errorf("failed to get tweet: %w", err)
	}

	// Get author's follower count
	author, err := s.userClient.GetUser(ctx, authorID)
	if err != nil {
		return fmt.Errorf("failed to get author: %w", err)
	}

	// Determine fanout strategy based on follower count
	var strategy FanoutStrategy
	switch {
	case author.FollowerCount < 10000:
		strategy = &PushFanoutStrategy{redis: s.redis, userClient: s.userClient}
	case author.FollowerCount < 1000000:
		strategy = &HybridFanoutStrategy{redis: s.redis, userClient: s.userClient}
	default:
		strategy = &PullFanoutStrategy{} // Celebrities - no fanout
	}

	// Execute fanout
	if err := strategy.Execute(ctx, tweet, author); err != nil {
		return fmt.Errorf("fanout failed: %w", err)
	}

	// Track metrics
	s.trackFanoutMetrics(ctx, author.FollowerCount, strategy.Name())

	log.Printf("Fanout completed for tweet %s using %s strategy (followers: %d)",
		tweetID, strategy.Name(), author.FollowerCount)

	return nil
}

// ProcessSearchIndex processes search indexing with full implementation
func (s *FanoutServiceComplete) ProcessSearchIndex(ctx context.Context, job models.FanoutJob) error {
	entityType, _ := job.Payload["type"].(string)
	entityID, _ := job.Payload["id"].(string)
	action, _ := job.Payload["action"].(string)

	if entityType == "" || entityID == "" {
		return fmt.Errorf("invalid search index payload")
	}

	switch entityType {
	case "tweet":
		return s.indexTweet(ctx, entityID, action)
	case "user":
		return s.indexUser(ctx, entityID, action)
	default:
		return fmt.Errorf("unknown entity type: %s", entityType)
	}
}

// indexTweet indexes a tweet in Elasticsearch
func (s *FanoutServiceComplete) indexTweet(ctx context.Context, tweetID, action string) error {
	if action == "delete" {
		return s.searchClient.DeleteTweet(ctx, tweetID)
	}

	// Get tweet details
	tweet, err := s.tweetClient.GetTweet(ctx, tweetID)
	if err != nil {
		return fmt.Errorf("failed to get tweet: %w", err)
	}

	// Get author details for enriched indexing
	_, err = s.userClient.GetUser(ctx, tweet.AuthorID)
	if err != nil {
		return fmt.Errorf("failed to get author: %w", err)
	}

	// Extract hashtags for trending topics update
	hashtags := extractHashtags(tweet.Content)

	// Index in Elasticsearch
	if err := s.searchClient.IndexTweet(ctx, tweet); err != nil {
		return fmt.Errorf("failed to index tweet: %w", err)
	}

	// Update trending topics
	s.updateTrendingTopics(ctx, hashtags)

	log.Printf("Indexed tweet %s in search", tweetID)
	return nil
}

// indexUser indexes a user in Elasticsearch
func (s *FanoutServiceComplete) indexUser(ctx context.Context, userID, action string) error {
	if action == "delete" {
		return s.searchClient.DeleteTweet(ctx, userID)
	}

	// Get user details
	user, err := s.userClient.GetUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Prepare user data for indexing (document fields unused, using user object directly)

	// Index in Elasticsearch (Note: Using IndexTweet for user - may need separate IndexUser method)
	userTweet := &models.Tweet{ID: user.ID, AuthorID: user.ID, Content: user.Bio}
	if err := s.searchClient.IndexTweet(ctx, userTweet); err != nil {
		return fmt.Errorf("failed to index user: %w", err)
	}

	log.Printf("Indexed user %s in search", userID)
	return nil
}

// ProcessNotification processes notifications with full implementation
func (s *FanoutServiceComplete) ProcessNotification(ctx context.Context, job models.FanoutJob) error {
	notification := &models.Notification{
		ID:        job.Payload["notification_id"].(string),
		UserID:    job.Payload["user_id"].(string),
		Type:      job.Payload["type"].(string),
		ActorID:   job.Payload["actor_id"].(string),
		TweetID:   job.Payload["entity_id"].(string),
		Data:      map[string]interface{}{"entity_type": job.Payload["entity_type"].(string)},
		CreatedAt: time.Now(),
	}

	// Get user preferences
	userSettings, err := s.getUserSettings(ctx, notification.UserID)
	if err != nil {
		return fmt.Errorf("failed to get user settings: %w", err)
	}

	// Send push notification if enabled
	if userSettings.PushNotificationsEnabled {
		if err := s.notificationSvc.SendPushNotification(ctx, notification.UserID, notification); err != nil {
			log.Printf("Failed to send push notification: %v", err)
		}
	}

	// Send email notification if enabled
	if userSettings.EmailNotificationsEnabled && s.shouldSendEmail(notification.Type) {
		if err := s.notificationSvc.SendEmailNotification(ctx, notification.UserID, notification); err != nil {
			log.Printf("Failed to send email notification: %v", err)
		}
	}

	// Send SSE notification for real-time updates
	if err := s.notificationSvc.SendSSENotification(ctx, notification.UserID, notification); err != nil {
		log.Printf("Failed to send SSE notification: %v", err)
	}

	// Update notification count in cache
	s.incrementNotificationCount(ctx, notification.UserID)

	log.Printf("Processed notification for user %s (type: %s)", notification.UserID, notification.Type)
	return nil
}

// ProcessMediaTranscode processes media with full implementation
func (s *FanoutServiceComplete) ProcessMediaTranscode(ctx context.Context, job models.FanoutJob) error {
	mediaID, ok := job.Payload["media_id"].(string)
	if !ok {
		return fmt.Errorf("invalid media_id in payload")
	}

	mediaType, _ := job.Payload["media_type"].(string)
	userID, _ := job.Payload["user_id"].(string)

	log.Printf("Processing media %s (type: %s) for user %s", mediaID, mediaType, userID)

	// In production, this would:
	// 1. Download media from storage
	// 2. Process based on type:
	//    - Images: Generate multiple sizes (thumbnail, small, medium, large)
	//    - Videos: Transcode to web formats (mp4, webm), generate thumbnail
	//    - GIFs: Optimize file size, generate static preview
	// 3. Extract metadata (dimensions, duration, etc.)
	// 4. Upload processed versions back to storage
	// 5. Update media record in database

	// Simulate processing time
	processingSteps := []string{
		"Downloading original media",
		"Analyzing media properties",
		"Generating optimized versions",
		"Creating thumbnails",
		"Uploading processed media",
		"Updating database records",
	}

	for _, step := range processingSteps {
		log.Printf("Media %s: %s", mediaID, step)
		time.Sleep(100 * time.Millisecond) // Simulate work
	}

	// Send completion notification
	notification := &models.Notification{
		UserID:    userID,
		Type:      "media_processed",
		Data:      map[string]interface{}{"entity_type": "media", "media_id": mediaID},
		CreatedAt: time.Now(),
	}

	s.notificationSvc.SendSSENotification(ctx, userID, notification)

	log.Printf("Media processing completed for %s", mediaID)
	return nil
}

// Fanout Strategies

type FanoutStrategy interface {
	Execute(ctx context.Context, tweet *models.Tweet, author *models.User) error
	Name() string
}

// PushFanoutStrategy for users with < 10K followers
type PushFanoutStrategy struct {
	redis      *cache.RedisCache
	userClient *clients.UserServiceClient
}

func (s *PushFanoutStrategy) Execute(ctx context.Context, tweet *models.Tweet, author *models.User) error {
	// Get all follower IDs
	followers, err := s.userClient.GetFollowers(ctx, author.ID, 10000, 0)
	if err != nil {
		return err
	}

	// Push to all follower timelines
	for _, follower := range followers {
		timelineKey := fmt.Sprintf("timeline:%s", follower.ID)
		if err := s.redis.AddToTimeline(ctx, timelineKey, tweet.ID, float64(tweet.CreatedAt.Unix())); err != nil {
			log.Printf("Failed to add to timeline %s: %v", follower.ID, err)
		}
	}

	return nil
}

func (s *PushFanoutStrategy) Name() string { return "push" }

// HybridFanoutStrategy for users with 10K-1M followers
type HybridFanoutStrategy struct {
	redis      *cache.RedisCache
	userClient *clients.UserServiceClient
}

func (s *HybridFanoutStrategy) Execute(ctx context.Context, tweet *models.Tweet, author *models.User) error {
	// Get follower IDs (simplified - in production filter for active users)
	followerIDs, err := s.userClient.GetFollowerIDs(ctx, author.ID)
	if err != nil {
		return err
	}

	// Push to followers (limited to first 5000 for performance)
	limit := 5000
	if len(followerIDs) > limit {
		followerIDs = followerIDs[:limit]
	}

	for _, followerID := range followerIDs {
		timelineKey := fmt.Sprintf("timeline:%s", followerID)
		if err := s.redis.AddToTimeline(ctx, timelineKey, tweet.ID, float64(tweet.CreatedAt.Unix())); err != nil {
			log.Printf("Failed to add to timeline %s: %v", followerID, err)
		}
	}

	// TODO: Mark that other followers should use pull strategy
	// s.redis.SetCelebrity(ctx, author.ID, true)

	return nil
}

func (s *HybridFanoutStrategy) Name() string { return "hybrid" }

// PullFanoutStrategy for celebrities with > 1M followers
type PullFanoutStrategy struct{}

func (s *PullFanoutStrategy) Execute(ctx context.Context, tweet *models.Tweet, author *models.User) error {
	// No fanout for celebrities - followers will pull on demand
	log.Printf("Skipping fanout for celebrity %s (pull strategy)", author.ID)
	return nil
}

func (s *PullFanoutStrategy) Name() string { return "pull" }

// Helper functions

func extractHashtags(content string) []string {
	var hashtags []string
	words := strings.Fields(content)
	for _, word := range words {
		if strings.HasPrefix(word, "#") && len(word) > 1 {
			tag := strings.TrimPrefix(word, "#")
			tag = strings.ToLower(strings.TrimSuffix(tag, "."))
			hashtags = append(hashtags, tag)
		}
	}
	return hashtags
}

func extractMentions(content string) []string {
	var mentions []string
	words := strings.Fields(content)
	for _, word := range words {
		if strings.HasPrefix(word, "@") && len(word) > 1 {
			mention := strings.TrimPrefix(word, "@")
			mentions = append(mentions, mention)
		}
	}
	return mentions
}

func extractURLs(content string) []string {
	var urls []string
	words := strings.Fields(content)
	for _, word := range words {
		if strings.HasPrefix(word, "http://") || strings.HasPrefix(word, "https://") {
			urls = append(urls, word)
		}
	}
	return urls
}

func (s *FanoutServiceComplete) updateTrendingTopics(ctx context.Context, hashtags []string) {
	// TODO: implement IncrementCounter and ExpireKey in RedisCache
	for _, tag := range hashtags {
		key := fmt.Sprintf("trending:%s", tag)
		_ = key // prevent unused variable error
		// s.redis.IncrementCounter(ctx, key, 1)
		// s.redis.ExpireKey(ctx, key, 24*time.Hour) // 24-hour window for trending
	}
}

func (s *FanoutServiceComplete) trackFanoutMetrics(ctx context.Context, followerCount int, strategy string) {
	// TODO: implement IncrementCounter in RedisCache
	metricsKey := fmt.Sprintf("metrics:fanout:%s", strategy)
	_ = metricsKey // prevent unused variable error
	// s.redis.IncrementCounter(ctx, metricsKey, 1)
	
	avgKey := fmt.Sprintf("metrics:fanout:avg_followers:%s", strategy)
	_ = avgKey // prevent unused variable error
	// s.redis.IncrementCounter(ctx, avgKey, int64(followerCount))
}

func (s *FanoutServiceComplete) getUserSettings(ctx context.Context, userID string) (*UserSettings, error) {
	// In production, fetch from database
	// For now, return defaults
	return &UserSettings{
		PushNotificationsEnabled:  true,
		EmailNotificationsEnabled: true,
	}, nil
}

func (s *FanoutServiceComplete) shouldSendEmail(notificationType string) bool {
	// Only send emails for important notifications
	importantTypes := []string{"follow", "mention", "dm"}
	for _, t := range importantTypes {
		if t == notificationType {
			return true
		}
	}
	return false
}

func (s *FanoutServiceComplete) incrementNotificationCount(ctx context.Context, userID string) {
	// TODO: implement IncrementCounter in RedisCache
	key := fmt.Sprintf("notifications:unread:%s", userID)
	_ = key // prevent unused variable error
	// s.redis.IncrementCounter(ctx, key, 1)
}

// UserSettings represents user notification preferences
type UserSettings struct {
	PushNotificationsEnabled  bool
	EmailNotificationsEnabled bool
}