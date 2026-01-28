package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/alexprut/twitter-clone/pkg/middleware"
	"github.com/alexprut/twitter-clone/pkg/models"
	"github.com/alexprut/twitter-clone/pkg/testutil"
	"github.com/alexprut/twitter-clone/services/tweet-service/internal/repository"
	"github.com/alexprut/twitter-clone/services/tweet-service/internal/service"
)

func setupTweetIntegrationService(t *testing.T) *http.ServeMux {
	pool := testutil.TestDB(t)

	repo := repository.NewTweetRepository(pool)
	if err := repo.Migrate(context.Background()); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	testutil.CleanupTables(t, pool, "likes", "retweets", "tweets")

	svc := service.NewTweetService(repo, nil, nil, nil, nil)
	handler := NewTweetHandler(svc)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	mux.HandleFunc("GET /health", HealthHandler)

	return mux
}

func TestTweetIntegration_CreateAndGetTweet(t *testing.T) {
	mux := setupTweetIntegrationService(t)

	authorID := uuid.New().String()

	// Create tweet
	createReq := models.CreateTweetRequest{
		Content: "Hello, world! This is an integration test.",
	}
	body, _ := json.Marshal(createReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tweets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserID(req.Context(), authorID)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var createdTweet models.Tweet
	if err := json.NewDecoder(w.Body).Decode(&createdTweet); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if createdTweet.Content != createReq.Content {
		t.Errorf("Expected content %s, got %s", createReq.Content, createdTweet.Content)
	}

	// Get tweet by ID
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tweets/"+createdTweet.ID, nil)
	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var retrievedTweet models.Tweet
	if err := json.NewDecoder(w.Body).Decode(&retrievedTweet); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if retrievedTweet.ID != createdTweet.ID {
		t.Errorf("Expected ID %s, got %s", createdTweet.ID, retrievedTweet.ID)
	}
}

func TestTweetIntegration_CreateReply(t *testing.T) {
	mux := setupTweetIntegrationService(t)

	authorID := uuid.New().String()

	// Create original tweet
	originalReq := models.CreateTweetRequest{
		Content: "Original tweet",
	}
	body, _ := json.Marshal(originalReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tweets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserID(req.Context(), authorID)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	var originalTweet models.Tweet
	json.NewDecoder(w.Body).Decode(&originalTweet)

	// Create reply
	replyReq := models.CreateTweetRequest{
		Content:   "This is a reply",
		ReplyToID: originalTweet.ID,
	}
	body, _ = json.Marshal(replyReq)

	replyAuthorID := uuid.New().String()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/tweets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx = middleware.SetUserID(req.Context(), replyAuthorID)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var reply models.Tweet
	json.NewDecoder(w.Body).Decode(&reply)

	if reply.ReplyToID != originalTweet.ID {
		t.Errorf("Expected ReplyToID %s, got %s", originalTweet.ID, reply.ReplyToID)
	}

	// Get replies
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tweets/"+originalTweet.ID+"/replies", nil)
	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var repliesResp struct {
		Tweets  []models.Tweet `json:"tweets"`
		HasMore bool           `json:"has_more"`
	}
	json.NewDecoder(w.Body).Decode(&repliesResp)

	if len(repliesResp.Tweets) != 1 {
		t.Errorf("Expected 1 reply, got %d", len(repliesResp.Tweets))
	}
}

func TestTweetIntegration_DeleteTweet(t *testing.T) {
	mux := setupTweetIntegrationService(t)

	authorID := uuid.New().String()

	// Create tweet
	createReq := models.CreateTweetRequest{
		Content: "Tweet to delete",
	}
	body, _ := json.Marshal(createReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tweets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserID(req.Context(), authorID)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	var tweet models.Tweet
	json.NewDecoder(w.Body).Decode(&tweet)

	// Delete tweet
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/tweets/"+tweet.ID, nil)
	ctx = middleware.SetUserID(req.Context(), authorID)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Try to get deleted tweet
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tweets/"+tweet.ID, nil)
	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestTweetIntegration_LikeUnlike(t *testing.T) {
	mux := setupTweetIntegrationService(t)

	authorID := uuid.New().String()
	likerID := uuid.New().String()

	// Create tweet
	createReq := models.CreateTweetRequest{
		Content: "Tweet to like",
	}
	body, _ := json.Marshal(createReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tweets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserID(req.Context(), authorID)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	var tweet models.Tweet
	json.NewDecoder(w.Body).Decode(&tweet)

	// Like tweet
	req = httptest.NewRequest(http.MethodPost, "/api/v1/tweets/"+tweet.ID+"/like", nil)
	ctx = middleware.SetUserID(req.Context(), likerID)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Get tweet to check like count
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tweets/"+tweet.ID, nil)
	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	var updatedTweet models.Tweet
	json.NewDecoder(w.Body).Decode(&updatedTweet)

	if updatedTweet.LikeCount != 1 {
		t.Errorf("Expected like count 1, got %d", updatedTweet.LikeCount)
	}

	// Unlike tweet
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/tweets/"+tweet.ID+"/like", nil)
	ctx = middleware.SetUserID(req.Context(), likerID)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Check like count again
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tweets/"+tweet.ID, nil)
	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	json.NewDecoder(w.Body).Decode(&updatedTweet)

	if updatedTweet.LikeCount != 0 {
		t.Errorf("Expected like count 0, got %d", updatedTweet.LikeCount)
	}
}

func TestTweetIntegration_Retweet(t *testing.T) {
	mux := setupTweetIntegrationService(t)

	authorID := uuid.New().String()
	retweeterID := uuid.New().String()

	// Create tweet
	createReq := models.CreateTweetRequest{
		Content: "Tweet to retweet",
	}
	body, _ := json.Marshal(createReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tweets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserID(req.Context(), authorID)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	var tweet models.Tweet
	json.NewDecoder(w.Body).Decode(&tweet)

	// Retweet
	req = httptest.NewRequest(http.MethodPost, "/api/v1/tweets/"+tweet.ID+"/retweet", nil)
	ctx = middleware.SetUserID(req.Context(), retweeterID)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	// Check retweet count
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tweets/"+tweet.ID, nil)
	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	var updatedTweet models.Tweet
	json.NewDecoder(w.Body).Decode(&updatedTweet)

	if updatedTweet.RetweetCount != 1 {
		t.Errorf("Expected retweet count 1, got %d", updatedTweet.RetweetCount)
	}
}

func TestTweetIntegration_HealthCheck(t *testing.T) {
	mux := setupTweetIntegrationService(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["status"] != "healthy" {
		t.Errorf("Expected healthy status, got %s", resp["status"])
	}
}

func TestTweetIntegration_MultipleLikes(t *testing.T) {
	mux := setupTweetIntegrationService(t)

	authorID := uuid.New().String()

	// Create tweet
	createReq := models.CreateTweetRequest{
		Content: "Popular tweet",
	}
	body, _ := json.Marshal(createReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tweets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserID(req.Context(), authorID)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	var tweet models.Tweet
	json.NewDecoder(w.Body).Decode(&tweet)

	// Multiple users like the tweet
	numLikers := 5
	for i := 0; i < numLikers; i++ {
		likerID := uuid.New().String()
		req = httptest.NewRequest(http.MethodPost, "/api/v1/tweets/"+tweet.ID+"/like", nil)
		ctx = middleware.SetUserID(req.Context(), likerID)
		req = req.WithContext(ctx)
		w = httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Failed to like tweet on iteration %d: %s", i, w.Body.String())
		}
	}

	// Check like count
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tweets/"+tweet.ID, nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var updatedTweet models.Tweet
	json.NewDecoder(w.Body).Decode(&updatedTweet)

	if updatedTweet.LikeCount != numLikers {
		t.Errorf("Expected like count %d, got %d", numLikers, updatedTweet.LikeCount)
	}
}
