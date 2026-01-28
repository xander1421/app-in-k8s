package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/alexprut/twitter-clone/pkg/middleware"
	"github.com/alexprut/twitter-clone/services/media-service/internal/service"
)

type MediaHandler struct {
	svc *service.MediaService
}

func NewMediaHandler(svc *service.MediaService) *MediaHandler {
	return &MediaHandler{svc: svc}
}

func (h *MediaHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/media/upload", h.Upload)
	mux.HandleFunc("POST /api/v1/media/presign", h.GetPresignedUploadURL)
	mux.HandleFunc("GET /api/v1/media/{id}", h.GetMedia)
	mux.HandleFunc("DELETE /api/v1/media/{id}", h.DeleteMedia)
}

func (h *MediaHandler) Upload(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB max
		middleware.WriteError(w, http.StatusBadRequest, "invalid_request", "Failed to parse multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "missing_file", "No file provided")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	media, err := h.svc.Upload(r.Context(), userID, contentType, file, header.Size)
	if err != nil {
		if strings.Contains(err.Error(), "unsupported") || strings.Contains(err.Error(), "too large") {
			middleware.WriteError(w, http.StatusBadRequest, "invalid_file", err.Error())
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusCreated, media)
}

func (h *MediaHandler) GetPresignedUploadURL(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var req struct {
		ContentType string `json:"content_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	uploadURL, mediaID, err := h.svc.GetPresignedUploadURL(r.Context(), userID, req.ContentType)
	if err != nil {
		if strings.Contains(err.Error(), "unsupported") {
			middleware.WriteError(w, http.StatusBadRequest, "invalid_content_type", err.Error())
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, map[string]string{
		"upload_url": uploadURL,
		"media_id":   mediaID,
	})
}

func (h *MediaHandler) GetMedia(w http.ResponseWriter, r *http.Request) {
	mediaID := r.PathValue("id")
	if mediaID == "" {
		middleware.WriteError(w, http.StatusBadRequest, "missing_id", "Media ID is required")
		return
	}

	// For now, we need the uploader ID to find the media
	// In a real implementation, we'd look this up from a database
	uploaderID := r.URL.Query().Get("uploader_id")
	if uploaderID == "" {
		uploaderID = middleware.GetUserID(r.Context())
	}

	media, err := h.svc.GetMedia(r.Context(), mediaID, uploaderID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			middleware.WriteError(w, http.StatusNotFound, "not_found", "Media not found")
			return
		}
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusOK, media)
}

func (h *MediaHandler) DeleteMedia(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	mediaID := r.PathValue("id")
	if mediaID == "" {
		middleware.WriteError(w, http.StatusBadRequest, "missing_id", "Media ID is required")
		return
	}

	if err := h.svc.Delete(r.Context(), userID, mediaID); err != nil {
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
