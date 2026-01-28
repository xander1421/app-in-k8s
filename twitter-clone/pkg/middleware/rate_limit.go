package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// RateLimiter tracks request counts per client
type RateLimiter struct {
	clients map[string]*Client
	mu      sync.RWMutex
	limit   int
	window  time.Duration
}

// Client tracks request timestamps
type Client struct {
	timestamps []time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		clients: make(map[string]*Client),
		limit:   limit,
		window:  window,
	}

	// Cleanup old entries every minute
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			rl.cleanup()
		}
	}()

	return rl
}

// Allow checks if request is allowed
func (rl *RateLimiter) Allow(clientID string) bool {
	rl.mu.RLock()
	client, exists := rl.clients[clientID]
	rl.mu.RUnlock()
	
	if !exists {
		rl.mu.Lock()
		// Double check after acquiring write lock
		client, exists = rl.clients[clientID]
		if !exists {
			client = &Client{
				timestamps: make([]time.Time, 0, rl.limit),
			}
			rl.clients[clientID] = client
		}
		rl.mu.Unlock()
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Remove old timestamps
	validTimestamps := make([]time.Time, 0, len(client.timestamps))
	for _, ts := range client.timestamps {
		if ts.After(cutoff) {
			validTimestamps = append(validTimestamps, ts)
		}
	}
	client.timestamps = validTimestamps

	// Check if limit exceeded
	if len(client.timestamps) >= rl.limit {
		return false
	}

	// Add current timestamp
	client.timestamps = append(client.timestamps, now)
	return true
}

// cleanup removes old client entries
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.window * 2)
	for clientID, client := range rl.clients {
		client.mu.Lock()
		if len(client.timestamps) == 0 || client.timestamps[len(client.timestamps)-1].Before(cutoff) {
			delete(rl.clients, clientID)
		}
		client.mu.Unlock()
	}
}

// RateLimit creates a rate limiting middleware
func RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	limiter := NewRateLimiter(limit, window)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip rate limiting for health checks
			if r.URL.Path == "/health" || r.URL.Path == "/ready" {
				next.ServeHTTP(w, r)
				return
			}

			// Use IP address as client identifier
			clientID := r.RemoteAddr
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				clientID = xff
			}

			// Check user ID from context for authenticated requests
			if userID := GetUserID(r.Context()); userID != "" {
				clientID = fmt.Sprintf("user:%s", userID)
			}

			// Check rate limit
			if !limiter.Allow(clientID) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
				w.Header().Set("X-RateLimit-Window", window.String())
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error": "rate limit exceeded"}`))
				return
			}

			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
			w.Header().Set("X-RateLimit-Window", window.String())

			next.ServeHTTP(w, r)
		})
	}
}

// TokenBucket implements a token bucket rate limiter
type TokenBucket struct {
	tokens   float64
	capacity float64
	refill   float64
	lastTime time.Time
	mu       sync.Mutex
}

// NewTokenBucket creates a new token bucket
func NewTokenBucket(capacity, refillRate float64) *TokenBucket {
	return &TokenBucket{
		tokens:   capacity,
		capacity: capacity,
		refill:   refillRate,
		lastTime: time.Now(),
	}
}

// Allow checks if request is allowed using token bucket algorithm
func (tb *TokenBucket) Allow(tokens float64) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastTime).Seconds()
	tb.lastTime = now

	// Refill tokens
	tb.tokens = tb.tokens + elapsed*tb.refill
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}

	// Check if enough tokens
	if tb.tokens < tokens {
		return false
	}

	tb.tokens -= tokens
	return true
}

// AdaptiveRateLimit creates an adaptive rate limiter that adjusts based on server load
func AdaptiveRateLimit(baseLimit int, window time.Duration) func(http.Handler) http.Handler {
	limiter := NewRateLimiter(baseLimit, window)
	
	// Track server metrics
	var (
		requestCount int64
		errorCount   int64
		mu           sync.RWMutex
	)

	// Adjust limits periodically
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			mu.RLock()
			errorRate := float64(errorCount) / float64(requestCount+1)
			mu.RUnlock()

			// Adjust limit based on error rate
			if errorRate > 0.1 {
				// High error rate, reduce limit
				limiter.limit = int(float64(baseLimit) * 0.8)
			} else if errorRate < 0.01 {
				// Low error rate, increase limit
				limiter.limit = int(float64(baseLimit) * 1.2)
			} else {
				// Normal error rate
				limiter.limit = baseLimit
			}

			// Reset counters
			mu.Lock()
			requestCount = 0
			errorCount = 0
			mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Track request
			mu.Lock()
			requestCount++
			mu.Unlock()

			// Create response writer wrapper to track status
			wrapper := &responseWriter{
				ResponseWriter: w,
				status:         http.StatusOK,
			}

			// Apply rate limiting
			RateLimit(limiter.limit, window)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(wrapper, r)

				// Track errors
				if wrapper.status >= 500 {
					mu.Lock()
					errorCount++
					mu.Unlock()
				}
			})).ServeHTTP(w, r)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}