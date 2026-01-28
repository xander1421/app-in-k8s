package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/alexprut/twitter-clone/pkg/cache"
	"github.com/alexprut/twitter-clone/pkg/database"
	"github.com/alexprut/twitter-clone/pkg/models"
	"github.com/alexprut/twitter-clone/services/user-service/internal/repository"
)

// TestServices holds all test service URLs
type TestServices struct {
	UserService         string
	TweetService        string
	TimelineService     string
	SearchService       string
	MediaService        string
	NotificationService string
	servers             []*httptest.Server
}

// Shutdown stops all test servers
func (ts *TestServices) Shutdown() {
	for _, server := range ts.servers {
		server.Close()
	}
}

// setupTestDatabase creates a test database
func setupTestDatabase(t *testing.T) *database.PostgresDB {
	ctx := context.Background()
	dbURL := "postgres://postgres:postgres@localhost:5432/test_db?sslmode=disable"
	
	db, err := database.NewPostgresDB(ctx, dbURL)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	
	// Clean database
	cleanTestDatabase(t, db.Pool())
	
	return db
}

// cleanTestDatabase removes all test data
func cleanTestDatabase(t *testing.T, pool *pgxpool.Pool) {
	ctx := context.Background()
	tables := []string{
		"bookmarks",
		"likes",
		"retweets",
		"tweets",
		"follows",
		"blocks",
		"mutes",
		"sessions",
		"password_resets",
		"email_verifications",
		"login_history",
		"users",
		"media",
		"notifications",
	}
	
	for _, table := range tables {
		_, err := pool.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE IF EXISTS %s CASCADE", table))
		if err != nil {
			t.Logf("Warning: Failed to truncate %s: %v", table, err)
		}
	}
}

// setupAuthRepository creates auth repository for testing
func setupAuthRepository(db *database.PostgresDB) *repository.UserRepositoryAuth {
	return repository.NewUserRepositoryAuth(db.Pool())
}

// setupMockRedis creates a mock Redis cache for testing
func setupMockRedis(t *testing.T) *cache.RedisCache {
	// Use in-memory mock or connect to test Redis
	ctx := context.Background()
	cache, err := cache.NewRedisCacheSimple(ctx, "localhost:6379", "", "test")
	if err != nil {
		t.Logf("Warning: Using mock Redis cache: %v", err)
		// Return mock implementation
		return createMockRedisCache()
	}
	return cache
}

// createMockRedisCache creates an in-memory Redis mock
func createMockRedisCache() *cache.RedisCache {
	// This would be a mock implementation
	// For now, returning nil - should implement proper mock
	return nil
}

// startTestServices starts all microservices for testing
func startTestServices(t *testing.T) *TestServices {
	services := &TestServices{
		servers: make([]*httptest.Server, 0),
	}
	
	// Start each service
	// These would start actual service instances or mocks
	// For now, using httptest servers
	
	// Mock implementations would go here
	userServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock user service
		w.WriteHeader(http.StatusOK)
	}))
	services.servers = append(services.servers, userServer)
	services.UserService = userServer.URL
	
	// Add other services...
	
	return services
}

// waitForServices waits for all services to be ready
func waitForServices(t *testing.T, services *TestServices) {
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for services to be ready")
		case <-ticker.C:
			if checkServiceHealth(services.UserService + "/health") {
				return
			}
		}
	}
}

// checkServiceHealth checks if a service is healthy
func checkServiceHealth(url string) bool {
	resp, err := http.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// API Helper Functions

func registerNewUser(t *testing.T, serviceURL, username, email string) *models.AuthResponse {
	registerReq := models.RegisterRequest{
		Username: username,
		Email:    email,
		Password: "SecureP@ss123",
		Name:     fmt.Sprintf("%s User", username),
	}
	
	body, _ := json.Marshal(registerReq)
	resp, err := http.Post(serviceURL+"/api/v1/auth/register", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to register user: %s", body)
	}
	
	var authResp models.AuthResponse
	json.NewDecoder(resp.Body).Decode(&authResp)
	return &authResp
}

func followUser(t *testing.T, serviceURL, token, targetUserID string) {
	req, _ := http.NewRequest("POST", serviceURL+"/api/v1/users/"+targetUserID+"/follow", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Errorf("Failed to follow user: status %d", resp.StatusCode)
	}
}

func postTweet(t *testing.T, serviceURL, token, content string) *models.Tweet {
	tweet, err := postTweetWithError(serviceURL, token, content)
	if err != nil {
		t.Fatal(err)
	}
	return tweet
}

func postTweetWithError(serviceURL, token, content string) (*models.Tweet, error) {
	tweetReq := map[string]interface{}{
		"content": content,
	}
	
	body, _ := json.Marshal(tweetReq)
	req, _ := http.NewRequest("POST", serviceURL+"/api/v1/tweets", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limit exceeded")
	}
	
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create tweet: %s", body)
	}
	
	var tweet models.Tweet
	json.NewDecoder(resp.Body).Decode(&tweet)
	return &tweet, nil
}

