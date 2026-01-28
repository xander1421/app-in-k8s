package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/alexprut/twitter-clone/pkg/models"
	"github.com/alexprut/twitter-clone/pkg/testutil"
)

func TestTweetRepository_Create(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := NewTweetRepository(pool)

	ctx := context.Background()
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	testutil.CleanupTables(t, pool, "likes", "retweets", "tweets")

	tweet := &models.Tweet{
		ID:        uuid.New().String(),
		AuthorID:  uuid.New().String(),
		Content:   "Hello, world! #test",
		CreatedAt: time.Now(),
	}

	err := repo.Create(ctx, tweet)
	if err != nil {
		t.Fatalf("Failed to create tweet: %v", err)
	}

	// Verify tweet was created
	retrieved, err := repo.GetByID(ctx, tweet.ID)
	if err != nil {
		t.Fatalf("Failed to get tweet: %v", err)
	}

	if retrieved.Content != tweet.Content {
		t.Errorf("Expected content %s, got %s", tweet.Content, retrieved.Content)
	}
	if retrieved.AuthorID != tweet.AuthorID {
		t.Errorf("Expected author %s, got %s", tweet.AuthorID, retrieved.AuthorID)
	}
}

func TestTweetRepository_CreateReply(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := NewTweetRepository(pool)

	ctx := context.Background()
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	testutil.CleanupTables(t, pool, "likes", "retweets", "tweets")

	// Create original tweet
	originalTweet := &models.Tweet{
		ID:        uuid.New().String(),
		AuthorID:  uuid.New().String(),
		Content:   "Original tweet",
		CreatedAt: time.Now(),
	}
	if err := repo.Create(ctx, originalTweet); err != nil {
		t.Fatalf("Failed to create original tweet: %v", err)
	}

	// Create reply
	reply := &models.Tweet{
		ID:        uuid.New().String(),
		AuthorID:  uuid.New().String(),
		Content:   "This is a reply",
		ReplyToID: originalTweet.ID,
		CreatedAt: time.Now(),
	}
	if err := repo.Create(ctx, reply); err != nil {
		t.Fatalf("Failed to create reply: %v", err)
	}

	// Verify reply
	retrieved, err := repo.GetByID(ctx, reply.ID)
	if err != nil {
		t.Fatalf("Failed to get reply: %v", err)
	}

	if retrieved.ReplyToID != originalTweet.ID {
		t.Errorf("Expected ReplyToID %s, got %s", originalTweet.ID, retrieved.ReplyToID)
	}
}

func TestTweetRepository_Delete(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := NewTweetRepository(pool)

	ctx := context.Background()
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	testutil.CleanupTables(t, pool, "likes", "retweets", "tweets")

	tweet := &models.Tweet{
		ID:        uuid.New().String(),
		AuthorID:  uuid.New().String(),
		Content:   "Tweet to delete",
		CreatedAt: time.Now(),
	}
	if err := repo.Create(ctx, tweet); err != nil {
		t.Fatalf("Failed to create tweet: %v", err)
	}

	// Delete tweet
	if err := repo.Delete(ctx, tweet.ID); err != nil {
		t.Fatalf("Failed to delete tweet: %v", err)
	}

	// Verify deletion
	_, err := repo.GetByID(ctx, tweet.ID)
	if err == nil {
		t.Error("Expected error when getting deleted tweet")
	}
}

func TestTweetRepository_GetByAuthor(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := NewTweetRepository(pool)

	ctx := context.Background()
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	testutil.CleanupTables(t, pool, "likes", "retweets", "tweets")

	authorID := uuid.New().String()

	// Create multiple tweets
	for i := 0; i < 5; i++ {
		tweet := &models.Tweet{
			ID:        uuid.New().String(),
			AuthorID:  authorID,
			Content:   "Tweet content",
			CreatedAt: time.Now().Add(time.Duration(i) * time.Second),
		}
		if err := repo.Create(ctx, tweet); err != nil {
			t.Fatalf("Failed to create tweet: %v", err)
		}
	}

	// Get tweets by author
	tweets, err := repo.GetByAuthor(ctx, authorID, 10, 0)
	if err != nil {
		t.Fatalf("Failed to get tweets: %v", err)
	}

	if len(tweets) != 5 {
		t.Errorf("Expected 5 tweets, got %d", len(tweets))
	}
}

