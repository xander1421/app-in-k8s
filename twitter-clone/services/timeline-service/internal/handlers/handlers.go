package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/alexprut/twitter-clone/pkg/middleware"
	"github.com/alexprut/twitter-clone/services/timeline-service/internal/service"
)

type TimelineHandler struct {
	svc *service.TimelineService
}

func NewTimelineHandler(svc *service.TimelineService) *TimelineHandler {
	return &TimelineHandler{svc: svc}
}

func (h *TimelineHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/timeline/home", h.GetHomeTimeline)
	mux.HandleFunc("GET /api/v1/timeline/user/{id}", h.GetUserTimeline)
	mux.HandleFunc("POST /api/v1/timeline/add", h.AddToTimeline)
}

func (h *TimelineHandler) GetHomeTimeline(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	resp, err := h.svc.GetHomeTimeline(r.Context(), userID, limit, offset)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, resp)
}

func (h *TimelineHandler) GetUserTimeline(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if userID == "" {
		middleware.WriteError(w, http.StatusBadRequest, "missing_id", "User ID is required")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	resp, err := h.svc.GetUserTimeline(r.Context(), userID, limit, offset)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, resp)
}

func (h *TimelineHandler) AddToTimeline(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID  string  `json:"user_id"`
		TweetID string  `json:"tweet_id"`
		Score   float64 `json:"score"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if err := h.svc.AddToTimeline(r.Context(), req.UserID, req.TweetID, req.Score); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "added"})
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

func ReadyHandler(w http.ResponseWriter, r *http.Request) {
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
