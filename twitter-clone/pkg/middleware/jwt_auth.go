package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/alexprut/twitter-clone/pkg/auth"
)

// ContextKey type for context keys
type ContextKey string

const (
	// UserIDKey is the context key for user ID
	UserIDKey ContextKey = "userID"
	// UsernameKey is the context key for username
	UsernameKey ContextKey = "username"
	// EmailKey is the context key for email
	EmailKey ContextKey = "email"
)

// JWTAuth middleware validates JWT tokens
func JWTAuth(jwtManager *auth.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for health and ready endpoints
			if r.URL.Path == "/health" || r.URL.Path == "/ready" {
				next.ServeHTTP(w, r)
				return
			}

			// Skip auth for login and register endpoints
			if r.URL.Path == "/api/v1/auth/login" || r.URL.Path == "/api/v1/auth/register" || r.URL.Path == "/api/v1/auth/refresh" ||
			   r.URL.Path == "/api/auth/login" || r.URL.Path == "/api/auth/signup" {
				next.ServeHTTP(w, r)
				return
			}

			// Skip auth for static UI files and WebTransport
			if r.URL.Path == "/" || r.URL.Path == "/style.css" || r.URL.Path == "/app.js" || 
			   r.URL.Path == "/webtransport" || r.URL.Path == "/favicon.ico" {
				next.ServeHTTP(w, r)
				return
			}

			// Get token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error": "missing authorization header"}`, http.StatusUnauthorized)
				return
			}

			// Check for Bearer scheme
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, `{"error": "invalid authorization format"}`, http.StatusUnauthorized)
				return
			}

			token := parts[1]

			// Validate token
			claims, err := jwtManager.ValidateToken(token)
			if err != nil {
				http.Error(w, `{"error": "invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			// Add user info to context
			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, UsernameKey, claims.Username)
			ctx = context.WithValue(ctx, EmailKey, claims.Email)

			// Pass to next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID extracts user ID from context
func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		return userID
	}
	return ""
}

// GetUsername extracts username from context
func GetUsername(ctx context.Context) string {
	if username, ok := ctx.Value(UsernameKey).(string); ok {
		return username
	}
	return ""
}

// GetEmail extracts email from context
func GetEmail(ctx context.Context) string {
	if email, ok := ctx.Value(EmailKey).(string); ok {
		return email
	}
	return ""
}

// RequireAuth returns a middleware that requires authentication for specific paths
func RequireAuth(jwtManager *auth.JWTManager, excludePaths ...string) func(http.Handler) http.Handler {
	excludeMap := make(map[string]bool)
	for _, path := range excludePaths {
		excludeMap[path] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path is excluded
			if excludeMap[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// Apply JWT auth
			JWTAuth(jwtManager)(next).ServeHTTP(w, r)
		})
	}
}