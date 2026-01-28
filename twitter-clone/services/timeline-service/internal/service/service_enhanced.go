package service

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"

	"github.com/alexprut/twitter-clone/pkg/cache"
	"github.com/alexprut/twitter-clone/pkg/clients"
	"github.com/alexprut/twitter-clone/pkg/models"
)

// TimelineServiceEnhanced provides enhanced timeline generation with fallback strategies
type TimelineServiceEnhanced struct {
	redis       *cache.RedisCache
	userClient  *clients.UserServiceClient
	tweetClient *clients.TweetServiceClient
}

// NewTimelineServiceEnhanced creates an enhanced timeline service
func NewTimelineServiceEnhanced(redis *cache.RedisCache, userClient *clients.UserServiceClient, tweetClient *clients.TweetServiceClient) *TimelineServiceEnhanced {
	return &TimelineServiceEnhanced{
		redis:       redis,
		userClient:  userClient,
		tweetClient: tweetClient,
	}
}

// TimelineStrategy defines the strategy for timeline generation
type TimelineStrategy int

const (
	StrategyPush TimelineStrategy = iota  // Pre-computed timeline in Redis
	StrategyPull                          // On-demand from database
	StrategyHybrid                        // Mix of push and pull
)

// GetHomeTimeline returns the home timeline with fallback strategies
func (s *TimelineServiceEnhanced) GetHomeTimeline(ctx context.Context, userID string, limit int, cursor string) (*models.TimelineResponse, error) {
	// Determine strategy based on user characteristics
	strategy := s.determineStrategy(ctx, userID)
	
	switch strategy {
	case StrategyPush:
		return s.getPushTimeline(ctx, userID, limit, cursor)
	case StrategyPull:
		return s.getPullTimeline(ctx, userID, limit, cursor)
	case StrategyHybrid:
		return s.getHybridTimeline(ctx, userID, limit, cursor)
	default:
		return s.getPushTimeline(ctx, userID, limit, cursor)
	}
}

// determineStrategy determines the best strategy for timeline generation
func (s *TimelineServiceEnhanced) determineStrategy(ctx context.Context, userID string) TimelineStrategy {
	// Check if user has cached timeline
	timelineKey := fmt.Sprintf("timeline:%s", userID)
	exists, _ := s.redis.Exists(ctx, timelineKey)
	
	if exists {
		// Check timeline freshness
		size := int64(0) // TODO: implement GetTimelineSize in RedisCache
		if size > 0 {
			return StrategyPush
		}
	}
	
	// Check if user follows celebrities
	user, err := s.userClient.GetUser(ctx, userID)
	if err == nil && user.FollowingCount > 500 {
		// Heavy follower, use hybrid
		return StrategyHybrid
	}
	
	// Default to pull for new or inactive users
	return StrategyPull
}

