package integration

import (
	"context"
	"testing"
	"time"

	"github.com/alexprut/twitter-clone/pkg/cache"
	"github.com/alexprut/twitter-clone/pkg/clients"
	"github.com/alexprut/twitter-clone/pkg/models"
	"github.com/alexprut/twitter-clone/services/timeline-service/internal/service"
)

// TestTimelineStrategies tests different timeline generation strategies
func TestTimelineStrategies(t *testing.T) {
	ctx := context.Background()
	
	// Setup mocks
	redisCache := setupMockRedis(t)
	userClient := setupMockUserClient()
	tweetClient := setupMockTweetClient()
	
	timelineService := service.NewTimelineServiceEnhanced(redisCache, userClient, tweetClient)

	t.Run("Push Strategy - Cached Timeline", func(t *testing.T) {
		userID := "user1"
		
		// Pre-populate cache with timeline
		timelineKey := "timeline:" + userID
		tweetIDs := []string{"tweet1", "tweet2", "tweet3"}
		for i, id := range tweetIDs {
			redisCache.AddToTimeline(ctx, timelineKey, id, time.Now().Unix()-int64(i))
		}
		
		// Get timeline
		timeline, err := timelineService.GetHomeTimeline(ctx, userID, 10, "")
		if err != nil {
			t.Fatal(err)
		}
		
		// Verify results
		if len(timeline.Tweets) != 3 {
			t.Errorf("Expected 3 tweets, got %d", len(timeline.Tweets))
		}
	})

	t.Run("Pull Strategy - On-Demand Generation", func(t *testing.T) {
		userID := "user2"
		
		// Ensure no cached timeline
		timelineKey := "timeline:" + userID
		redisCache.Delete(ctx, timelineKey)
		
		// Mock user following
		following := []*models.User{
			{ID: "friend1", Username: "friend1"},
			{ID: "friend2", Username: "friend2"},
		}
		userClient.(*mockUserClient).SetFollowing(userID, following)
		
		// Mock tweets from followed users
		tweetClient.(*mockTweetClient).AddUserTweets("friend1", []*models.Tweet{
			{ID: "tweet1", AuthorID: "friend1", Content: "Hello from friend1", CreatedAt: time.Now()},
		})
		tweetClient.(*mockTweetClient).AddUserTweets("friend2", []*models.Tweet{
			{ID: "tweet2", AuthorID: "friend2", Content: "Hello from friend2", CreatedAt: time.Now()},
		})
		
		// Get timeline
		timeline, err := timelineService.GetHomeTimeline(ctx, userID, 10, "")
		if err != nil {
			t.Fatal(err)
		}
		
		// Should have pulled tweets from followed users
		if len(timeline.Tweets) != 2 {
			t.Errorf("Expected 2 tweets, got %d", len(timeline.Tweets))
		}
	})

	t.Run("Hybrid Strategy - Celebrity Following", func(t *testing.T) {
		userID := "user3"
		
		// Mock user following celebrities
		following := []*models.User{
			{ID: "celebrity1", Username: "celebrity1", FollowerCount: 1000000},
			{ID: "regular1", Username: "regular1", FollowerCount: 100},
		}
		userClient.(*mockUserClient).SetFollowing(userID, following)
		
		// Add some cached tweets
		timelineKey := "timeline:" + userID
		redisCache.AddToTimeline(ctx, timelineKey, "cached1", time.Now().Unix())
		
		// Mock celebrity tweets (pulled on-demand)
		tweetClient.(*mockTweetClient).AddUserTweets("celebrity1", []*models.Tweet{
			{ID: "celeb_tweet1", AuthorID: "celebrity1", Content: "Celebrity update", CreatedAt: time.Now()},
		})
		
		// Get timeline
		timeline, err := timelineService.GetHomeTimeline(ctx, userID, 10, "")
		if err != nil {
			t.Fatal(err)
		}
		
		// Should have both cached and pulled tweets
		if len(timeline.Tweets) < 2 {
			t.Errorf("Expected at least 2 tweets, got %d", len(timeline.Tweets))
		}
	})

	t.Run("Timeline Rebuild on Empty Cache", func(t *testing.T) {
		userID := "user4"
		
		// Ensure empty cache
		timelineKey := "timeline:" + userID
		redisCache.Delete(ctx, timelineKey)
		
		// Mock following and their tweets
		following := []*models.User{
			{ID: "friend1", Username: "friend1"},
		}
		userClient.(*mockUserClient).SetFollowing(userID, following)
		
		tweetClient.(*mockTweetClient).AddUserTweets("friend1", []*models.Tweet{
			{ID: "rebuild1", AuthorID: "friend1", Content: "Rebuild tweet", CreatedAt: time.Now()},
		})
		
		// Get timeline - should trigger rebuild
		timeline, err := timelineService.GetHomeTimeline(ctx, userID, 10, "")
		if err != nil {
			t.Fatal(err)
		}
		
		// Check if timeline was rebuilt
		if len(timeline.Tweets) == 0 {
			t.Error("Timeline rebuild failed")
		}
		
		// Cache should now be populated
		cachedIDs, _ := redisCache.GetTimelineRange(ctx, timelineKey, 0, 10)
		if len(cachedIDs) == 0 {
			t.Error("Cache was not populated after rebuild")
		}
	})

	t.Run("Pagination", func(t *testing.T) {
		userID := "user5"
		timelineKey := "timeline:" + userID
		
		// Add many tweets
		for i := 0; i < 25; i++ {
			tweetID := fmt.Sprintf("tweet%d", i)
			redisCache.AddToTimeline(ctx, timelineKey, tweetID, time.Now().Unix()-int64(i))
		}
		
		// First page
		page1, err := timelineService.GetHomeTimeline(ctx, userID, 10, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(page1.Tweets) != 10 {
			t.Errorf("Expected 10 tweets in first page, got %d", len(page1.Tweets))
		}
		if !page1.HasMore {
			t.Error("Expected HasMore to be true")
		}
		
		// Second page
		page2, err := timelineService.GetHomeTimeline(ctx, userID, 10, page1.NextCursor)
		if err != nil {
			t.Fatal(err)
		}
		if len(page2.Tweets) != 10 {
			t.Errorf("Expected 10 tweets in second page, got %d", len(page2.Tweets))
		}
		
		// Check no duplicates
		seen := make(map[string]bool)
		for _, tweet := range page1.Tweets {
			seen[tweet.ID] = true
		}
		for _, tweet := range page2.Tweets {
			if seen[tweet.ID] {
				t.Errorf("Duplicate tweet %s in pagination", tweet.ID)
			}
		}
	})

	t.Run("Trending Timeline", func(t *testing.T) {
		// Add trending tweets
		trendingKey := "timeline:trending"
		trendingTweets := []string{"trending1", "trending2", "trending3"}
		for i, id := range trendingTweets {
			redisCache.AddToTimeline(ctx, trendingKey, id, time.Now().Unix()-int64(i))
		}
		
		// Mock tweet details
		for _, id := range trendingTweets {
			tweetClient.(*mockTweetClient).AddTweet(&models.Tweet{
				ID:        id,
				Content:   "Trending content",
				CreatedAt: time.Now(),
			})
		}
		
		// Get trending timeline
		timeline, err := timelineService.GetTrendingTimeline(ctx, 10, "")
		if err != nil {
			t.Fatal(err)
		}
		
		if len(timeline.Tweets) != 3 {
			t.Errorf("Expected 3 trending tweets, got %d", len(timeline.Tweets))
		}
	})
}

// TestTimelinePerformance tests timeline generation performance
func TestTimelinePerformance(t *testing.T) {
	ctx := context.Background()
	redisCache := setupMockRedis(t)
	userClient := setupMockUserClient()
	tweetClient := setupMockTweetClient()
	
	timelineService := service.NewTimelineServiceEnhanced(redisCache, userClient, tweetClient)

	t.Run("Large Following List", func(t *testing.T) {
		userID := "heavy_user"
		
		// Mock user following 1000 users
		following := make([]*models.User, 1000)
		for i := 0; i < 1000; i++ {
			following[i] = &models.User{
				ID:       fmt.Sprintf("user%d", i),
				Username: fmt.Sprintf("user%d", i),
			}
		}
		userClient.(*mockUserClient).SetFollowing(userID, following)
		
		// Add tweets for some users
		for i := 0; i < 100; i++ {
			tweetClient.(*mockTweetClient).AddUserTweets(fmt.Sprintf("user%d", i), []*models.Tweet{
				{
					ID:        fmt.Sprintf("tweet%d", i),
					AuthorID:  fmt.Sprintf("user%d", i),
					Content:   "Test tweet",
					CreatedAt: time.Now(),
				},
			})
		}
		
		// Measure timeline generation time
		start := time.Now()
		timeline, err := timelineService.GetHomeTimeline(ctx, userID, 50, "")
		duration := time.Since(start)
		
		if err != nil {
			t.Fatal(err)
		}
		
		// Should complete within reasonable time
		if duration > 2*time.Second {
			t.Errorf("Timeline generation took too long: %v", duration)
		}
		
		t.Logf("Generated timeline with %d tweets in %v", len(timeline.Tweets), duration)
	})

	t.Run("Cache Hit Performance", func(t *testing.T) {
		userID := "cached_user"
		timelineKey := "timeline:" + userID
		
		// Pre-populate cache with 1000 tweets
		for i := 0; i < 1000; i++ {
			tweetID := fmt.Sprintf("tweet%d", i)
			redisCache.AddToTimeline(ctx, timelineKey, tweetID, time.Now().Unix()-int64(i))
		}
		
		// Measure cache hit performance
		start := time.Now()
		timeline, err := timelineService.GetHomeTimeline(ctx, userID, 50, "")
		duration := time.Since(start)
		
		if err != nil {
			t.Fatal(err)
		}
		
		// Cache hit should be very fast
		if duration > 100*time.Millisecond {
			t.Errorf("Cache hit took too long: %v", duration)
		}
		
		t.Logf("Cache hit returned %d tweets in %v", len(timeline.Tweets), duration)
	})
}

// TestTimelineFanout tests fanout operations
func TestTimelineFanout(t *testing.T) {
	ctx := context.Background()
	redisCache := setupMockRedis(t)
	
	t.Run("Add Tweet to Followers", func(t *testing.T) {
		authorID := "author1"
		tweetID := "new_tweet"
		followerIDs := []string{"follower1", "follower2", "follower3"}
		
		// Add tweet to each follower's timeline
		timestamp := time.Now().Unix()
		for _, followerID := range followerIDs {
			timelineKey := fmt.Sprintf("timeline:%s", followerID)
			err := redisCache.AddToTimeline(ctx, timelineKey, tweetID, timestamp)
			if err != nil {
				t.Fatal(err)
			}
		}
		
		// Verify tweet appears in all timelines
		for _, followerID := range followerIDs {
			timelineKey := fmt.Sprintf("timeline:%s", followerID)
			tweets, err := redisCache.GetTimelineRange(ctx, timelineKey, 0, 10)
			if err != nil {
				t.Fatal(err)
			}
			
			found := false
			for _, id := range tweets {
				if id == tweetID {
					found = true
					break
				}
			}
			
			if !found {
				t.Errorf("Tweet not found in %s's timeline", followerID)
			}
		}
	})

	t.Run("Remove Tweet from Timelines", func(t *testing.T) {
		tweetID := "deleted_tweet"
		affectedUsers := []string{"user1", "user2", "user3"}
		
		// First add tweet
		timestamp := time.Now().Unix()
		for _, userID := range affectedUsers {
			timelineKey := fmt.Sprintf("timeline:%s", userID)
			redisCache.AddToTimeline(ctx, timelineKey, tweetID, timestamp)
		}
		
		// Now remove tweet
		for _, userID := range affectedUsers {
			timelineKey := fmt.Sprintf("timeline:%s", userID)
			err := redisCache.RemoveFromTimeline(ctx, timelineKey, tweetID)
			if err != nil {
				t.Fatal(err)
			}
		}
		
		// Verify removal
		for _, userID := range affectedUsers {
			timelineKey := fmt.Sprintf("timeline:%s", userID)
			tweets, _ := redisCache.GetTimelineRange(ctx, timelineKey, 0, 100)
			
			for _, id := range tweets {
				if id == tweetID {
					t.Errorf("Deleted tweet still in %s's timeline", userID)
				}
			}
		}
	})

	t.Run("Timeline Trimming", func(t *testing.T) {
		userID := "trim_user"
		timelineKey := fmt.Sprintf("timeline:%s", userID)
		maxSize := 800
		
		// Add more tweets than max size
		for i := 0; i < 1000; i++ {
			tweetID := fmt.Sprintf("tweet%d", i)
			redisCache.AddToTimeline(ctx, timelineKey, tweetID, time.Now().Unix()-int64(i))
		}
		
		// Trim timeline
		err := redisCache.TrimTimeline(ctx, timelineKey, maxSize)
		if err != nil {
			t.Fatal(err)
		}
		
		// Check size
		size, err := redisCache.GetTimelineSize(ctx, timelineKey)
		if err != nil {
			t.Fatal(err)
		}
		
		if size > maxSize {
			t.Errorf("Timeline not trimmed properly: size=%d, max=%d", size, maxSize)
		}
	})
}

// Mock implementations

type mockUserClient struct {
	following map[string][]*models.User
}

func setupMockUserClient() clients.UserClient {
	return &mockUserClient{
		following: make(map[string][]*models.User),
	}
}

func (m *mockUserClient) SetFollowing(userID string, users []*models.User) {
	m.following[userID] = users
}

func (m *mockUserClient) GetFollowing(ctx context.Context, userID string, limit, offset int) ([]*models.User, error) {
	users := m.following[userID]
	if users == nil {
		return []*models.User{}, nil
	}
	
	end := offset + limit
	if end > len(users) {
		end = len(users)
	}
	
	return users[offset:end], nil
}

func (m *mockUserClient) GetUser(ctx context.Context, userID string) (*models.User, error) {
	return &models.User{
		ID:             userID,
		Username:       userID,
		FollowingCount: len(m.following[userID]),
	}, nil
}

type mockTweetClient struct {
	tweets     map[string]*models.Tweet
	userTweets map[string][]*models.Tweet
}

func setupMockTweetClient() clients.TweetClient {
	return &mockTweetClient{
		tweets:     make(map[string]*models.Tweet),
		userTweets: make(map[string][]*models.Tweet),
	}
}

func (m *mockTweetClient) AddTweet(tweet *models.Tweet) {
	m.tweets[tweet.ID] = tweet
}

func (m *mockTweetClient) AddUserTweets(userID string, tweets []*models.Tweet) {
	m.userTweets[userID] = tweets
	for _, tweet := range tweets {
		m.tweets[tweet.ID] = tweet
	}
}

func (m *mockTweetClient) GetTweetsBatch(ctx context.Context, ids []string) ([]models.Tweet, error) {
	var tweets []models.Tweet
	for _, id := range ids {
		if tweet, ok := m.tweets[id]; ok {
			tweets = append(tweets, *tweet)
		}
	}
	return tweets, nil
}

func (m *mockTweetClient) GetUserTweets(ctx context.Context, userID string, limit, offset int) ([]models.Tweet, error) {
	userTweets := m.userTweets[userID]
	if userTweets == nil {
		return []models.Tweet{}, nil
	}
	
	var result []models.Tweet
	end := offset + limit
	if end > len(userTweets) {
		end = len(userTweets)
	}
	
	for i := offset; i < end; i++ {
		result = append(result, *userTweets[i])
	}
	
	return result, nil
}