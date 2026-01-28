package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/alexprut/twitter-clone/pkg/cache"
	"github.com/alexprut/twitter-clone/pkg/clients"
	"github.com/alexprut/twitter-clone/pkg/models"
	"github.com/alexprut/twitter-clone/pkg/queue"
	"github.com/alexprut/twitter-clone/pkg/search"
	"github.com/alexprut/twitter-clone/services/tweet-service/internal/repository"
)

const MaxTweetLength = 280

type TweetService struct {
	repo       *repository.TweetRepository
	cache      *cache.RedisCache
	search     *search.ElasticsearchClient
	queue      *queue.RabbitMQ
	userClient *clients.UserServiceClient
}

func NewTweetService(
	repo *repository.TweetRepository,
	cache *cache.RedisCache,
	search *search.ElasticsearchClient,
	queue *queue.RabbitMQ,
	userClient *clients.UserServiceClient,
) *TweetService {
	return &TweetService{
		repo:       repo,
		cache:      cache,
		search:     search,
		queue:      queue,
		userClient: userClient,
	}
}

func (s *TweetService) CreateTweet(ctx context.Context, authorID string, req models.CreateTweetRequest) (*models.Tweet, error) {
	if len(req.Content) == 0 {
		return nil, fmt.Errorf("tweet content cannot be empty")
	}
	if len(req.Content) > MaxTweetLength {
		return nil, fmt.Errorf("tweet content exceeds %d characters", MaxTweetLength)
	}

	tweet := &models.Tweet{
		ID:        uuid.New().String(),
		AuthorID:  authorID,
		Content:   req.Content,
		MediaIDs:  req.MediaIDs,
		ReplyToID: req.ReplyToID,
		CreatedAt: time.Now(),
	}

	if err := s.repo.Create(ctx, tweet); err != nil {
		return nil, fmt.Errorf("create tweet: %w", err)
	}

	// If this is a reply, increment reply count on parent
	if tweet.ReplyToID != "" {
		go s.repo.IncrementReplyCount(context.Background(), tweet.ReplyToID)
	}

	// Add to user's timeline in Redis
	if s.cache != nil {
		score := float64(tweet.CreatedAt.UnixNano())
		s.cache.AddToUserTimeline(ctx, authorID, tweet.ID, score)
	}

	// Queue fanout job
	if s.queue != nil {
		followerCount := 0
		if s.userClient != nil {
			if count, err := s.getFollowerCount(ctx, authorID); err == nil {
				followerCount = count
			}
		}
		s.queue.PublishFanout(ctx, tweet.ID, authorID, followerCount)

		// Queue search indexing
		s.queue.PublishSearchIndex(ctx, tweet.ID, tweet.Content)
	}

	// Index in Elasticsearch directly (for immediate searchability)
	if s.search != nil {
		go s.search.IndexTweet(context.Background(), tweet)
	}

	return tweet, nil
}

func (s *TweetService) GetTweet(ctx context.Context, tweetID string) (*models.Tweet, error) {
	tweet, err := s.repo.GetByID(ctx, tweetID)
	if err != nil {
		return nil, fmt.Errorf("get tweet: %w", err)
	}

	// Optionally populate author info
	if s.userClient != nil {
		if author, err := s.userClient.GetUser(ctx, tweet.AuthorID); err == nil {
			tweet.Author = author
		}
	}

	return tweet, nil
}

func (s *TweetService) DeleteTweet(ctx context.Context, tweetID, userID string) error {
	// Verify ownership
	tweet, err := s.repo.GetByID(ctx, tweetID)
	if err != nil {
		return fmt.Errorf("get tweet: %w", err)
	}

	if tweet.AuthorID != userID {
		return fmt.Errorf("not authorized to delete this tweet")
	}

	if err := s.repo.Delete(ctx, tweetID); err != nil {
		return fmt.Errorf("delete tweet: %w", err)
	}

	// Remove from user's timeline
	if s.cache != nil {
		s.cache.RemoveFromTimeline(ctx, cache.PrefixTimeline+"user:"+userID, tweetID)
	}

	// Remove from search index
	if s.search != nil {
		go s.search.DeleteTweet(context.Background(), tweetID)
	}

	return nil
}

func (s *TweetService) GetTweetsByAuthor(ctx context.Context, authorID string, limit, offset int) ([]models.Tweet, bool, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	tweets, err := s.repo.GetByAuthor(ctx, authorID, limit+1, offset)
	if err != nil {
		return nil, false, fmt.Errorf("get tweets: %w", err)
	}

	hasMore := len(tweets) > limit
	if hasMore {
		tweets = tweets[:limit]
	}

	return tweets, hasMore, nil
}

