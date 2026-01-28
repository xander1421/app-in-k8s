package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/alexprut/fileshare/internal/cache"
	"github.com/alexprut/fileshare/internal/config"
	"github.com/alexprut/fileshare/internal/database"
	"github.com/alexprut/fileshare/internal/models"
	"github.com/alexprut/fileshare/internal/queue"
	"github.com/alexprut/fileshare/internal/search"
)

type Handlers struct {
	cfg      *config.Config
	db       *database.PostgresDB
	cache    *cache.RedisCache
	search   *search.ElasticsearchClient
	queue    *queue.RabbitMQ
	startAt  time.Time
	requests int64
}

func NewHandlers(
	cfg *config.Config,
	db *database.PostgresDB,
	cache *cache.RedisCache,
	search *search.ElasticsearchClient,
	queue *queue.RabbitMQ,
) *Handlers {
	return &Handlers{
		cfg:     cfg,
		db:      db,
		cache:   cache,
		search:  search,
		queue:   queue,
		startAt: time.Now(),
	}
}

// Router creates the HTTP router
func (h *Handlers) Router() http.Handler {
	mux := http.NewServeMux()

	// Health endpoints
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /health/ready", h.Ready)

	// Cluster info
	mux.HandleFunc("GET /cluster", h.ClusterInfo)

	// File operations
	mux.HandleFunc("POST /files", h.rateLimit(h.UploadFile))
	mux.HandleFunc("GET /files", h.ListFiles)
	mux.HandleFunc("GET /files/{id}", h.GetFile)
	mux.HandleFunc("GET /files/{id}/download", h.DownloadFile)
	mux.HandleFunc("DELETE /files/{id}", h.DeleteFile)

	// Search
	mux.HandleFunc("GET /search", h.Search)

	// Share links
	mux.HandleFunc("POST /files/{id}/share", h.CreateShareLink)
	mux.HandleFunc("GET /share/{token}", h.AccessShareLink)

	// Queue stats
	mux.HandleFunc("GET /queues", h.QueueStats)

	return h.middleware(mux)
}

func (h *Handlers) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&h.requests, 1)

		// Log request
		start := time.Now()
		log.Printf("[%s] %s %s", h.cfg.InstanceID[:8], r.Method, r.URL.Path)

		// Set headers
		w.Header().Set("X-Instance-ID", h.cfg.InstanceID)
		w.Header().Set("X-Protocol", r.Proto)

		next.ServeHTTP(w, r)

		log.Printf("[%s] %s %s took %v", h.cfg.InstanceID[:8], r.Method, r.URL.Path, time.Since(start))
	})
}

func (h *Handlers) rateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.cache != nil {
			// Use client IP for rate limiting
			key := "upload:" + r.RemoteAddr
			allowed, remaining, err := h.cache.CheckRateLimit(r.Context(), key, 10, time.Minute)
			if err != nil {
				log.Printf("Rate limit check error: %v", err)
			}

			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			if !allowed {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
		}
		next(w, r)
	}
}

// ============== Health Endpoints ==============

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	h.json(w, http.StatusOK, map[string]interface{}{
		"status":      "ok",
		"instance_id": h.cfg.InstanceID,
		"protocol":    r.Proto,
		"timestamp":   time.Now().Format(time.RFC3339),
	})
}

