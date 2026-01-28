package service

import (
	"context"
	"log"
	"time"

	"github.com/alexprut/twitter-clone/pkg/cache"
	"github.com/alexprut/twitter-clone/pkg/clients"
	"github.com/alexprut/twitter-clone/pkg/models"
)

const (
	// Fan-out thresholds
	SmallFollowerThreshold  = 10000   // Push to all followers
	MediumFollowerThreshold = 1000000 // Push to active followers only
	// Above 1M: Pull-based (don't fan out)

	// Batch size for fan-out
	FanoutBatchSize = 1000

	// Active follower window
	ActiveDaysWindow = 7
)

type FanoutService struct {
	cache       *cache.RedisCache
	userClient  *clients.UserServiceClient
	tweetClient *clients.TweetServiceClient
}

func NewFanoutService(
	cache *cache.RedisCache,
	userClient *clients.UserServiceClient,
	tweetClient *clients.TweetServiceClient,
) *FanoutService {
	return &FanoutService{
		cache:       cache,
		userClient:  userClient,
		tweetClient: tweetClient,
	}
}

// ProcessFanout processes a fanout job from the queue
func (s *FanoutService) ProcessFanout(ctx context.Context, job models.FanoutJob) error {
	log.Printf("Processing fanout job: %s for tweet %s by author %s", job.ID, job.TweetID, job.AuthorID)

	followerCount := 0
	if count, ok := job.Payload["follower_count"].(float64); ok {
		followerCount = int(count)
	}

	// Get the tweet to use its timestamp as score
	var score float64
	if s.tweetClient != nil {
		tweet, err := s.tweetClient.GetTweet(ctx, job.TweetID)
		if err != nil {
			log.Printf("Warning: could not get tweet %s: %v", job.TweetID, err)
			score = float64(time.Now().UnixNano())
		} else {
			score = float64(tweet.CreatedAt.UnixNano())
		}
	} else {
		score = float64(time.Now().UnixNano())
	}

	// Determine fanout strategy based on follower count
	if followerCount > MediumFollowerThreshold {
		// Celebrity account - don't fan out (pull-based)
		log.Printf("Skipping fanout for celebrity account %s (%d followers)", job.AuthorID, followerCount)
		return nil
	}

	// Get follower IDs
	var followerIDs []string
	var err error

	if s.userClient != nil {
		followerIDs, err = s.userClient.GetFollowerIDs(ctx, job.AuthorID)
		if err != nil {
			return err
		}
	}

	if len(followerIDs) == 0 {
		log.Printf("No followers to fan out to for author %s", job.AuthorID)
		return nil
	}

	log.Printf("Fanning out tweet %s to %d followers", job.TweetID, len(followerIDs))

	// Process in batches
	for i := 0; i < len(followerIDs); i += FanoutBatchSize {
		end := i + FanoutBatchSize
		if end > len(followerIDs) {
			end = len(followerIDs)
		}

		batch := followerIDs[i:end]
		s.fanoutBatch(ctx, batch, job.TweetID, score)
	}

	log.Printf("Completed fanout for tweet %s", job.TweetID)
	return nil
}

func (s *FanoutService) fanoutBatch(ctx context.Context, followerIDs []string, tweetID string, score float64) {
	for _, followerID := range followerIDs {
		if err := s.cache.AddToTimeline(ctx, followerID, tweetID, score); err != nil {
			log.Printf("Error adding tweet %s to timeline of user %s: %v", tweetID, followerID, err)
			continue
		}
	}
}

// ProcessSearchIndex processes a search indexing job
func (s *FanoutService) ProcessSearchIndex(ctx context.Context, job models.FanoutJob) error {
	log.Printf("Processing search index job: %s for tweet %s", job.ID, job.TweetID)

	// The actual indexing is handled by the search service
	// This is just a placeholder for additional processing if needed
	return nil
}

// ProcessNotification processes a notification job
func (s *FanoutService) ProcessNotification(ctx context.Context, job models.FanoutJob) error {
	log.Printf("Processing notification job: %s", job.ID)

	// Extract notification details from payload
	userID, _ := job.Payload["user_id"].(string)
	notifType, _ := job.Payload["notif_type"].(string)
	actorID, _ := job.Payload["actor_id"].(string)

	log.Printf("Notification: type=%s, user=%s, actor=%s, tweet=%s",
		notifType, userID, actorID, job.TweetID)

	// The actual notification storage is handled by the notification service
	// This worker could send push notifications, emails, etc.
	return nil
}

// ProcessMediaTranscode processes a media transcoding job
func (s *FanoutService) ProcessMediaTranscode(ctx context.Context, job models.FanoutJob) error {
	log.Printf("Processing media transcode job: %s", job.ID)

	mediaID, _ := job.Payload["media_id"].(string)
	contentType, _ := job.Payload["content_type"].(string)

	log.Printf("Media processing: id=%s, type=%s", mediaID, contentType)

	// The actual media processing would happen here
	// For now, this is a placeholder
	return nil
}
