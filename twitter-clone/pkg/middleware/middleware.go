package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/alexprut/twitter-clone/pkg/cache"
	"github.com/alexprut/twitter-clone/pkg/models"
)

type contextKey string

const (
	UserIDKey contextKey = "user_id"
)

// Logger logs HTTP requests
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		log.Printf("%s %s %d %s",
			r.Method,
			r.URL.Path,
			wrapped.statusCode,
			time.Since(start),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Recovery recovers from panics
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %v\n%s", err, debug.Stack())
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// CORS adds CORS headers
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-User-ID")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// JSON sets JSON content type
func JSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// RateLimiter provides rate limiting using Redis
type RateLimiter struct {
	cache  *cache.RedisCache
	limit  int
	window time.Duration
}

func NewRateLimiter(c *cache.RedisCache, limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		cache:  c,
		limit:  limit,
		window: window,
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
		key := ip + ":" + r.URL.Path

		allowed, remaining, err := rl.cache.CheckRateLimit(r.Context(), key, rl.limit, rl.window)
		if err != nil {
			log.Printf("rate limit error: %v", err)
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.limit))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

		if !allowed {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(rl.window.Seconds())))
			WriteError(w, http.StatusTooManyRequests, "rate_limit_exceeded", "Too many requests")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Auth extracts user ID from header (simplified - no JWT for demo)
func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				userID = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		if userID != "" {
			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}

// RequireAuth ensures user is authenticated
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := GetUserID(r.Context())
		if userID == "" {
			WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GetUserID extracts user ID from context
func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		return userID
	}
	return ""
}

// SetUserID sets user ID in context (used for testing)
func SetUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// WriteError writes a JSON error response
func WriteError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(models.ErrorResponse{
		Error: message,
		Code:  code,
	})
}

// WriteJSON writes a JSON response
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func getClientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	parts := strings.Split(r.RemoteAddr, ":")
	if len(parts) > 0 {
		return parts[0]
	}
	return r.RemoteAddr
}
