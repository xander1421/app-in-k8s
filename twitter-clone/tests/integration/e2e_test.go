package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/alexprut/twitter-clone/pkg/models"
)

// TestEndToEndUserJourney tests a complete user journey through the system
func TestEndToEndUserJourney(t *testing.T) {
	// Start all services
	services := startTestServices(t)
	defer services.Shutdown()

	// Wait for services to be ready
	waitForServices(t, services)

	t.Run("Complete User Journey", func(t *testing.T) {
		// 1. Register new user
		user1 := registerNewUser(t, services.UserService, "alice", "alice@example.com")
		user2 := registerNewUser(t, services.UserService, "bob", "bob@example.com")

		// 2. Users follow each other
		followUser(t, services.UserService, user1.AccessToken, user2.User.ID)
		followUser(t, services.UserService, user2.AccessToken, user1.User.ID)

		// 3. Alice posts a tweet
		tweet1 := postTweet(t, services.TweetService, user1.AccessToken, "Hello Twitter Clone! 🚀")

		// 4. Bob likes and retweets Alice's tweet
		likeTweet(t, services.TweetService, user2.AccessToken, tweet1.ID)
		retweet(t, services.TweetService, user2.AccessToken, tweet1.ID)

		// 5. Bob replies to Alice's tweet
		reply := replyToTweet(t, services.TweetService, user2.AccessToken, tweet1.ID, "Welcome Alice! 👋")

		// 6. Alice bookmarks Bob's reply
		bookmarkTweet(t, services.TweetService, user1.AccessToken, reply.ID)

		// 7. Check Bob's timeline - should see Alice's tweet
		timeline := getTimeline(t, services.TimelineService, user2.AccessToken)
		if !containsTweet(timeline.Tweets, tweet1.ID) {
			t.Error("Alice's tweet not in Bob's timeline")
		}

		// 8. Check notifications
		notifications := getNotifications(t, services.NotificationService, user1.AccessToken)
		if len(notifications) < 3 { // follow, like, retweet
			t.Errorf("Expected at least 3 notifications, got %d", len(notifications))
		}

		// 9. Search for tweets
		searchResults := searchTweets(t, services.SearchService, "Twitter Clone")
		if len(searchResults) == 0 {
			t.Error("Search returned no results")
		}

		// 10. Upload media
		mediaID := uploadImage(t, services.MediaService, user1.AccessToken, "test.jpg")

		// 11. Post tweet with media
		tweetWithMedia := postTweetWithMedia(t, services.TweetService, user1.AccessToken, 
			"Check out this image!", []string{mediaID})

		// 12. Get user profile
		profile := getUserProfile(t, services.UserService, user1.AccessToken, user1.User.ID)
		if profile.TweetCount < 2 {
			t.Errorf("Expected at least 2 tweets, got %d", profile.TweetCount)
		}
	})
}

// TestConcurrentOperations tests system behavior under concurrent load
func TestConcurrentOperations(t *testing.T) {
	services := startTestServices(t)
	defer services.Shutdown()
	waitForServices(t, services)

	// Create test users
	users := make([]*models.AuthResponse, 10)
	for i := 0; i < 10; i++ {
		users[i] = registerNewUser(t, services.UserService, 
			fmt.Sprintf("user%d", i), 
			fmt.Sprintf("user%d@example.com", i))
	}

	t.Run("Concurrent Tweet Creation", func(t *testing.T) {
		var wg sync.WaitGroup
		tweets := make([]*models.Tweet, len(users))
		
		for i, user := range users {
			wg.Add(1)
			go func(idx int, u *models.AuthResponse) {
				defer wg.Done()
				tweet := postTweet(t, services.TweetService, u.AccessToken, 
					fmt.Sprintf("Concurrent tweet %d", idx))
				tweets[idx] = tweet
			}(i, user)
		}
		
		wg.Wait()

		// Verify all tweets were created
		for i, tweet := range tweets {
			if tweet == nil {
				t.Errorf("Tweet %d was not created", i)
			}
		}
	})

	t.Run("Concurrent Timeline Access", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make([]error, len(users))
		
		for i, user := range users {
			wg.Add(1)
			go func(idx int, u *models.AuthResponse) {
				defer wg.Done()
				_, err := getTimelineWithError(services.TimelineService, u.AccessToken)
				errors[idx] = err
			}(i, user)
		}
		
		wg.Wait()

		// Check for errors
		errorCount := 0
		for _, err := range errors {
			if err != nil {
				errorCount++
			}
		}
		
		if errorCount > 0 {
			t.Errorf("%d/%d timeline requests failed", errorCount, len(users))
		}
	})

	t.Run("Concurrent Following", func(t *testing.T) {
		// Each user follows all others
		var wg sync.WaitGroup
		
		for i, user1 := range users {
			for j, user2 := range users {
				if i != j {
					wg.Add(1)
					go func(u1, u2 *models.AuthResponse) {
						defer wg.Done()
						followUser(t, services.UserService, u1.AccessToken, u2.User.ID)
					}(user1, user2)
				}
			}
		}
		
		wg.Wait()

		// Verify following counts
		for _, user := range users {
			profile := getUserProfile(t, services.UserService, user.AccessToken, user.User.ID)
			if profile.FollowingCount != 9 { // Following 9 others
				t.Errorf("User %s following count is %d, expected 9", 
					user.User.Username, profile.FollowingCount)
			}
		}
	})
}