func (s *TweetService) GetReplies(ctx context.Context, tweetID string, limit, offset int) ([]models.Tweet, bool, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	tweets, err := s.repo.GetReplies(ctx, tweetID, limit+1, offset)
	if err != nil {
		return nil, false, fmt.Errorf("get replies: %w", err)
	}

	hasMore := len(tweets) > limit
	if hasMore {
		tweets = tweets[:limit]
	}

	return tweets, hasMore, nil
}

func (s *TweetService) BatchGetTweets(ctx context.Context, tweetIDs []string) ([]models.Tweet, error) {
	return s.repo.BatchGet(ctx, tweetIDs)
}

// Like operations

func (s *TweetService) Like(ctx context.Context, userID, tweetID string) error {
	// Check if already liked
	isLiked, err := s.repo.IsLiked(ctx, userID, tweetID)
	if err != nil {
		return fmt.Errorf("check like: %w", err)
	}
	if isLiked {
		return nil // Already liked
	}

	if err := s.repo.Like(ctx, userID, tweetID); err != nil {
		return fmt.Errorf("like: %w", err)
	}

	// Increment cached counter
	if s.cache != nil {
		s.cache.IncrCounter(ctx, cache.PrefixLikes+tweetID)
	}

	// Send notification to tweet author
	if s.queue != nil {
		tweet, err := s.repo.GetByID(ctx, tweetID)
		if err == nil && tweet.AuthorID != userID {
			s.queue.PublishNotification(ctx, tweet.AuthorID, "like", userID, tweetID)
		}
	}

	return nil
}

func (s *TweetService) Unlike(ctx context.Context, userID, tweetID string) error {
	if err := s.repo.Unlike(ctx, userID, tweetID); err != nil {
		return fmt.Errorf("unlike: %w", err)
	}

	// Decrement cached counter
	if s.cache != nil {
		s.cache.DecrCounter(ctx, cache.PrefixLikes+tweetID)
	}

	return nil
}

func (s *TweetService) IsLiked(ctx context.Context, userID, tweetID string) (bool, error) {
	return s.repo.IsLiked(ctx, userID, tweetID)
}

// Retweet operations

func (s *TweetService) Retweet(ctx context.Context, userID, tweetID string) (*models.Tweet, error) {
	// Check if already retweeted
	isRetweeted, err := s.repo.IsRetweeted(ctx, userID, tweetID)
	if err != nil {
		return nil, fmt.Errorf("check retweet: %w", err)
	}
	if isRetweeted {
		return nil, nil // Already retweeted
	}

	// Get original tweet
	originalTweet, err := s.repo.GetByID(ctx, tweetID)
	if err != nil {
		return nil, fmt.Errorf("get tweet: %w", err)
	}

	// Create retweet record
	if err := s.repo.Retweet(ctx, userID, tweetID); err != nil {
		return nil, fmt.Errorf("retweet: %w", err)
	}

	// Create a retweet entry in tweets table
	retweet := &models.Tweet{
		ID:          uuid.New().String(),
		AuthorID:    userID,
		Content:     "",
		RetweetOfID: tweetID,
		CreatedAt:   time.Now(),
	}

	if err := s.repo.Create(ctx, retweet); err != nil {
		return nil, fmt.Errorf("create retweet: %w", err)
	}

	// Increment cached counter
	if s.cache != nil {
		s.cache.IncrCounter(ctx, cache.PrefixRetweets+tweetID)
	}

	// Queue fanout for retweet
	if s.queue != nil {
		followerCount := 0
		if count, err := s.getFollowerCount(ctx, userID); err == nil {
			followerCount = count
		}
		s.queue.PublishFanout(ctx, retweet.ID, userID, followerCount)

		// Send notification
		if originalTweet.AuthorID != userID {
			s.queue.PublishNotification(ctx, originalTweet.AuthorID, "retweet", userID, tweetID)
		}
	}

	return retweet, nil
}

func (s *TweetService) Unretweet(ctx context.Context, userID, tweetID string) error {
	if err := s.repo.Unretweet(ctx, userID, tweetID); err != nil {
		return fmt.Errorf("unretweet: %w", err)
	}

	// Decrement cached counter
	if s.cache != nil {
		s.cache.DecrCounter(ctx, cache.PrefixRetweets+tweetID)
	}

	return nil
}

func (s *TweetService) getFollowerCount(ctx context.Context, userID string) (int, error) {
	if s.userClient == nil {
		return 0, nil
	}

	user, err := s.userClient.GetUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	return user.FollowerCount, nil
}