func (h *Handlers) Ready(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	checks := map[string]interface{}{}
	allHealthy := true

	// PostgreSQL
	if err := h.db.Health(ctx); err != nil {
		checks["postgresql"] = map[string]interface{}{"healthy": false, "error": err.Error()}
		allHealthy = false
	} else {
		checks["postgresql"] = map[string]interface{}{"healthy": true}
	}

	// Redis
	if h.cache != nil {
		if err := h.cache.Health(ctx); err != nil {
			checks["redis"] = map[string]interface{}{"healthy": false, "error": err.Error()}
			// Redis is optional, don't fail readiness
		} else {
			checks["redis"] = map[string]interface{}{"healthy": true}
		}
	} else {
		checks["redis"] = map[string]interface{}{"healthy": false, "error": "not configured"}
	}

	// Elasticsearch
	if h.search != nil {
		if err := h.search.Health(ctx); err != nil {
			checks["elasticsearch"] = map[string]interface{}{"healthy": false, "error": err.Error()}
			// ES is optional, don't fail readiness
		} else {
			checks["elasticsearch"] = map[string]interface{}{"healthy": true}
		}
	}

	// RabbitMQ
	if h.queue != nil {
		if err := h.queue.Health(ctx); err != nil {
			checks["rabbitmq"] = map[string]interface{}{"healthy": false, "error": err.Error()}
			// RabbitMQ is optional for readiness
		} else {
			checks["rabbitmq"] = map[string]interface{}{"healthy": true}
		}
	}

	status := http.StatusOK
	if !allHealthy {
		status = http.StatusServiceUnavailable
	}

	h.json(w, status, map[string]interface{}{
		"status":    map[bool]string{true: "ready", false: "not_ready"}[allHealthy],
		"checks":    checks,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// ============== Cluster Info ==============

func (h *Handlers) ClusterInfo(w http.ResponseWriter, r *http.Request) {
	info := models.ClusterInfo{
		InstanceID:    h.cfg.InstanceID,
		UptimeSeconds: int64(time.Since(h.startAt).Seconds()),
	}
	if h.cache != nil {
		info.ActivePeers = h.cache.GetActivePeers()
	}

	h.json(w, http.StatusOK, info)
}

// ============== File Operations ==============

func (h *Handlers) UploadFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse multipart form
	if err := r.ParseMultipartForm(h.cfg.MaxUploadSize); err != nil {
		http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Get user (simplified - use X-User-ID header)
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "anonymous"
	}

	// Ensure user exists
	user, err := h.db.GetOrCreateUser(ctx, userID, userID+"@example.com")
	if err != nil {
		log.Printf("Create user error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Generate file ID and path
	fileID := uuid.New().String()
	filePath := filepath.Join(h.cfg.UploadDir, fileID)

	// Use distributed lock to prevent race conditions (if Redis available)
	if h.cache != nil {
		lockKey := "upload:" + header.Filename + ":" + user.ID
		acquired, err := h.cache.AcquireLock(ctx, lockKey, 30*time.Second)
		if err != nil || !acquired {
			http.Error(w, "upload in progress", http.StatusConflict)
			return
		}
		defer h.cache.ReleaseLock(ctx, lockKey)
	}

	// Create file on disk
	if err := os.MkdirAll(h.cfg.UploadDir, 0755); err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}

	dst, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	// Copy and calculate checksum
	hasher := sha256.New()
	size, err := io.Copy(io.MultiWriter(dst, hasher), file)
	if err != nil {
		os.Remove(filePath)
		http.Error(w, "upload failed", http.StatusInternalServerError)
		return
	}

	// Create file record
	now := time.Now()
	tags := strings.Split(r.FormValue("tags"), ",")
	if len(tags) == 1 && tags[0] == "" {
		tags = nil
	}

	f := &models.File{
		ID:          fileID,
		Name:        header.Filename,
		Size:        size,
		ContentType: header.Header.Get("Content-Type"),
		Checksum:    hex.EncodeToString(hasher.Sum(nil)),
		OwnerID:     user.ID,
		Path:        filePath,
		Tags:        tags,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Save to PostgreSQL
	if err := h.db.CreateFile(ctx, f); err != nil {
		os.Remove(filePath)
		log.Printf("DB error: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	// Index in Elasticsearch
	if h.search != nil {
		if err := h.search.IndexFile(ctx, f); err != nil {
			log.Printf("ES index error: %v", err)
		}
	}

	// Queue async processing
	if h.queue != nil {
		h.queue.PublishProcessingJob(ctx, f.ID, f.Path, f.ContentType)
		if strings.HasPrefix(f.ContentType, "image/") {
			h.queue.PublishThumbnailJob(ctx, f.ID, f.Path)
		}
	}

	// Publish event to all instances via Redis
	if h.cache != nil {
		h.cache.PublishFileEvent(ctx, models.FileEvent{
			Type:     "upload",
			FileID:   f.ID,
			FileName: f.Name,
			UserID:   user.ID,
		})
	}

	h.json(w, http.StatusCreated, f)
}

func (h *Handlers) ListFiles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "anonymous"
	}

	user, err := h.db.GetOrCreateUser(ctx, userID, userID+"@example.com")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, _ := strconv.Atoi(l); v > 0 && v <= 100 {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, _ := strconv.Atoi(o); v > 0 {
			offset = v
		}
	}

	// Try cache first
	cacheKey := fmt.Sprintf("files:%s:%d:%d", user.ID, limit, offset)
	var files []models.File
	if h.cache != nil {
		if err := h.cache.Get(ctx, cacheKey, &files); err == nil {
			w.Header().Set("X-Cache", "HIT")
			h.json(w, http.StatusOK, files)
			return
		}
	}

	files, err = h.db.ListFiles(ctx, user.ID, limit, offset)
	if err != nil {
		log.Printf("List files error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Cache for 1 minute
	if h.cache != nil {
		h.cache.Set(ctx, cacheKey, files, time.Minute)
	}

	w.Header().Set("X-Cache", "MISS")
	h.json(w, http.StatusOK, files)
}

func (h *Handlers) GetFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	f, err := h.db.GetFile(ctx, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	h.json(w, http.StatusOK, f)
}

func (h *Handlers) DownloadFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	f, err := h.db.GetFile(ctx, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Increment downloads
	h.db.IncrementDownloads(ctx, id)

	// Publish download event
	if h.cache != nil {
		h.cache.PublishFileEvent(ctx, models.FileEvent{
			Type:     "download",
			FileID:   f.ID,
			FileName: f.Name,
			UserID:   r.Header.Get("X-User-ID"),
		})
	}

	w.Header().Set("Content-Type", f.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", f.Name))
	http.ServeFile(w, r, f.Path)
}

func (h *Handlers) DeleteFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	f, err := h.db.GetFile(ctx, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Delete from disk
	os.Remove(f.Path)

	// Delete from DB
	if err := h.db.DeleteFile(ctx, id); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Delete from search index
	if h.search != nil {
		h.search.DeleteFile(ctx, id)
	}

	// Publish event
	if h.cache != nil {
		h.cache.PublishFileEvent(ctx, models.FileEvent{
			Type:     "delete",
			FileID:   f.ID,
			FileName: f.Name,
			UserID:   r.Header.Get("X-User-ID"),
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

// ============== Search ==============

func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "missing query parameter 'q'", http.StatusBadRequest)
		return
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		userID = "anonymous"
	}

	user, _ := h.db.GetOrCreateUser(ctx, userID, userID+"@example.com")

	if h.search == nil {
		http.Error(w, "search not available", http.StatusServiceUnavailable)
		return
	}

	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, _ := strconv.Atoi(l); v > 0 && v <= 100 {
			limit = v
		}
	}

	result, err := h.search.Search(ctx, query, user.ID, limit, offset)
	if err != nil {
		log.Printf("Search error: %v", err)
		http.Error(w, "search error", http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, result)
}

// ============== Share Links ==============

func (h *Handlers) CreateShareLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	f, err := h.db.GetFile(ctx, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var req struct {
		ExpiresIn *int `json:"expires_in"` // seconds
		MaxUses   *int `json:"max_uses"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	token := uuid.New().String()[:16]
	var expiresAt *time.Time
	if req.ExpiresIn != nil {
		t := time.Now().Add(time.Duration(*req.ExpiresIn) * time.Second)
		expiresAt = &t
	}

	link := &models.ShareLink{
		ID:        uuid.New().String(),
		FileID:    f.ID,
		Token:     token,
		ExpiresAt: expiresAt,
		MaxUses:   req.MaxUses,
		CreatedAt: time.Now(),
	}

	if err := h.db.CreateShareLink(ctx, link); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Publish share event
	if h.cache != nil {
		h.cache.PublishFileEvent(ctx, models.FileEvent{
			Type:     "share",
			FileID:   f.ID,
			FileName: f.Name,
			UserID:   r.Header.Get("X-User-ID"),
			Payload:  map[string]string{"token": token},
		})
	}

	h.json(w, http.StatusCreated, map[string]interface{}{
		"token":      token,
		"url":        fmt.Sprintf("/share/%s", token),
		"expires_at": expiresAt,
		"max_uses":   req.MaxUses,
	})
}

func (h *Handlers) AccessShareLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := r.PathValue("token")

	link, err := h.db.GetShareLinkByToken(ctx, token)
	if err != nil {
		http.Error(w, "invalid or expired link", http.StatusNotFound)
		return
	}

	// Check expiration
	if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
		http.Error(w, "link expired", http.StatusGone)
		return
	}

	// Check uses
	if link.MaxUses != nil && link.Uses >= *link.MaxUses {
		http.Error(w, "link exhausted", http.StatusGone)
		return
	}

	// Increment uses
	h.db.IncrementShareLinkUses(ctx, link.ID)

	f, err := h.db.GetFile(ctx, link.FileID)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	// Serve file
	w.Header().Set("Content-Type", f.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", f.Name))
	http.ServeFile(w, r, f.Path)
}

// ============== Queue Stats ==============

func (h *Handlers) QueueStats(w http.ResponseWriter, r *http.Request) {
	if h.queue == nil {
		http.Error(w, "queues not available", http.StatusServiceUnavailable)
		return
	}

	stats := map[string]interface{}{}

	for _, q := range []string{queue.QueueThumbnails, queue.QueueProcessing, queue.QueueNotify} {
		msgs, consumers, err := h.queue.GetQueueStats(q)
		if err != nil {
			stats[q] = map[string]interface{}{"error": err.Error()}
		} else {
			stats[q] = map[string]interface{}{
				"messages":  msgs,
				"consumers": consumers,
			}
		}
	}

	h.json(w, http.StatusOK, stats)
}

// ============== Helpers ==============

func (h *Handlers) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
