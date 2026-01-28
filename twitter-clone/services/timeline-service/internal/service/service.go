package service

import (
	"context"
	"fmt"

	"github.com/alexprut/twitter-clone/pkg/cache"
	"github.com/alexprut/twitter-clone/pkg/clients"
	"github.com/alexprut/twitter-clone/pkg/models"
)

const (
	DefaultTimelineLimit = 20
	MaxTimelineLimit     = 100
	TimelineMaxSize      = 800 // Max tweets to keep in timeline
)

type TimelineService struct {
	cache       *cache.RedisCache
	tweetClient *clients.TweetServiceClient
	userClient  *clients.UserServiceClient
}

func NewTimelineService(
	cache *cache.RedisCache,
	tweetClient *clients.TweetServiceClient,
	userClient *clients.UserServiceClient,
) *TimelineService {
	return &TimelineService{
		cache:       cache,
		tweetClient: tweetClient,
		userClient:  userClient,
	}
}

// GetHomeTimeline returns a user's home timeline
func (s *TimelineService) GetHomeTimeline(ctx context.Context, userID string, limit, offset int) (*models.TimelineResponse, error) {
	if limit <= 0 {
		limit = DefaultTimelineLimit
	}
	if limit > MaxTimelineLimit {
		limit = MaxTimelineLimit
	}

	// Get tweet IDs from Redis
	tweetIDs, err := s.cache.GetHomeTimeline(ctx, userID, int64(offset), int64(limit+1))
	if err != nil {
		return nil, fmt.Errorf("get timeline: %w", err)
	}

	hasMore := len(tweetIDs) > limit
	if hasMore {
		tweetIDs = tweetIDs[:limit]
	}

	// Fetch tweets from tweet service
	var tweets []models.Tweet
	if len(tweetIDs) > 0 && s.tweetClient != nil {
		tweets, err = s.tweetClient.GetTweets(ctx, tweetIDs)
		if err != nil {
			return nil, fmt.Errorf("get tweets: %w", err)
		}
	}

	resp := &models.TimelineResponse{
		Tweets:  tweets,
		HasMore: hasMore,
	}
	if hasMore {
		resp.NextCursor = fmt.Sprintf("%d", offset+limit)
	}

	return resp, nil
}

// GetUserTimeline returns a user's own tweets timeline
func (s *TimelineService) GetUserTimeline(ctx context.Context, userID string, limit, offset int) (*models.TimelineResponse, error) {
	if limit <= 0 {
		limit = DefaultTimelineLimit
	}
	if limit > MaxTimelineLimit {
		limit = MaxTimelineLimit
	}

	// Get tweet IDs from Redis
	tweetIDs, err := s.cache.GetUserTimeline(ctx, userID, int64(offset), int64(limit+1))
	if err != nil {
		return nil, fmt.Errorf("get timeline: %w", err)
	}

	hasMore := len(tweetIDs) > limit
	if hasMore {
		tweetIDs = tweetIDs[:limit]
	}

	// Fetch tweets from tweet service
	var tweets []models.Tweet
	if len(tweetIDs) > 0 && s.tweetClient != nil {
		tweets, err = s.tweetClient.GetTweets(ctx, tweetIDs)
		if err != nil {
			return nil, fmt.Errorf("get tweets: %w", err)
		}
	}

	resp := &models.TimelineResponse{
		Tweets:  tweets,
		HasMore: hasMore,
	}
	if hasMore {
		resp.NextCursor = fmt.Sprintf("%d", offset+limit)
	}

	return resp, nil
}

// AddToTimeline adds a tweet to a user's home timeline
func (s *TimelineService) AddToTimeline(ctx context.Context, userID, tweetID string, score float64) error {
	if err := s.cache.AddToTimeline(ctx, userID, tweetID, score); err != nil {
		return fmt.Errorf("add to timeline: %w", err)
	}

	// Trim timeline to max size
	key := cache.PrefixTimeline + "home:" + userID
	s.cache.TrimTimeline(ctx, key, TimelineMaxSize)

	return nil
}

// AddToUserTimeline adds a tweet to a user's own tweets timeline
func (s *TimelineService) AddToUserTimeline(ctx context.Context, userID, tweetID string, score float64) error {
	if err := s.cache.AddToUserTimeline(ctx, userID, tweetID, score); err != nil {
		return fmt.Errorf("add to user timeline: %w", err)
	}

	key := cache.PrefixTimeline + "user:" + userID
	s.cache.TrimTimeline(ctx, key, TimelineMaxSize)

	return nil
}

// RemoveFromTimeline removes a tweet from a user's home timeline
func (s *TimelineService) RemoveFromTimeline(ctx context.Context, userID, tweetID string) error {
	key := cache.PrefixTimeline + "home:" + userID
	return s.cache.RemoveFromTimeline(ctx, key, tweetID)
}

// RemoveFromUserTimeline removes a tweet from a user's own tweets timeline
func (s *TimelineService) RemoveFromUserTimeline(ctx context.Context, userID, tweetID string) error {
	key := cache.PrefixTimeline + "user:" + userID
	return s.cache.RemoveFromTimeline(ctx, key, tweetID)
}