// TestRateLimiting tests rate limiting functionality
func TestRateLimiting(t *testing.T) {
	services := startTestServices(t)
	defer services.Shutdown()
	waitForServices(t, services)

	user := registerNewUser(t, services.UserService, "ratelimit", "ratelimit@example.com")

	t.Run("Tweet Service Rate Limit", func(t *testing.T) {
		// Attempt to create many tweets rapidly
		successCount := 0
		rateLimitCount := 0
		
		for i := 0; i < 400; i++ { // Try 400 requests (limit is 300/min)
			_, err := postTweetWithError(services.TweetService, user.AccessToken, 
				fmt.Sprintf("Tweet %d", i))
			
			if err == nil {
				successCount++
			} else if isRateLimitError(err) {
				rateLimitCount++
			}
		}
		
		if rateLimitCount == 0 {
			t.Error("No rate limit errors received")
		}
		
		t.Logf("Successful: %d, Rate limited: %d", successCount, rateLimitCount)
	})

	t.Run("Timeline Service Rate Limit", func(t *testing.T) {
		// Timeline has 200/min limit
		hitLimit := false
		
		for i := 0; i < 250; i++ {
			_, err := getTimelineWithError(services.TimelineService, user.AccessToken)
			if err != nil && isRateLimitError(err) {
				hitLimit = true
				break
			}
		}
		
		if !hitLimit {
			t.Error("Timeline rate limit not enforced")
		}
	})
}

// TestMediaProcessing tests media upload and processing
func TestMediaProcessing(t *testing.T) {
	services := startTestServices(t)
	defer services.Shutdown()
	waitForServices(t, services)

	user := registerNewUser(t, services.UserService, "media_user", "media@example.com")

	t.Run("Image Upload and Processing", func(t *testing.T) {
		// Upload image
		mediaID := uploadImage(t, services.MediaService, user.AccessToken, "test.jpg")
		
		// Wait for processing
		time.Sleep(2 * time.Second)
		
		// Get media status
		media := getMediaStatus(t, services.MediaService, user.AccessToken, mediaID)
		if media.ProcessingStatus != "completed" {
			t.Errorf("Media processing status: %s", media.ProcessingStatus)
		}
		
		// Check variants were created
		if len(media.Variants) < 3 { // thumbnail, small, medium
			t.Errorf("Expected at least 3 variants, got %d", len(media.Variants))
		}
	})

	t.Run("Video Upload and Processing", func(t *testing.T) {
		// Upload video
		mediaID := uploadVideo(t, services.MediaService, user.AccessToken, "test.mp4")
		
		// Wait for processing (longer for video)
		time.Sleep(5 * time.Second)
		
		// Get media status
		media := getMediaStatus(t, services.MediaService, user.AccessToken, mediaID)
		if media.ProcessingStatus != "completed" {
			t.Errorf("Video processing status: %s", media.ProcessingStatus)
		}
		
		// Check thumbnail was generated
		if media.ThumbnailURL == "" {
			t.Error("No thumbnail generated for video")
		}
	})

	t.Run("Content Moderation", func(t *testing.T) {
		// Upload potentially sensitive image
		mediaID := uploadImage(t, services.MediaService, user.AccessToken, "sensitive.jpg")
		
		// Wait for moderation
		time.Sleep(3 * time.Second)
		
		// Check moderation status
		media := getMediaStatus(t, services.MediaService, user.AccessToken, mediaID)
		if media.ModerationStatus == "" {
			t.Error("Media not moderated")
		}
	})
}