// getPushTimeline gets pre-computed timeline from Redis
func (s *TimelineServiceEnhanced) getPushTimeline(ctx context.Context, userID string, limit int, cursor string) (*models.TimelineResponse, error) {
	_ = fmt.Sprintf("timeline:%s", userID) // TODO: use when GetTimelineRange is implemented
	
	// Get tweet IDs from Redis
	offset := 0
	if cursor != "" {
		// Parse cursor to get offset
		fmt.Sscanf(cursor, "%d", &offset)
	}
	
	tweetIDs := []string{} // TODO: implement GetTimelineRange in RedisCache
	err := fmt.Errorf("not implemented")
	if err != nil {
		// Fallback to pull strategy
		log.Printf("Redis error, falling back to pull strategy: %v", err)
		return s.getPullTimeline(ctx, userID, limit, cursor)
	}
	
	if len(tweetIDs) == 0 {
		// Timeline empty, try to rebuild
		log.Printf("Timeline empty for user %s, rebuilding", userID)
		return s.rebuildAndGetTimeline(ctx, userID, limit, cursor)
	}
	
	// Fetch tweet details
	tweets, err := s.tweetClient.GetTweetsBatch(ctx, tweetIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tweets: %w", err)
	}
	
	// Prepare response
	hasMore := len(tweetIDs) == limit
	nextCursor := ""
	if hasMore {
		nextCursor = fmt.Sprintf("%d", offset+limit)
	}
	
	return &models.TimelineResponse{
		Tweets:     tweets,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// getPullTimeline generates timeline on-demand from database
func (s *TimelineServiceEnhanced) getPullTimeline(ctx context.Context, userID string, limit int, cursor string) (*models.TimelineResponse, error) {
	// Get user's following list
	following, err := s.userClient.GetFollowing(ctx, userID, 1000, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get following: %w", err)
	}
	
	if len(following) == 0 {
		// User follows nobody, return empty timeline
		return &models.TimelineResponse{
			Tweets:  []models.Tweet{},
			HasMore: false,
		}, nil
	}
	
	// Get recent tweets from followed users
	var allTweets []models.Tweet
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	// Limit concurrent requests
	semaphore := make(chan struct{}, 10)
	
	for _, followedUser := range following {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			// Get user's recent tweets
			tweets, err := s.tweetClient.GetUserTweets(ctx, userID, 20, 0)
			if err != nil {
				log.Printf("Failed to get tweets for user %s: %v", userID, err)
				return
			}
			
			mu.Lock()
			allTweets = append(allTweets, tweets...)
			mu.Unlock()
		}(followedUser.ID)
	}
	
	wg.Wait()
	
	// Sort tweets by timestamp
	sort.Slice(allTweets, func(i, j int) bool {
		return allTweets[i].CreatedAt.After(allTweets[j].CreatedAt)
	})
	
	// Apply pagination
	offset := 0
	if cursor != "" {
		fmt.Sscanf(cursor, "%d", &offset)
	}
	
	end := offset + limit
	if end > len(allTweets) {
		end = len(allTweets)
	}
	
	paginatedTweets := allTweets[offset:end]
	
	// Cache the timeline for future use
	go s.cacheTimeline(context.Background(), userID, allTweets)
	
	hasMore := end < len(allTweets)
	nextCursor := ""
	if hasMore {
		nextCursor = fmt.Sprintf("%d", end)
	}
	
	return &models.TimelineResponse{
		Tweets:     paginatedTweets,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// getHybridTimeline combines push and pull strategies
func (s *TimelineServiceEnhanced) getHybridTimeline(ctx context.Context, userID string, limit int, cursor string) (*models.TimelineResponse, error) {
	_ = fmt.Sprintf("timeline:%s", userID) // TODO: use when GetTimelineRange is implemented
	
	// Get cached timeline
	offset := 0
	if cursor != "" {
		fmt.Sscanf(cursor, "%d", &offset)
	}
	
	cachedIDs := []string{} // TODO: implement GetTimelineRange in RedisCache
	
	// Get tweets from celebrities (pull)
	celebrities, err := s.getCelebritiesFollowed(ctx, userID)
	if err != nil {
		celebrities = []string{}
	}
	
	var celebrityTweets []models.Tweet
	for _, celebID := range celebrities {
		tweets, err := s.tweetClient.GetUserTweets(ctx, celebID, 5, 0)
		if err == nil {
			celebrityTweets = append(celebrityTweets, tweets...)
		}
	}
	
	// Fetch cached tweets
	var cachedTweets []models.Tweet
	if len(cachedIDs) > 0 {
		cachedTweets, _ = s.tweetClient.GetTweetsBatch(ctx, cachedIDs)
	}
	
	// Merge and deduplicate
	tweetMap := make(map[string]models.Tweet)
	for _, tweet := range cachedTweets {
		tweetMap[tweet.ID] = tweet
	}
	for _, tweet := range celebrityTweets {
		tweetMap[tweet.ID] = tweet
	}
	
	// Convert to slice and sort
	var allTweets []models.Tweet
	for _, tweet := range tweetMap {
		allTweets = append(allTweets, tweet)
	}
	
	sort.Slice(allTweets, func(i, j int) bool {
		return allTweets[i].CreatedAt.After(allTweets[j].CreatedAt)
	})
	
	// Apply limit
	if len(allTweets) > limit {
		allTweets = allTweets[:limit]
	}
	
	hasMore := len(allTweets) == limit
	nextCursor := ""
	if hasMore {
		nextCursor = fmt.Sprintf("%d", offset+limit)
	}
	
	return &models.TimelineResponse{
		Tweets:     allTweets,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// rebuildAndGetTimeline rebuilds the timeline from scratch
func (s *TimelineServiceEnhanced) rebuildAndGetTimeline(ctx context.Context, userID string, limit int, cursor string) (*models.TimelineResponse, error) {
	log.Printf("Rebuilding timeline for user %s", userID)
	
	// Get following list
	following, err := s.userClient.GetFollowing(ctx, userID, 500, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get following: %w", err)
	}
	
	timelineKey := fmt.Sprintf("timeline:%s", userID)
	
	// Rebuild timeline
	for _, user := range following {
		// Get recent tweets
		tweets, err := s.tweetClient.GetUserTweets(ctx, user.ID, 10, 0)
		if err != nil {
			continue
		}
		
		// Add to timeline
		for _, tweet := range tweets {
			s.redis.AddToTimeline(ctx, timelineKey, tweet.ID, float64(tweet.CreatedAt.Unix()))
		}
	}
	
	// Set expiration
	// TODO: implement ExpireKey in RedisCache
	
	// Now get the timeline
	return s.getPushTimeline(ctx, userID, limit, cursor)
}

// cacheTimeline caches timeline in Redis
func (s *TimelineServiceEnhanced) cacheTimeline(ctx context.Context, userID string, tweets []models.Tweet) {
	timelineKey := fmt.Sprintf("timeline:%s", userID)
	
	// Clear existing timeline
	s.redis.Delete(ctx, timelineKey)
	
	// Add tweets to timeline
	for _, tweet := range tweets {
		s.redis.AddToTimeline(ctx, timelineKey, tweet.ID, float64(tweet.CreatedAt.Unix()))
	}
	
	// Trim to reasonable size (keep last 800 tweets)
	s.redis.TrimTimeline(ctx, timelineKey, 800)
	
	// Set expiration
	// TODO: implement ExpireKey in RedisCache
}

// getCelebritiesFollowed returns celebrity IDs that user follows
func (s *TimelineServiceEnhanced) getCelebritiesFollowed(ctx context.Context, userID string) ([]string, error) {
	// Get all following
	following, err := s.userClient.GetFollowing(ctx, userID, 1000, 0)
	if err != nil {
		return nil, err
	}
	
	var celebrities []string
	for _, user := range following {
		if user.FollowerCount > 100000 { // Celebrity threshold
			celebrities = append(celebrities, user.ID)
		}
	}
	
	return celebrities, nil
}

// GetUserTimeline returns a specific user's timeline
func (s *TimelineServiceEnhanced) GetUserTimeline(ctx context.Context, userID string, limit int, cursor string) (*models.TimelineResponse, error) {
	offset := 0
	if cursor != "" {
		fmt.Sscanf(cursor, "%d", &offset)
	}
	
	// Get user's tweets
	tweets, err := s.tweetClient.GetUserTweets(ctx, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get user tweets: %w", err)
	}
	
	hasMore := len(tweets) == limit
	nextCursor := ""
	if hasMore {
		nextCursor = fmt.Sprintf("%d", offset+limit)
	}
	
	return &models.TimelineResponse{
		Tweets:     tweets,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// AddToTimeline adds a tweet to followers' timelines (for fanout)
func (s *TimelineServiceEnhanced) AddToTimeline(ctx context.Context, userID, tweetID string, timestamp int64) error {
	timelineKey := fmt.Sprintf("timeline:%s", userID)
	
	// Add tweet to timeline
	if err := s.redis.AddToTimeline(ctx, timelineKey, tweetID, float64(timestamp)); err != nil {
		return err
	}
	
	// Trim timeline to max size
	return s.redis.TrimTimeline(ctx, timelineKey, 800)
}

// RemoveFromTimeline removes a tweet from timelines (for deletion)
func (s *TimelineServiceEnhanced) RemoveFromTimeline(ctx context.Context, userID, tweetID string) error {
	timelineKey := fmt.Sprintf("timeline:%s", userID)
	return s.redis.RemoveFromTimeline(ctx, timelineKey, tweetID)
}

// GetTrendingTimeline returns trending tweets
func (s *TimelineServiceEnhanced) GetTrendingTimeline(ctx context.Context, limit int, cursor string) (*models.TimelineResponse, error) {
	// Get trending tweet IDs from cache
	_ = "timeline:trending" // TODO: use when GetTimelineRange is implemented
	
	offset := 0
	if cursor != "" {
		fmt.Sscanf(cursor, "%d", &offset)
	}
	
	tweetIDs := []string{} // TODO: implement GetTimelineRange in RedisCache
	err := fmt.Errorf("not implemented")
	if err != nil || len(tweetIDs) == 0 {
		// Fallback: get popular tweets from last 24 hours
		return s.getPopularTweets(ctx, limit, cursor)
	}
	
	// Fetch tweet details
	tweets, err := s.tweetClient.GetTweetsBatch(ctx, tweetIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tweets: %w", err)
	}
	
	hasMore := len(tweets) == limit
	nextCursor := ""
	if hasMore {
		nextCursor = fmt.Sprintf("%d", offset+limit)
	}
	
	return &models.TimelineResponse{
		Tweets:     tweets,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// getPopularTweets gets popular tweets based on engagement
func (s *TimelineServiceEnhanced) getPopularTweets(ctx context.Context, limit int, cursor string) (*models.TimelineResponse, error) {
	// This would query tweets ordered by engagement score
	// For now, return empty
	return &models.TimelineResponse{
		Tweets:  []models.Tweet{},
		HasMore: false,
	}, nil
}