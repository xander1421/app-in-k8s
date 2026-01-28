package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/alexprut/twitter-clone/pkg/middleware"
	"github.com/alexprut/twitter-clone/pkg/models"
	"github.com/alexprut/twitter-clone/services/user-service/internal/service"
)

type UserHandler struct {
	svc *service.UserService
}

func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/users", h.CreateUser)
	mux.HandleFunc("GET /api/v1/users/{id}", h.GetUser)
	mux.HandleFunc("PUT /api/v1/users/{id}", h.UpdateUser)
	mux.HandleFunc("POST /api/v1/users/{id}/follow", h.Follow)
	mux.HandleFunc("DELETE /api/v1/users/{id}/follow", h.Unfollow)
	mux.HandleFunc("GET /api/v1/users/{id}/followers", h.GetFollowers)
	mux.HandleFunc("GET /api/v1/users/{id}/following", h.GetFollowing)
	mux.HandleFunc("GET /api/v1/users/{id}/follower-ids", h.GetFollowerIDs)
	mux.HandleFunc("GET /api/v1/lookup/username/{username}", h.GetUserByUsername)
}

func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req models.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.Username == "" || req.Email == "" {
		middleware.WriteError(w, http.StatusBadRequest, "missing_fields", "Username and email are required")
		return
	}

	user, err := h.svc.CreateUser(r.Context(), req)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			middleware.WriteError(w, http.StatusConflict, "user_exists", "Username or email already exists")
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusCreated, user)
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if userID == "" {
		middleware.WriteError(w, http.StatusBadRequest, "missing_id", "User ID is required")
		return
	}

	user, err := h.svc.GetUser(r.Context(), userID)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			middleware.WriteError(w, http.StatusNotFound, "not_found", "User not found")
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, user)
}

func (h *UserHandler) GetUserByUsername(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if username == "" {
		middleware.WriteError(w, http.StatusBadRequest, "missing_username", "Username is required")
		return
	}

	user, err := h.svc.GetUserByUsername(r.Context(), username)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			middleware.WriteError(w, http.StatusNotFound, "not_found", "User not found")
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, user)
}

func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	currentUserID := middleware.GetUserID(r.Context())

	if currentUserID == "" {
		middleware.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	if userID != currentUserID {
		middleware.WriteError(w, http.StatusForbidden, "forbidden", "Cannot update another user's profile")
		return
	}

	var req models.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	user, err := h.svc.UpdateUser(r.Context(), userID, req)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, user)
}

func (h *UserHandler) Follow(w http.ResponseWriter, r *http.Request) {
	followeeID := r.PathValue("id")
	followerID := middleware.GetUserID(r.Context())

	if followerID == "" {
		middleware.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	if err := h.svc.Follow(r.Context(), followerID, followeeID); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "followed"})
}

func (h *UserHandler) Unfollow(w http.ResponseWriter, r *http.Request) {
	followeeID := r.PathValue("id")
	followerID := middleware.GetUserID(r.Context())

	if followerID == "" {
		middleware.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	if err := h.svc.Unfollow(r.Context(), followerID, followeeID); err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "unfollowed"})
}

func (h *UserHandler) GetFollowers(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit == 0 {
		limit = 20
	}

	users, hasMore, err := h.svc.GetFollowers(r.Context(), userID, limit, offset)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	resp := models.FollowersResponse{
		Users:   users,
		HasMore: hasMore,
	}
	if hasMore {
		resp.NextCursor = strconv.Itoa(offset + limit)
	}

	middleware.WriteJSON(w, http.StatusOK, resp)
}

func (h *UserHandler) GetFollowing(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit == 0 {
		limit = 20
	}

	users, hasMore, err := h.svc.GetFollowing(r.Context(), userID, limit, offset)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	resp := models.FollowersResponse{
		Users:   users,
		HasMore: hasMore,
	}
	if hasMore {
		resp.NextCursor = strconv.Itoa(offset + limit)
	}

	middleware.WriteJSON(w, http.StatusOK, resp)
}

func (h *UserHandler) GetFollowerIDs(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")

	ids, err := h.svc.GetFollowerIDs(r.Context(), userID)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, ids)
}

// Health check handler
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

// Ready check handler
func ReadyHandler(w http.ResponseWriter, r *http.Request) {
	middleware.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
