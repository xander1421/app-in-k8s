package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/alexprut/twitter-clone/pkg/middleware"
	"github.com/alexprut/twitter-clone/pkg/models"
)

// AddBookmark adds a tweet to bookmarks
func (h *TweetHandler) AddBookmark(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	tweetID := r.PathValue("id")
	if tweetID == "" {
		http.Error(w, `{"error": "tweet ID required"}`, http.StatusBadRequest)
		return
	}

	// Check if tweet exists
	tweet, err := h.svc.GetTweet(r.Context(), tweetID)
	if err != nil {
		http.Error(w, `{"error": "tweet not found"}`, http.StatusNotFound)
		return
	}

	// Add bookmark
	bookmark := &models.Bookmark{
		UserID:  userID,
		TweetID: tweetID,
	}

	if err := h.svc.AddBookmark(r.Context(), bookmark); err != nil {
		http.Error(w, `{"error": "failed to add bookmark"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "bookmark added",
		"tweet":   tweet,
	})
}

// RemoveBookmark removes a tweet from bookmarks
func (h *TweetHandler) RemoveBookmark(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	tweetID := r.PathValue("id")
	if tweetID == "" {
		http.Error(w, `{"error": "tweet ID required"}`, http.StatusBadRequest)
		return
	}

	// Remove bookmark
	if err := h.svc.RemoveBookmark(r.Context(), userID, tweetID); err != nil {
		http.Error(w, `{"error": "failed to remove bookmark"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "bookmark removed",
	})
}

// GetBookmarks returns user's bookmarked tweets
func (h *TweetHandler) GetBookmarks(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	// Parse query parameters
	limit := 20
	cursor := r.URL.Query().Get("cursor")

	// Get bookmarks
	bookmarks, nextCursor, err := h.svc.GetUserBookmarks(r.Context(), userID, limit, cursor)
	if err != nil {
		http.Error(w, `{"error": "failed to get bookmarks"}`, http.StatusInternalServerError)
		return
	}

	// Get tweet details
	var tweets []models.Tweet
	for _, bookmark := range bookmarks {
		tweet, err := h.svc.GetTweet(r.Context(), bookmark.TweetID)
		if err == nil {
			tweets = append(tweets, *tweet)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tweets":      tweets,
		"next_cursor": nextCursor,
		"has_more":    len(tweets) == limit,
	})
}

// CheckBookmark checks if a tweet is bookmarked by user
func (h *TweetHandler) CheckBookmark(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}

	tweetID := r.PathValue("id")
	if tweetID == "" {
		http.Error(w, `{"error": "tweet ID required"}`, http.StatusBadRequest)
		return
	}

	// Check bookmark
	isBookmarked, err := h.svc.IsBookmarked(r.Context(), userID, tweetID)
	if err != nil {
		http.Error(w, `{"error": "failed to check bookmark"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{
		"bookmarked": isBookmarked,
	})
}

// GetBookmarkCount returns count of bookmarks for a tweet
func (h *TweetHandler) GetBookmarkCount(w http.ResponseWriter, r *http.Request) {
	tweetID := r.PathValue("id")
	if tweetID == "" {
		http.Error(w, `{"error": "tweet ID required"}`, http.StatusBadRequest)
		return
	}

	count, err := h.svc.GetBookmarkCount(r.Context(), tweetID)
	if err != nil {
		http.Error(w, `{"error": "failed to get bookmark count"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{
		"count": count,
	})
}

// RegisterBookmarkRoutes registers bookmark-related routes
func (h *TweetHandler) RegisterBookmarkRoutes(mux *http.ServeMux) {
	// Bookmark management
	mux.HandleFunc("POST /api/v1/tweets/{id}/bookmark", h.AddBookmark)
	mux.HandleFunc("DELETE /api/v1/tweets/{id}/bookmark", h.RemoveBookmark)
	mux.HandleFunc("GET /api/v1/tweets/{id}/bookmark", h.CheckBookmark)
	mux.HandleFunc("GET /api/v1/tweets/{id}/bookmarks/count", h.GetBookmarkCount)
	
	// User bookmarks
	mux.HandleFunc("GET /api/v1/bookmarks", h.GetBookmarks)
}