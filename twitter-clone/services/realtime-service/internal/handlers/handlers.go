package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/alexprut/twitter-clone/pkg/middleware"
	"github.com/alexprut/twitter-clone/pkg/sse"
	"github.com/alexprut/twitter-clone/services/realtime-service/internal/service"
)

// RealtimeHandler handles HTTP requests for realtime service
type RealtimeHandler struct {
	hub     *sse.Hub
	service *service.RealtimeService
}

// NewRealtimeHandler creates a new realtime handler
func NewRealtimeHandler(hub *sse.Hub, service *service.RealtimeService) *RealtimeHandler {
	return &RealtimeHandler{
		hub:     hub,
		service: service,
	}
}

// RegisterRoutes registers HTTP routes
func (h *RealtimeHandler) RegisterRoutes(mux *http.ServeMux) {
	// Status endpoints
	mux.HandleFunc("GET /api/v1/realtime/status", h.GetStatus)
	mux.HandleFunc("GET /api/v1/realtime/online", h.GetOnlineUsers)
	mux.HandleFunc("GET /api/v1/realtime/presence/{userID}", h.GetUserPresence)
	
	// Room endpoints
	mux.HandleFunc("POST /api/v1/realtime/rooms", h.CreateRoom)
	mux.HandleFunc("POST /api/v1/realtime/rooms/{roomID}/message", h.SendRoomMessage)
	mux.HandleFunc("GET /api/v1/realtime/rooms/{roomID}/history", h.GetRoomHistory)
	
	// Admin endpoints
	mux.HandleFunc("POST /api/v1/realtime/broadcast", h.BroadcastMessage)
}

// GetStatus returns realtime service status
func (h *RealtimeHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	stats := h.hub.GetStats()
	status := map[string]interface{}{
		"status": "healthy",
		"stats":  stats,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// GetOnlineUsers returns list of online users
func (h *RealtimeHandler) GetOnlineUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.service.GetOnlineUsers(r.Context())
	if err != nil {
		http.Error(w, `{"error": "failed to get online users"}`, http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users": users,
		"count": len(users),
	})
}

// GetUserPresence returns user's presence status
func (h *RealtimeHandler) GetUserPresence(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userID")
	if userID == "" {
		http.Error(w, `{"error": "user ID required"}`, http.StatusBadRequest)
		return
	}
	
	status, err := h.service.GetUserPresence(r.Context(), userID)
	if err != nil {
		status = "offline"
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"user_id": userID,
		"status":  status,
	})
}

// CreateRoom creates a new chat room
func (h *RealtimeHandler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	// Check auth
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}
	
	var req struct {
		RoomID  string   `json:"room_id"`
		UserIDs []string `json:"user_ids"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
		return
	}
	
	// Ensure creator is in the room
	found := false
	for _, id := range req.UserIDs {
		if id == userID {
			found = true
			break
		}
	}
	if !found {
		req.UserIDs = append(req.UserIDs, userID)
	}
	
	if err := h.service.CreateRoom(r.Context(), req.RoomID, req.UserIDs); err != nil {
		http.Error(w, `{"error": "failed to create room"}`, http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"room_id": req.RoomID,
		"members": req.UserIDs,
		"created": true,
	})
}

// SendRoomMessage sends a message to a room
func (h *RealtimeHandler) SendRoomMessage(w http.ResponseWriter, r *http.Request) {
	// Check auth
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}
	
	roomID := r.PathValue("roomID")
	if roomID == "" {
		http.Error(w, `{"error": "room ID required"}`, http.StatusBadRequest)
		return
	}
	
	var req struct {
		Message string `json:"message"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
		return
	}
	
	if err := h.service.BroadcastToRoom(r.Context(), roomID, userID, req.Message); err != nil {
		http.Error(w, `{"error": "failed to send message"}`, http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sent": true,
	})
}

// GetRoomHistory returns chat history for a room
func (h *RealtimeHandler) GetRoomHistory(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("roomID")
	if roomID == "" {
		http.Error(w, `{"error": "room ID required"}`, http.StatusBadRequest)
		return
	}
	
	// This would fetch from Redis or database
	// For now, return empty history
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"room_id":  roomID,
		"messages": []interface{}{},
	})
}

// BroadcastMessage broadcasts a system message (admin only)
func (h *RealtimeHandler) BroadcastMessage(w http.ResponseWriter, r *http.Request) {
	// Check admin auth (simplified for now)
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		return
	}
	
	var req struct {
		Message string `json:"message"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
		return
	}
	
	h.service.SendSystemMessage(r.Context(), req.Message)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"broadcast": true,
		"message":   req.Message,
	})
}

// HealthHandler returns health status
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}

// ReadyHandler returns readiness status
func ReadyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ready",
	})
}