func TestTweetRepository_Like(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := NewTweetRepository(pool)

	ctx := context.Background()
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	testutil.CleanupTables(t, pool, "likes", "retweets", "tweets")

	// Create a tweet
	tweet := &models.Tweet{
		ID:        uuid.New().String(),
		AuthorID:  uuid.New().String(),
		Content:   "Tweet to like",
		CreatedAt: time.Now(),
	}
	if err := repo.Create(ctx, tweet); err != nil {
		t.Fatalf("Failed to create tweet: %v", err)
	}

	userID := uuid.New().String()

	// Like the tweet
	if err := repo.Like(ctx, userID, tweet.ID); err != nil {
		t.Fatalf("Failed to like tweet: %v", err)
	}

	// Check if liked
	isLiked, err := repo.IsLiked(ctx, userID, tweet.ID)
	if err != nil {
		t.Fatalf("Failed to check like: %v", err)
	}
	if !isLiked {
		t.Error("Expected tweet to be liked")
	}

	// Check like count
	retrieved, err := repo.GetByID(ctx, tweet.ID)
	if err != nil {
		t.Fatalf("Failed to get tweet: %v", err)
	}
	if retrieved.LikeCount != 1 {
		t.Errorf("Expected like count 1, got %d", retrieved.LikeCount)
	}
}

func TestTweetRepository_Unlike(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := NewTweetRepository(pool)

	ctx := context.Background()
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	testutil.CleanupTables(t, pool, "likes", "retweets", "tweets")

	tweet := &models.Tweet{
		ID:        uuid.New().String(),
		AuthorID:  uuid.New().String(),
		Content:   "Tweet to unlike",
		CreatedAt: time.Now(),
	}
	if err := repo.Create(ctx, tweet); err != nil {
		t.Fatalf("Failed to create tweet: %v", err)
	}

	userID := uuid.New().String()

	// Like then unlike
	if err := repo.Like(ctx, userID, tweet.ID); err != nil {
		t.Fatalf("Failed to like: %v", err)
	}
	if err := repo.Unlike(ctx, userID, tweet.ID); err != nil {
		t.Fatalf("Failed to unlike: %v", err)
	}

	// Check if still liked
	isLiked, err := repo.IsLiked(ctx, userID, tweet.ID)
	if err != nil {
		t.Fatalf("Failed to check like: %v", err)
	}
	if isLiked {
		t.Error("Expected tweet to be unliked")
	}

	// Check like count
	retrieved, err := repo.GetByID(ctx, tweet.ID)
	if err != nil {
		t.Fatalf("Failed to get tweet: %v", err)
	}
	if retrieved.LikeCount != 0 {
		t.Errorf("Expected like count 0, got %d", retrieved.LikeCount)
	}
}

func TestTweetRepository_Retweet(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := NewTweetRepository(pool)

	ctx := context.Background()
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	testutil.CleanupTables(t, pool, "likes", "retweets", "tweets")

	tweet := &models.Tweet{
		ID:        uuid.New().String(),
		AuthorID:  uuid.New().String(),
		Content:   "Tweet to retweet",
		CreatedAt: time.Now(),
	}
	if err := repo.Create(ctx, tweet); err != nil {
		t.Fatalf("Failed to create tweet: %v", err)
	}

	userID := uuid.New().String()

	// Retweet
	if err := repo.Retweet(ctx, userID, tweet.ID); err != nil {
		t.Fatalf("Failed to retweet: %v", err)
	}

	// Check if retweeted
	isRetweeted, err := repo.IsRetweeted(ctx, userID, tweet.ID)
	if err != nil {
		t.Fatalf("Failed to check retweet: %v", err)
	}
	if !isRetweeted {
		t.Error("Expected tweet to be retweeted")
	}

	// Check retweet count
	retrieved, err := repo.GetByID(ctx, tweet.ID)
	if err != nil {
		t.Fatalf("Failed to get tweet: %v", err)
	}
	if retrieved.RetweetCount != 1 {
		t.Errorf("Expected retweet count 1, got %d", retrieved.RetweetCount)
	}
}

