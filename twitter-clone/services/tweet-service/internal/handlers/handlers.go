package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/alexprut/twitter-clone/pkg/middleware"
	"github.com/alexprut/twitter-clone/pkg/models"
	"github.com/alexprut/twitter-clone/services/tweet-service/internal/service"
)

type TweetHandler struct {
	svc *service.TweetService
}

func NewTweetHandler(svc *service.TweetService) *TweetHandler {
	return &TweetHandler{svc: svc}
}

func (h *TweetHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/tweets", h.CreateTweet)
	mux.HandleFunc("GET /api/v1/tweets/{id}", h.GetTweet)
	mux.HandleFunc("DELETE /api/v1/tweets/{id}", h.DeleteTweet)
	mux.HandleFunc("POST /api/v1/tweets/{id}/like", h.Like)
	mux.HandleFunc("DELETE /api/v1/tweets/{id}/like", h.Unlike)
	mux.HandleFunc("POST /api/v1/tweets/{id}/retweet", h.Retweet)
	mux.HandleFunc("DELETE /api/v1/tweets/{id}/retweet", h.Unretweet)
	mux.HandleFunc("GET /api/v1/tweets/{id}/replies", h.GetReplies)
	mux.HandleFunc("POST /api/v1/tweets/batch", h.BatchGetTweets)
	mux.HandleFunc("GET /api/v1/users/{id}/tweets", h.GetUserTweets)
}

func (h *TweetHandler) CreateTweet(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var req models.CreateTweetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	tweet, err := h.svc.CreateTweet(r.Context(), userID, req)
	if err != nil {
		if strings.Contains(err.Error(), "cannot be empty") || strings.Contains(err.Error(), "exceeds") {
			middleware.WriteError(w, http.StatusBadRequest, "invalid_content", err.Error())
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusCreated, tweet)
}

func (h *TweetHandler) GetTweet(w http.ResponseWriter, r *http.Request) {
	tweetID := r.PathValue("id")
	if tweetID == "" {
		middleware.WriteError(w, http.StatusBadRequest, "missing_id", "Tweet ID is required")
		return
	}

	tweet, err := h.svc.GetTweet(r.Context(), tweetID)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			middleware.WriteError(w, http.StatusNotFound, "not_found", "Tweet not found")
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, tweet)
}

func (h *TweetHandler) DeleteTweet(w http.ResponseWriter, r *http.Request) {
	tweetID := r.PathValue("id")
	userID := middleware.GetUserID(r.Context())

	if userID == "" {
		middleware.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	if err := h.svc.DeleteTweet(r.Context(), tweetID, userID); err != nil {
		if strings.Contains(err.Error(), "not authorized") {
			middleware.WriteError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		if strings.Contains(err.Error(), "no rows") {
			middleware.WriteError(w, http.StatusNotFound, "not_found", "Tweet not found")
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *TweetHandler) Like(w http.ResponseWriter, r *http.Request) {
	tweetID := r.PathValue("id")
	userID := middleware.GetUserID(r.Context())

	if userID == "" {
		middleware.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	if err := h.svc.Like(r.Context(), userID, tweetID); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "liked"})
}

func (h *TweetHandler) Unlike(w http.ResponseWriter, r *http.Request) {
	tweetID := r.PathValue("id")
	userID := middleware.GetUserID(r.Context())

	if userID == "" {
		middleware.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	if err := h.svc.Unlike(r.Context(), userID, tweetID); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "unliked"})
}

func (h *TweetHandler) Retweet(w http.ResponseWriter, r *http.Request) {
	tweetID := r.PathValue("id")
	userID := middleware.GetUserID(r.Context())

	if userID == "" {
		middleware.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	retweet, err := h.svc.Retweet(r.Context(), userID, tweetID)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	if retweet == nil {
		middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "already_retweeted"})
		return
	}

	middleware.WriteJSON(w, http.StatusCreated, retweet)
}

func (h *TweetHandler) Unretweet(w http.ResponseWriter, r *http.Request) {
	tweetID := r.PathValue("id")
	userID := middleware.GetUserID(r.Context())

	if userID == "" {
		middleware.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	if err := h.svc.Unretweet(r.Context(), userID, tweetID); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "unretweeted"})
}

func (h *TweetHandler) GetReplies(w http.ResponseWriter, r *http.Request) {
	tweetID := r.PathValue("id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit == 0 {
		limit = 20
	}

	tweets, hasMore, err := h.svc.GetReplies(r.Context(), tweetID, limit, offset)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	resp := models.TimelineResponse{
		Tweets:  tweets,
		HasMore: hasMore,
	}
	if hasMore {
		resp.NextCursor = strconv.Itoa(offset + limit)
	}

	middleware.WriteJSON(w, http.StatusOK, resp)
}

func (h *TweetHandler) GetUserTweets(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit == 0 {
		limit = 20
	}

	tweets, hasMore, err := h.svc.GetTweetsByAuthor(r.Context(), userID, limit, offset)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	resp := models.TimelineResponse{
		Tweets:  tweets,
		HasMore: hasMore,
	}
	if hasMore {
		resp.NextCursor = strconv.Itoa(offset + limit)
	}

	middleware.WriteJSON(w, http.StatusOK, resp)
}

func (h *TweetHandler) BatchGetTweets(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if len(req.IDs) == 0 {
		middleware.WriteJSON(w, http.StatusOK, []models.Tweet{})
		return
	}

	if len(req.IDs) > 100 {
		middleware.WriteError(w, http.StatusBadRequest, "too_many_ids", "Maximum 100 IDs allowed")
		return
	}

	tweets, err := h.svc.BatchGetTweets(r.Context(), req.IDs)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, tweets)
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

func ReadyHandler(w http.ResponseWriter, r *http.Request) {
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
