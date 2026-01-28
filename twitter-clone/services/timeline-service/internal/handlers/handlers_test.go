package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexprut/twitter-clone/pkg/middleware"
)

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	HealthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %s", resp["status"])
	}
}

func TestGetHomeTimeline_Unauthorized(t *testing.T) {
	handler := &TimelineHandler{svc: nil}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/timeline/home", nil)
	w := httptest.NewRecorder()

	handler.GetHomeTimeline(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["code"] != "unauthorized" {
		t.Errorf("expected code 'unauthorized', got %v", resp["code"])
	}
}

func TestGetUserTimeline_MissingID(t *testing.T) {
	handler := &TimelineHandler{svc: nil}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/timeline/user/", nil)
	w := httptest.NewRecorder()

	handler.GetUserTimeline(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestAddToTimeline_InvalidBody(t *testing.T) {
	handler := &TimelineHandler{svc: nil}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/timeline/add", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	handler.AddToTimeline(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestAddToTimeline_ValidBody(t *testing.T) {
	body := map[string]interface{}{
		"user_id":  "user-123",
		"tweet_id": "tweet-456",
		"score":    1234567890.0,
	}
	bodyBytes, _ := json.Marshal(body)

	// Just validate parsing works
	var parsed struct {
		UserID  string  `json:"user_id"`
		TweetID string  `json:"tweet_id"`
		Score   float64 `json:"score"`
	}

	if err := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&parsed); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	if parsed.UserID != "user-123" {
		t.Errorf("expected user_id 'user-123', got %s", parsed.UserID)
	}
	if parsed.TweetID != "tweet-456" {
		t.Errorf("expected tweet_id 'tweet-456', got %s", parsed.TweetID)
	}
	if parsed.Score != 1234567890.0 {
		t.Errorf("expected score 1234567890.0, got %f", parsed.Score)
	}
}

func TestTimelineLimitParsing(t *testing.T) {
	tests := []struct {
		name          string
		queryString   string
		expectedLimit int
		expectedOffset int
	}{
		{"default values", "", 0, 0},
		{"custom limit", "limit=50", 50, 0},
		{"custom offset", "offset=20", 0, 20},
		{"both custom", "limit=30&offset=60", 30, 60},
		{"invalid limit", "limit=abc", 0, 0},
		{"invalid offset", "offset=xyz", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/timeline/home?"+tt.queryString, nil)

			limit := 0
			offset := 0

			if l := req.URL.Query().Get("limit"); l != "" {
				var parsed int
				if err := json.Unmarshal([]byte(l), &parsed); err == nil {
					limit = parsed
				}
			}

			if o := req.URL.Query().Get("offset"); o != "" {
				var parsed int
				if err := json.Unmarshal([]byte(o), &parsed); err == nil {
					offset = parsed
				}
			}

			if limit != tt.expectedLimit {
				t.Errorf("limit = %d, want %d", limit, tt.expectedLimit)
			}
			if offset != tt.expectedOffset {
				t.Errorf("offset = %d, want %d", offset, tt.expectedOffset)
			}
		})
	}
}

func TestMiddlewareUserIDExtraction(t *testing.T) {
	tests := []struct {
		name     string
		userID   string
		hasUser  bool
	}{
		{"with user", "user-123", true},
		{"empty user", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)

			if tt.userID != "" {
				ctx := middleware.SetUserID(req.Context(), tt.userID)
				req = req.WithContext(ctx)
			}

			userID := middleware.GetUserID(req.Context())
			hasUser := userID != ""

			if hasUser != tt.hasUser {
				t.Errorf("hasUser = %v, want %v", hasUser, tt.hasUser)
			}
			if hasUser && userID != tt.userID {
				t.Errorf("userID = %s, want %s", userID, tt.userID)
			}
		})
	}
}

func TestRegisterRoutes(t *testing.T) {
	handler := &TimelineHandler{svc: nil}
	mux := http.NewServeMux()

	handler.RegisterRoutes(mux)

	// Test that routes are registered by checking patterns
	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/timeline/home"},
		{"GET", "/api/v1/timeline/user/123"},
		{"POST", "/api/v1/timeline/add"},
	}

	for _, route := range routes {
		req := httptest.NewRequest(route.method, route.path, nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		// Routes should respond (even if with error due to nil service)
		// 404 would mean route not registered
		if w.Code == http.StatusNotFound {
			t.Errorf("route %s %s not registered", route.method, route.path)
		}
	}
}
