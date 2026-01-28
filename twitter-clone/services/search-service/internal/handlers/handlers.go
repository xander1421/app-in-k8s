package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/alexprut/twitter-clone/pkg/middleware"
	"github.com/alexprut/twitter-clone/pkg/models"
	"github.com/alexprut/twitter-clone/services/search-service/internal/service"
)

type SearchHandler struct {
	svc *service.SearchService
}

func NewSearchHandler(svc *service.SearchService) *SearchHandler {
	return &SearchHandler{svc: svc}
}

func (h *SearchHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/search/tweets", h.SearchTweets)
	mux.HandleFunc("GET /api/v1/search/users", h.SearchUsers)
	mux.HandleFunc("GET /api/v1/trending", h.GetTrending)
	mux.HandleFunc("POST /api/v1/search/index/tweet", h.IndexTweet)
	mux.HandleFunc("POST /api/v1/search/index/user", h.IndexUser)
	mux.HandleFunc("DELETE /api/v1/search/index/tweet/{id}", h.DeleteTweet)
}

func (h *SearchHandler) SearchTweets(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		middleware.WriteError(w, http.StatusBadRequest, "missing_query", "Search query is required")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	result, err := h.svc.SearchTweets(r.Context(), query, limit, offset)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, result)
}

func (h *SearchHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		middleware.WriteError(w, http.StatusBadRequest, "missing_query", "Search query is required")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	result, err := h.svc.SearchUsers(r.Context(), query, limit, offset)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, result)
}

func (h *SearchHandler) GetTrending(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 10
	}

	trending, err := h.svc.GetTrending(r.Context(), limit)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"trending": trending,
	})
}

func (h *SearchHandler) IndexTweet(w http.ResponseWriter, r *http.Request) {
	var tweet models.Tweet
	if err := json.NewDecoder(r.Body).Decode(&tweet); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.svc.IndexTweet(r.Context(), &tweet); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "indexed"})
}

func (h *SearchHandler) IndexUser(w http.ResponseWriter, r *http.Request) {
	var user models.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.svc.IndexUser(r.Context(), &user); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "indexed"})
}

func (h *SearchHandler) DeleteTweet(w http.ResponseWriter, r *http.Request) {
	tweetID := r.PathValue("id")
	if tweetID == "" {
		middleware.WriteError(w, http.StatusBadRequest, "missing_id", "Tweet ID is required")
		return
	}

	if err := h.svc.DeleteTweet(r.Context(), tweetID); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

func ReadyHandler(w http.ResponseWriter, r *http.Request) {
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