func postTweetWithMedia(t *testing.T, serviceURL, token, content string, mediaIDs []string) *models.Tweet {
	tweetReq := map[string]interface{}{
		"content":   content,
		"media_ids": mediaIDs,
	}
	
	body, _ := json.Marshal(tweetReq)
	req, _ := http.NewRequest("POST", serviceURL+"/api/v1/tweets", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	var tweet models.Tweet
	json.NewDecoder(resp.Body).Decode(&tweet)
	return &tweet
}

func likeTweet(t *testing.T, serviceURL, token, tweetID string) {
	req, _ := http.NewRequest("POST", serviceURL+"/api/v1/tweets/"+tweetID+"/like", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Errorf("Failed to like tweet: status %d", resp.StatusCode)
	}
}

func retweet(t *testing.T, serviceURL, token, tweetID string) {
	req, _ := http.NewRequest("POST", serviceURL+"/api/v1/tweets/"+tweetID+"/retweet", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Errorf("Failed to retweet: status %d", resp.StatusCode)
	}
}

func replyToTweet(t *testing.T, serviceURL, token, tweetID, content string) *models.Tweet {
	replyReq := map[string]interface{}{
		"content":      content,
		"reply_to_id": tweetID,
	}
	
	body, _ := json.Marshal(replyReq)
	req, _ := http.NewRequest("POST", serviceURL+"/api/v1/tweets", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	var tweet models.Tweet
	json.NewDecoder(resp.Body).Decode(&tweet)
	return &tweet
}

func bookmarkTweet(t *testing.T, serviceURL, token, tweetID string) {
	req, _ := http.NewRequest("POST", serviceURL+"/api/v1/tweets/"+tweetID+"/bookmark", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Errorf("Failed to bookmark tweet: status %d", resp.StatusCode)
	}
}

func getTimeline(t *testing.T, serviceURL, token string) *models.TimelineResponse {
	timeline, err := getTimelineWithError(serviceURL, token)
	if err != nil {
		t.Fatal(err)
	}
	return timeline
}

func getTimelineWithError(serviceURL, token string) (*models.TimelineResponse, error) {
	req, _ := http.NewRequest("GET", serviceURL+"/api/v1/timeline/home", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limit exceeded")
	}
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get timeline: %s", body)
	}
	
	var timeline models.TimelineResponse
	json.NewDecoder(resp.Body).Decode(&timeline)
	return &timeline, nil
}

func getNotifications(t *testing.T, serviceURL, token string) []models.Notification {
	req, _ := http.NewRequest("GET", serviceURL+"/api/v1/notifications", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	var notifications []models.Notification
	json.NewDecoder(resp.Body).Decode(&notifications)
	return notifications
}

func clearNotifications(t *testing.T, serviceURL, token string) {
	req, _ := http.NewRequest("DELETE", serviceURL+"/api/v1/notifications/read-all", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
}

func searchTweets(t *testing.T, serviceURL, query string) []models.Tweet {
	req, _ := http.NewRequest("GET", serviceURL+"/api/v1/search/tweets?q="+query, nil)
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	var results struct {
		Tweets []models.Tweet `json:"tweets"`
	}
	json.NewDecoder(resp.Body).Decode(&results)
	return results.Tweets
}

func searchUsers(t *testing.T, serviceURL, query string) []models.User {
	req, _ := http.NewRequest("GET", serviceURL+"/api/v1/search/users?q="+query, nil)
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	var results struct {
		Users []models.User `json:"users"`
	}
	json.NewDecoder(resp.Body).Decode(&results)
	return results.Users
}

func advancedSearch(t *testing.T, serviceURL string, params map[string]interface{}) []models.Tweet {
	body, _ := json.Marshal(params)
	resp, err := http.Post(serviceURL+"/api/v1/search/advanced", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	var results struct {
		Tweets []models.Tweet `json:"tweets"`
	}
	json.NewDecoder(resp.Body).Decode(&results)
	return results.Tweets
}

func uploadImage(t *testing.T, serviceURL, token, filename string) string {
	// Mock image upload
	return fmt.Sprintf("media_%d", time.Now().Unix())
}

func uploadVideo(t *testing.T, serviceURL, token, filename string) string {
	// Mock video upload
	return fmt.Sprintf("video_%d", time.Now().Unix())
}

func getMediaStatus(t *testing.T, serviceURL, token, mediaID string) *models.Media {
	req, _ := http.NewRequest("GET", serviceURL+"/api/v1/media/"+mediaID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	var media models.Media
	json.NewDecoder(resp.Body).Decode(&media)
	return &media
}

func getUserProfile(t *testing.T, serviceURL, token, userID string) *models.User {
	req, _ := http.NewRequest("GET", serviceURL+"/api/v1/users/"+userID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	
	var user models.User
	json.NewDecoder(resp.Body).Decode(&user)
	return &user
}

func updateNotificationPreferences(t *testing.T, serviceURL, token string, prefs map[string]bool) {
	body, _ := json.Marshal(prefs)
	req, _ := http.NewRequest("PUT", serviceURL+"/api/v1/users/settings/notifications", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
}