// TestNotificationDelivery tests notification system
func TestNotificationDelivery(t *testing.T) {
	services := startTestServices(t)
	defer services.Shutdown()
	waitForServices(t, services)

	user1 := registerNewUser(t, services.UserService, "notif1", "notif1@example.com")
	user2 := registerNewUser(t, services.UserService, "notif2", "notif2@example.com")

	t.Run("Follow Notification", func(t *testing.T) {
		// Clear notifications
		clearNotifications(t, services.NotificationService, user2.AccessToken)
		
		// User1 follows User2
		followUser(t, services.UserService, user1.AccessToken, user2.User.ID)
		
		// Wait for notification
		time.Sleep(1 * time.Second)
		
		// Check User2's notifications
		notifications := getNotifications(t, services.NotificationService, user2.AccessToken)
		
		found := false
		for _, notif := range notifications {
			if notif.Type == "follow" && notif.ActorID == user1.User.ID {
				found = true
				break
			}
		}
		
		if !found {
			t.Error("Follow notification not received")
		}
	})

	t.Run("Mention Notification", func(t *testing.T) {
		// Clear notifications
		clearNotifications(t, services.NotificationService, user2.AccessToken)
		
		// User1 mentions User2
		tweet := postTweet(t, services.TweetService, user1.AccessToken, 
			fmt.Sprintf("Hey @%s, check this out!", user2.User.Username))
		
		// Wait for notification
		time.Sleep(1 * time.Second)
		
		// Check User2's notifications
		notifications := getNotifications(t, services.NotificationService, user2.AccessToken)
		
		found := false
		for _, notif := range notifications {
			if notif.Type == "mention" && notif.TweetID == tweet.ID {
				found = true
				break
			}
		}
		
		if !found {
			t.Error("Mention notification not received")
		}
	})

	t.Run("Notification Preferences", func(t *testing.T) {
		// Update notification preferences
		updateNotificationPreferences(t, services.UserService, user2.AccessToken, 
			map[string]bool{
				"likes":    false,
				"retweets": false,
				"follows":  true,
				"mentions": true,
			})
		
		// Clear notifications
		clearNotifications(t, services.NotificationService, user2.AccessToken)
		
		// Create tweet
		tweet := postTweet(t, services.TweetService, user2.AccessToken, "Test tweet")
		
		// User1 likes (should not notify)
		likeTweet(t, services.TweetService, user1.AccessToken, tweet.ID)
		
		// Wait
		time.Sleep(1 * time.Second)
		
		// Check notifications
		notifications := getNotifications(t, services.NotificationService, user2.AccessToken)
		
		for _, notif := range notifications {
			if notif.Type == "like" {
				t.Error("Received like notification when disabled")
			}
		}
	})
}

// TestSearchFunctionality tests search capabilities
func TestSearchFunctionality(t *testing.T) {
	services := startTestServices(t)
	defer services.Shutdown()
	waitForServices(t, services)

	user := registerNewUser(t, services.UserService, "searcher", "search@example.com")

	// Create test content
	tweets := []string{
		"The quick brown fox jumps over the lazy dog",
		"Machine learning is revolutionizing technology",
		"#golang is amazing for backend development",
		"Check out this cool @TwitterClone feature",
	}

	for _, content := range tweets {
		postTweet(t, services.TweetService, user.AccessToken, content)
	}

	// Wait for indexing
	time.Sleep(2 * time.Second)

	t.Run("Text Search", func(t *testing.T) {
		results := searchTweets(t, services.SearchService, "machine learning")
		if len(results) != 1 {
			t.Errorf("Expected 1 result for 'machine learning', got %d", len(results))
		}
	})

	t.Run("Hashtag Search", func(t *testing.T) {
		results := searchTweets(t, services.SearchService, "#golang")
		if len(results) != 1 {
			t.Errorf("Expected 1 result for '#golang', got %d", len(results))
		}
	})

	t.Run("User Search", func(t *testing.T) {
		results := searchUsers(t, services.SearchService, "searcher")
		found := false
		for _, u := range results {
			if u.Username == "searcher" {
				found = true
				break
			}
		}
		if !found {
			t.Error("User not found in search results")
		}
	})

	t.Run("Advanced Search", func(t *testing.T) {
		// Search with filters
		results := advancedSearch(t, services.SearchService, map[string]interface{}{
			"query":     "technology",
			"from_user": user.User.Username,
			"min_likes": 0,
			"has_media": false,
		})
		
		if len(results) == 0 {
			t.Error("Advanced search returned no results")
		}
	})
}

// Helper function to check if error is rate limit
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "rate limit exceeded"
}

// Helper function to check if timeline contains tweet
func containsTweet(tweets []models.Tweet, tweetID string) bool {
	for _, tweet := range tweets {
		if tweet.ID == tweetID {
			return true
		}
	}
	return false
}