func TestTweetRepository_GetReplies(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := NewTweetRepository(pool)

	ctx := context.Background()
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	testutil.CleanupTables(t, pool, "likes", "retweets", "tweets")

	// Create original tweet
	originalTweet := &models.Tweet{
		ID:        uuid.New().String(),
		AuthorID:  uuid.New().String(),
		Content:   "Original tweet",
		CreatedAt: time.Now(),
	}
	if err := repo.Create(ctx, originalTweet); err != nil {
		t.Fatalf("Failed to create original tweet: %v", err)
	}

	// Create replies
	for i := 0; i < 3; i++ {
		reply := &models.Tweet{
			ID:        uuid.New().String(),
			AuthorID:  uuid.New().String(),
			Content:   "Reply content",
			ReplyToID: originalTweet.ID,
			CreatedAt: time.Now().Add(time.Duration(i) * time.Second),
		}
		if err := repo.Create(ctx, reply); err != nil {
			t.Fatalf("Failed to create reply: %v", err)
		}
	}

	// Get replies
	replies, err := repo.GetReplies(ctx, originalTweet.ID, 10, 0)
	if err != nil {
		t.Fatalf("Failed to get replies: %v", err)
	}

	if len(replies) != 3 {
		t.Errorf("Expected 3 replies, got %d", len(replies))
	}
}

func TestTweetRepository_BatchGet(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := NewTweetRepository(pool)

	ctx := context.Background()
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	testutil.CleanupTables(t, pool, "likes", "retweets", "tweets")

	// Create multiple tweets
	tweetIDs := make([]string, 5)
	for i := 0; i < 5; i++ {
		tweet := &models.Tweet{
			ID:        uuid.New().String(),
			AuthorID:  uuid.New().String(),
			Content:   "Batch tweet content",
			CreatedAt: time.Now(),
		}
		tweetIDs[i] = tweet.ID
		if err := repo.Create(ctx, tweet); err != nil {
			t.Fatalf("Failed to create tweet: %v", err)
		}
	}

	// Batch get
	tweets, err := repo.BatchGet(ctx, tweetIDs)
	if err != nil {
		t.Fatalf("Failed to batch get: %v", err)
	}

	if len(tweets) != 5 {
		t.Errorf("Expected 5 tweets, got %d", len(tweets))
	}
}

func TestTweetRepository_BatchGet_Empty(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := NewTweetRepository(pool)

	ctx := context.Background()

	tweets, err := repo.BatchGet(ctx, []string{})
	if err != nil {
		t.Fatalf("Failed to batch get: %v", err)
	}

	if tweets != nil {
		t.Errorf("Expected nil, got %v", tweets)
	}
}

func TestTweetRepository_IncrementReplyCount(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := NewTweetRepository(pool)

	ctx := context.Background()
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	testutil.CleanupTables(t, pool, "likes", "retweets", "tweets")

	tweet := &models.Tweet{
		ID:        uuid.New().String(),
		AuthorID:  uuid.New().String(),
		Content:   "Tweet content",
		CreatedAt: time.Now(),
	}
	if err := repo.Create(ctx, tweet); err != nil {
		t.Fatalf("Failed to create tweet: %v", err)
	}

	// Increment reply count
	if err := repo.IncrementReplyCount(ctx, tweet.ID); err != nil {
		t.Fatalf("Failed to increment reply count: %v", err)
	}

	// Check reply count
	retrieved, err := repo.GetByID(ctx, tweet.ID)
	if err != nil {
		t.Fatalf("Failed to get tweet: %v", err)
	}
	if retrieved.ReplyCount != 1 {
		t.Errorf("Expected reply count 1, got %d", retrieved.ReplyCount)
	}
}
