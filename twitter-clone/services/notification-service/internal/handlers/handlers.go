package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/alexprut/twitter-clone/pkg/middleware"
	"github.com/alexprut/twitter-clone/services/notification-service/internal/service"
)

type NotificationHandler struct {
	svc *service.NotificationService
}

func NewNotificationHandler(svc *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

func (h *NotificationHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/notifications", h.GetNotifications)
	mux.HandleFunc("GET /api/v1/notifications/unread", h.GetUnreadNotifications)
	mux.HandleFunc("GET /api/v1/notifications/count", h.GetUnreadCount)
	mux.HandleFunc("POST /api/v1/notifications", h.CreateNotification)
	mux.HandleFunc("PUT /api/v1/notifications/{id}/read", h.MarkAsRead)
	mux.HandleFunc("PUT /api/v1/notifications/read-all", h.MarkAllAsRead)
	mux.HandleFunc("DELETE /api/v1/notifications/{id}", h.DeleteNotification)
}

func (h *NotificationHandler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	notifications, hasMore, err := h.svc.GetNotifications(r.Context(), userID, limit, offset)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	resp := map[string]interface{}{
		"notifications": notifications,
		"has_more":      hasMore,
	}
	if hasMore {
		resp["next_cursor"] = strconv.Itoa(offset + limit)
	}

	middleware.WriteJSON(w, http.StatusOK, resp)
}

func (h *NotificationHandler) GetUnreadNotifications(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	notifications, hasMore, err := h.svc.GetUnreadNotifications(r.Context(), userID, limit, offset)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	resp := map[string]interface{}{
		"notifications": notifications,
		"has_more":      hasMore,
	}

	middleware.WriteJSON(w, http.StatusOK, resp)
}

func (h *NotificationHandler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	count, err := h.svc.GetUnreadCount(r.Context(), userID)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]int{"count": count})
}

func (h *NotificationHandler) CreateNotification(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID   string                 `json:"user_id"`
		Type     string                 `json:"type"`
		ActorID  string                 `json:"actor_id"`
		TweetID  string                 `json:"tweet_id,omitempty"`
		Data     map[string]interface{} `json:"data,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	notif, err := h.svc.CreateNotification(r.Context(), req.UserID, req.Type, req.ActorID, req.TweetID, req.Data)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	if notif == nil {
		middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "skipped"})
		return
	}

	middleware.WriteJSON(w, http.StatusCreated, notif)
}

func (h *NotificationHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	notifID := r.PathValue("id")
	if err := h.svc.MarkAsRead(r.Context(), notifID, userID); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "marked_as_read"})
}

func (h *NotificationHandler) MarkAllAsRead(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	if err := h.svc.MarkAllAsRead(r.Context(), userID); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "all_marked_as_read"})
}

func (h *NotificationHandler) DeleteNotification(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	notifID := r.PathValue("id")
	if err := h.svc.Delete(r.Context(), notifID, userID); err != nil {
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
