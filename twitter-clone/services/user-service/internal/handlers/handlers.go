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
	
	// Auth endpoints
	mux.HandleFunc("POST /api/auth/signup", h.HandleSignup)
	mux.HandleFunc("POST /api/auth/login", h.HandleLogin)
	mux.HandleFunc("GET /api/auth/me", h.HandleMe)
	
	// Placeholder endpoints for frontend (until microservices are connected)
	mux.HandleFunc("GET /api/v1/timeline", h.HandleTimelinePlaceholder)
	mux.HandleFunc("POST /api/v1/tweets", h.HandleTweetPlaceholder)
	
	
	// Favicon
	mux.HandleFunc("GET /favicon.ico", h.HandleFavicon)
	
	// Serve Twitter clone UI
	mux.HandleFunc("GET /", h.ServeTwitterUI)
	mux.HandleFunc("GET /app.js", h.ServeAppJS)
	mux.HandleFunc("GET /style.css", h.ServeAppCSS)
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

// ServeTwitterUI serves the main Twitter clone application
func (h *UserHandler) ServeTwitterUI(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Twitter Clone</title>
    <link rel="stylesheet" href="/style.css">
</head>
<body>
    <div id="app">
        <!-- Loading state -->
        <div id="loading" class="loading">
            <div class="loading-spinner"></div>
            <p>Loading Twitter Clone...</p>
        </div>

        <!-- Login form -->
        <div id="login-page" class="auth-page" style="display: none;">
            <div class="auth-container">
                <div class="auth-header">
                    <h1>𝕏</h1>
                    <h2>Sign in to Twitter Clone</h2>
                </div>
                
                <form id="login-form" class="auth-form">
                    <div class="form-group">
                        <input type="text" id="login-username" placeholder="Username or email" required>
                    </div>
                    <div class="form-group">
                        <input type="password" id="login-password" placeholder="Password" required>
                    </div>
                    <button type="submit" class="btn btn-primary">Sign In</button>
                </form>
                
                <p class="auth-footer">
                    Don't have an account? <a href="#" id="show-signup">Sign up</a>
                </p>
            </div>
        </div>

        <!-- Signup form -->
        <div id="signup-page" class="auth-page" style="display: none;">
            <div class="auth-container">
                <div class="auth-header">
                    <h1>𝕏</h1>
                    <h2>Create your account</h2>
                </div>
                
                <form id="signup-form" class="auth-form">
                    <div class="form-group">
                        <input type="text" id="signup-username" placeholder="Username" required>
                    </div>
                    <div class="form-group">
                        <input type="email" id="signup-email" placeholder="Email" required>
                    </div>
                    <div class="form-group">
                        <input type="password" id="signup-password" placeholder="Password" required>
                    </div>
                    <button type="submit" class="btn btn-primary">Sign Up</button>
                </form>
                
                <p class="auth-footer">
                    Already have an account? <a href="#" id="show-login">Sign in</a>
                </p>
            </div>
        </div>

        <!-- Main app -->
        <div id="main-app" style="display: none;">
            <div class="app-container">
                <!-- Sidebar -->
                <div class="sidebar">
                    <div class="sidebar-header">
                        <h1>𝕏</h1>
                    </div>
                    <nav class="sidebar-nav">
                        <a href="#" class="nav-item active" data-section="timeline">
                            🏠 Home
                        </a>
                        <a href="#" class="nav-item" data-section="profile">
                            👤 Profile
                        </a>
                        <a href="#" class="nav-item" data-section="notifications">
                            🔔 Notifications
                            <span id="notification-count" class="notification-badge" style="display: none;">0</span>
                        </a>
                    </nav>
                    <button class="tweet-btn">Tweet</button>
                    <div class="user-menu">
                        <span id="current-username">@username</span>
                        <button id="logout-btn" class="btn-secondary">Logout</button>
                    </div>
                </div>

                <!-- Main content -->
                <div class="main-content">
                    <!-- Tweet composer -->
                    <div id="timeline-section" class="section active">
                        <div class="tweet-composer">
                            <div class="compose-header">
                                <h3>What's happening?</h3>
                                <div class="connection-status" id="connection-status">
                                    <span class="status-dot offline"></span>
                                    <span class="status-text">Offline</span>
                                </div>
                            </div>
                            <form id="tweet-form">
                                <textarea id="tweet-text" placeholder="What's happening?" maxlength="280"></textarea>
                                <div class="compose-footer">
                                    <span class="char-count">280</span>
                                    <button type="submit" class="btn btn-primary" disabled>Tweet</button>
                                </div>
                            </form>
                        </div>

                        <!-- Timeline -->
                        <div class="timeline" id="timeline">
                            <!-- Tweets will be loaded here -->
                        </div>
                    </div>

                    <!-- Profile section -->
                    <div id="profile-section" class="section">
                        <div class="profile-header">
                            <h3>Profile</h3>
                        </div>
                        <div id="profile-content">
                            <!-- Profile content will be loaded here -->
                        </div>
                    </div>

                    <!-- Notifications section -->
                    <div id="notifications-section" class="section">
                        <div class="notifications-header">
                            <h3>Notifications</h3>
                        </div>
                        <div id="notifications-content">
                            <!-- Notifications will be loaded here -->
                        </div>
                    </div>
                </div>

                <!-- Right sidebar -->
                <div class="right-sidebar">
                    <div class="trending">
                        <h3>What's happening</h3>
                        <div id="trending-topics">
                            <!-- Trending topics will be loaded here -->
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>

    <script src="/app.js"></script>
</body>
</html>`
	
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// ServeAppCSS serves the application CSS
func (h *UserHandler) ServeAppCSS(w http.ResponseWriter, r *http.Request) {
	css := `/* Twitter Clone Styles */
* {
    box-sizing: border-box;
    margin: 0;
    padding: 0;
}

body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    background: #000000;
    color: #ffffff;
    line-height: 1.4;
}

/* Loading */
.loading {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100vh;
    color: #1d9bf0;
}

.loading-spinner {
    width: 40px;
    height: 40px;
    border: 3px solid #333;
    border-top: 3px solid #1d9bf0;
    border-radius: 50%;
    animation: spin 1s linear infinite;
}

@keyframes spin {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
}

/* Auth pages */
.auth-page {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
    background: #000000;
}

.auth-container {
    background: #16181c;
    padding: 2rem;
    border-radius: 16px;
    border: 1px solid #2f3336;
    max-width: 400px;
    width: 100%;
}

.auth-header {
    text-align: center;
    margin-bottom: 2rem;
}

.auth-header h1 {
    font-size: 3rem;
    color: #1d9bf0;
    margin-bottom: 0.5rem;
}

.auth-header h2 {
    color: #e7e9ea;
    font-weight: 700;
    font-size: 1.5rem;
}

.auth-form {
    display: flex;
    flex-direction: column;
    gap: 1rem;
}

.form-group input {
    width: 100%;
    padding: 1rem;
    background: #000000;
    border: 1px solid #333639;
    border-radius: 8px;
    color: #ffffff;
    font-size: 1rem;
}

.form-group input:focus {
    outline: none;
    border-color: #1d9bf0;
}

.auth-footer {
    text-align: center;
    margin-top: 1rem;
    color: #71767b;
}

.auth-footer a {
    color: #1d9bf0;
    text-decoration: none;
}

/* Main app layout */
.app-container {
    display: grid;
    grid-template-columns: 250px 1fr 300px;
    height: 100vh;
    max-width: 1200px;
    margin: 0 auto;
}

/* Sidebar */
.sidebar {
    background: #000000;
    border-right: 1px solid #2f3336;
    padding: 1rem;
    display: flex;
    flex-direction: column;
}

.sidebar-header h1 {
    font-size: 2rem;
    color: #1d9bf0;
    margin-bottom: 2rem;
}

.sidebar-nav {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
}

.nav-item {
    display: flex;
    align-items: center;
    gap: 1rem;
    padding: 1rem;
    color: #e7e9ea;
    text-decoration: none;
    border-radius: 25px;
    transition: background 0.2s;
    position: relative;
}

.nav-item:hover {
    background: #0f1419;
}

.nav-item.active {
    font-weight: bold;
}

.notification-badge {
    background: #1d9bf0;
    color: white;
    border-radius: 10px;
    padding: 2px 6px;
    font-size: 0.7rem;
    position: absolute;
    right: 10px;
}

.tweet-btn {
    background: #1d9bf0;
    color: white;
    border: none;
    padding: 1rem;
    border-radius: 25px;
    font-size: 1rem;
    font-weight: bold;
    margin: 2rem 0;
    cursor: pointer;
    transition: background 0.2s;
}

.tweet-btn:hover {
    background: #1a91da;
}

.user-menu {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    padding-top: 1rem;
    border-top: 1px solid #2f3336;
}

/* Main content */
.main-content {
    background: #000000;
    border-right: 1px solid #2f3336;
    overflow-y: auto;
}

.section {
    display: none;
}

.section.active {
    display: block;
}

.tweet-composer {
    border-bottom: 1px solid #2f3336;
    padding: 1rem;
}

.compose-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
}

.compose-header h3 {
    color: #e7e9ea;
    font-size: 1.25rem;
}

.connection-status {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.8rem;
}

.status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #71767b;
}

.status-dot.online {
    background: #00ba7c;
}

.status-dot.offline {
    background: #f91880;
}

#tweet-text {
    width: 100%;
    background: transparent;
    border: none;
    color: #e7e9ea;
    font-size: 1.25rem;
    resize: none;
    min-height: 120px;
}

#tweet-text:focus {
    outline: none;
}

.compose-footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-top: 1rem;
}

.char-count {
    color: #71767b;
    font-size: 0.9rem;
}

/* Timeline */
.timeline {
    display: flex;
    flex-direction: column;
}

.tweet {
    border-bottom: 1px solid #2f3336;
    padding: 1rem;
    transition: background 0.2s;
}

.tweet:hover {
    background: #080808;
}

.tweet-header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-bottom: 0.5rem;
}

.tweet-username {
    font-weight: bold;
    color: #e7e9ea;
}

.tweet-time {
    color: #71767b;
    font-size: 0.9rem;
}

.tweet-content {
    color: #e7e9ea;
    margin-bottom: 0.75rem;
    line-height: 1.5;
}

.tweet-actions {
    display: flex;
    gap: 2rem;
    color: #71767b;
}

.tweet-action {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    cursor: pointer;
    padding: 0.25rem;
    border-radius: 4px;
    transition: color 0.2s;
}

.tweet-action:hover {
    color: #1d9bf0;
}

/* Right sidebar */
.right-sidebar {
    background: #000000;
    padding: 1rem;
}

.trending h3 {
    color: #e7e9ea;
    margin-bottom: 1rem;
    font-size: 1.25rem;
}

/* Buttons */
.btn {
    padding: 0.75rem 1.5rem;
    border: none;
    border-radius: 25px;
    font-size: 1rem;
    font-weight: bold;
    cursor: pointer;
    transition: all 0.2s;
}

.btn-primary {
    background: #1d9bf0;
    color: white;
}

.btn-primary:hover:not(:disabled) {
    background: #1a91da;
}

.btn-primary:disabled {
    background: #1e3a52;
    cursor: not-allowed;
}

.btn-secondary {
    background: #16181c;
    color: #e7e9ea;
    border: 1px solid #2f3336;
}

.btn-secondary:hover {
    background: #1d2126;
}

/* Responsive */
@media (max-width: 768px) {
    .app-container {
        grid-template-columns: 60px 1fr;
    }
    
    .right-sidebar {
        display: none;
    }
    
    .sidebar .nav-item span {
        display: none;
    }
}`
	
	w.Header().Set("Content-Type", "text/css")
	w.Write([]byte(css))
}

// HandleSignup handles user registration
func (h *UserHandler) HandleSignup(w http.ResponseWriter, r *http.Request) {
	var req models.SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// In frontend-only mode, return a mock success response
	response := map[string]interface{}{
		"success": true,
		"message": "Account created successfully! (Frontend mode - no actual account created)",
		"user": map[string]interface{}{
			"id":       "demo-user-123",
			"username": req.Username,
			"email":    req.Email,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleLogin handles user authentication
func (h *UserHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// In frontend-only mode, return a mock success response
	response := map[string]interface{}{
		"success":       true,
		"access_token":  "demo-token-123",
		"refresh_token": "demo-refresh-456",
		"user": map[string]interface{}{
			"id":       "demo-user-123",
			"username": req.Username,
			"email":    req.Username, // Use username as email for demo
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleMe returns current user info
func (h *UserHandler) HandleMe(w http.ResponseWriter, r *http.Request) {
	// In frontend-only mode, return mock user data
	response := map[string]interface{}{
		"id":       "demo-user-123",
		"username": "demouser",
		"email":    "demo@example.com",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ServeAppJS serves the main application JavaScript
func (h *UserHandler) ServeAppJS(w http.ResponseWriter, r *http.Request) {
	js := `// Twitter Clone Application with WebTransport
class TwitterApp {
    constructor() {
        this.transport = null;
        this.currentUser = null;
        this.tweets = [];
        this.notifications = [];
        
        this.init();
    }

    async init() {
        // Check for existing session
        const token = localStorage.getItem('authToken');
        if (token) {
            try {
                await this.validateSession(token);
                this.showMainApp();
                await this.connectWebTransport();
            } catch (error) {
                this.showLogin();
            }
        } else {
            this.showLogin();
        }

        this.setupEventListeners();
    }

    setupEventListeners() {
        // Auth form handlers
        document.getElementById('login-form').addEventListener('submit', (e) => this.handleLogin(e));
        document.getElementById('signup-form').addEventListener('submit', (e) => this.handleSignup(e));
        document.getElementById('show-signup').addEventListener('click', (e) => {
            e.preventDefault();
            this.showSignup();
        });
        document.getElementById('show-login').addEventListener('click', (e) => {
            e.preventDefault();
            this.showLogin();
        });

        // Main app handlers
        document.getElementById('tweet-form').addEventListener('submit', (e) => this.handleTweet(e));
        document.getElementById('logout-btn').addEventListener('click', () => this.logout());
        
        // Navigation
        document.querySelectorAll('.nav-item').forEach(item => {
            item.addEventListener('click', (e) => {
                e.preventDefault();
                const section = item.dataset.section;
                this.showSection(section);
            });
        });

        // Tweet composer
        const tweetText = document.getElementById('tweet-text');
        tweetText.addEventListener('input', () => this.updateCharCount());
    }

    showLogin() {
        document.getElementById('loading').style.display = 'none';
        document.getElementById('signup-page').style.display = 'none';
        document.getElementById('main-app').style.display = 'none';
        document.getElementById('login-page').style.display = 'flex';
    }

    showSignup() {
        document.getElementById('loading').style.display = 'none';
        document.getElementById('login-page').style.display = 'none';
        document.getElementById('main-app').style.display = 'none';
        document.getElementById('signup-page').style.display = 'flex';
    }

    showMainApp() {
        document.getElementById('loading').style.display = 'none';
        document.getElementById('login-page').style.display = 'none';
        document.getElementById('signup-page').style.display = 'none';
        document.getElementById('main-app').style.display = 'block';
        
        this.loadTimeline();
    }

    async handleLogin(e) {
        e.preventDefault();
        const username = document.getElementById('login-username').value;
        const password = document.getElementById('login-password').value;

        try {
            const response = await fetch('/api/auth/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ username, password })
            });

            if (response.ok) {
                const data = await response.json();
                localStorage.setItem('authToken', data.access_token);
                this.currentUser = data.user;
                document.getElementById('current-username').textContent = '@' + data.user.username;
                this.showMainApp();
                await this.connectWebTransport();
            } else {
                alert('Login failed');
            }
        } catch (error) {
            alert('Login error: ' + error.message);
        }
    }

    async handleSignup(e) {
        e.preventDefault();
        const username = document.getElementById('signup-username').value;
        const email = document.getElementById('signup-email').value;
        const password = document.getElementById('signup-password').value;

        try {
            const response = await fetch('/api/auth/signup', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ username, email, password })
            });

            if (response.ok) {
                alert('Account created! Please login.');
                this.showLogin();
            } else {
                const error = await response.json();
                alert('Signup failed: ' + error.message);
            }
        } catch (error) {
            alert('Signup error: ' + error.message);
        }
    }

    async handleTweet(e) {
        e.preventDefault();
        const content = document.getElementById('tweet-text').value.trim();
        
        if (!content) return;

        try {
            const response = await fetch('/api/v1/tweets', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': 'Bearer ' + localStorage.getItem('authToken')
                },
                body: JSON.stringify({ content })
            });

            if (response.ok) {
                document.getElementById('tweet-text').value = '';
                this.updateCharCount();
                this.loadTimeline(); // Refresh timeline
                
                // Send real-time update via WebTransport
                if (this.transport) {
                    this.sendRealtimeUpdate('new_tweet', { content });
                }
            }
        } catch (error) {
            console.error('Tweet failed:', error);
        }
    }

    async connectWebTransport() {
        try {
            // Only attempt WebTransport if running on HTTPS
            if (window.location.protocol !== 'https:') {
                console.log('WebTransport requires HTTPS - running in HTTP-only mode');
                this.updateConnectionStatus(false);
                return;
            }

            if (!window.WebTransport) {
                console.log('WebTransport not supported in this browser');
                this.updateConnectionStatus(false);
                return;
            }

            const url = 'https://' + window.location.host + '/webtransport';
            this.transport = new WebTransport(url);
            
            await this.transport.ready;
            
            this.updateConnectionStatus(true);
            this.handleRealtimeUpdates();
            
            console.log('WebTransport connected');
        } catch (error) {
            console.error('WebTransport connection failed:', error);
            this.updateConnectionStatus(false);
        }
    }

    async handleRealtimeUpdates() {
        if (!this.transport) return;

        try {
            const reader = this.transport.datagrams.readable.getReader();
            
            while (true) {
                const { value, done } = await reader.read();
                if (done) break;

                const decoder = new TextDecoder();
                const message = JSON.parse(decoder.decode(value));
                
                switch (message.type) {
                    case 'new_tweet':
                        this.loadTimeline();
                        break;
                    case 'notification':
                        this.addNotification(message.data);
                        break;
                    case 'like':
                    case 'retweet':
                        this.updateTweetStats(message.tweetId, message.type);
                        break;
                }
            }
        } catch (error) {
            console.error('Error handling realtime updates:', error);
        }
    }

    async sendRealtimeUpdate(type, data) {
        if (!this.transport) return;

        try {
            const writer = this.transport.datagrams.writable.getWriter();
            const message = JSON.stringify({ type, data, userId: this.currentUser.id });
            const encoder = new TextEncoder();
            await writer.write(encoder.encode(message));
        } catch (error) {
            console.error('Error sending realtime update:', error);
        }
    }

    updateConnectionStatus(connected) {
        const statusDot = document.querySelector('.status-dot');
        const statusText = document.querySelector('.status-text');
        
        if (connected) {
            statusDot.classList.remove('offline');
            statusDot.classList.add('online');
            statusText.textContent = 'Online';
        } else {
            statusDot.classList.remove('online');
            statusDot.classList.add('offline');
            statusText.textContent = 'Offline';
        }
    }

    async loadTimeline() {
        try {
            const response = await fetch('/api/v1/timeline', {
                headers: {
                    'Authorization': 'Bearer ' + localStorage.getItem('authToken')
                }
            });
            
            if (response.ok) {
                const tweets = await response.json();
                this.displayTweets(tweets);
            } else if (response.status === 404) {
                // Timeline service not available, show placeholder
                this.displayTweets([{
                    id: 'placeholder',
                    user: { username: 'system' },
                    content: 'Welcome to Twitter Clone! Timeline service is starting up...',
                    created_at: new Date().toISOString(),
                    likes: 0,
                    retweets: 0
                }]);
            }
        } catch (error) {
            console.error('Failed to load timeline:', error);
            // Show fallback content
            this.displayTweets([{
                id: 'error',
                user: { username: 'system' },
                content: 'Timeline temporarily unavailable. Please check back later.',
                created_at: new Date().toISOString(),
                likes: 0,
                retweets: 0
            }]);
        }
    }

    displayTweets(tweets) {
        const timeline = document.getElementById('timeline');
        timeline.innerHTML = '';
        
        tweets.forEach(tweet => {
            const tweetEl = this.createTweetElement(tweet);
            timeline.appendChild(tweetEl);
        });
    }

    createTweetElement(tweet) {
        const tweetEl = document.createElement('div');
        tweetEl.className = 'tweet';
        tweetEl.innerHTML = ` + "`" + `
            <div class="tweet-header">
                <span class="tweet-username">${tweet.user.username}</span>
                <span class="tweet-time">${this.formatTime(tweet.created_at)}</span>
            </div>
            <div class="tweet-content">${tweet.content}</div>
            <div class="tweet-actions">
                <div class="tweet-action" onclick="app.likeTweet('${tweet.id}')">
                    ❤️ <span>${tweet.likes_count || 0}</span>
                </div>
                <div class="tweet-action" onclick="app.retweetTweet('${tweet.id}')">
                    🔁 <span>${tweet.retweets_count || 0}</span>
                </div>
                <div class="tweet-action">
                    💬 <span>${tweet.replies_count || 0}</span>
                </div>
            </div>
        ` + "`" + `;
        return tweetEl;
    }

    async likeTweet(tweetId) {
        try {
            await fetch('/api/v1/tweets/' + tweetId + '/like', {
                method: 'POST',
                headers: {
                    'Authorization': 'Bearer ' + localStorage.getItem('authToken')
                }
            });
            
            if (this.transport) {
                this.sendRealtimeUpdate('like', { tweetId });
            }
        } catch (error) {
            console.error('Like failed:', error);
        }
    }

    updateCharCount() {
        const tweetText = document.getElementById('tweet-text');
        const charCount = document.querySelector('.char-count');
        const submitBtn = document.querySelector('#tweet-form button[type="submit"]');
        
        const remaining = 280 - tweetText.value.length;
        charCount.textContent = remaining;
        
        if (remaining < 0) {
            charCount.style.color = '#f91880';
            submitBtn.disabled = true;
        } else if (remaining < 20) {
            charCount.style.color = '#ffd400';
            submitBtn.disabled = false;
        } else {
            charCount.style.color = '#71767b';
            submitBtn.disabled = tweetText.value.trim().length === 0;
        }
    }

    showSection(section) {
        document.querySelectorAll('.section').forEach(s => s.classList.remove('active'));
        document.querySelectorAll('.nav-item').forEach(n => n.classList.remove('active'));
        
        document.getElementById(section + '-section').classList.add('active');
        document.querySelector('[data-section="' + section + '"]').classList.add('active');
        
        if (section === 'timeline') {
            this.loadTimeline();
        } else if (section === 'notifications') {
            this.loadNotifications();
        }
    }

    addNotification(notification) {
        this.notifications.unshift(notification);
        this.updateNotificationCount();
    }

    updateNotificationCount() {
        const badge = document.getElementById('notification-count');
        const unreadCount = this.notifications.filter(n => !n.read).length;
        
        if (unreadCount > 0) {
            badge.textContent = unreadCount;
            badge.style.display = 'block';
        } else {
            badge.style.display = 'none';
        }
    }

    formatTime(timestamp) {
        const date = new Date(timestamp);
        const now = new Date();
        const diff = now - date;
        
        if (diff < 60000) return 'now';
        if (diff < 3600000) return Math.floor(diff / 60000) + 'm';
        if (diff < 86400000) return Math.floor(diff / 3600000) + 'h';
        return Math.floor(diff / 86400000) + 'd';
    }

    async validateSession(token) {
        const response = await fetch('/api/auth/me', {
            headers: { 'Authorization': 'Bearer ' + token }
        });
        
        if (!response.ok) throw new Error('Invalid session');
        
        const user = await response.json();
        this.currentUser = user;
        document.getElementById('current-username').textContent = '@' + user.username;
        return user;
    }

    logout() {
        localStorage.removeItem('authToken');
        if (this.transport) {
            this.transport.close();
            this.transport = null;
        }
        this.showLogin();
    }
}

// Initialize app
let app;
document.addEventListener('DOMContentLoaded', () => {
    app = new TwitterApp();
});`
	
	w.Header().Set("Content-Type", "application/javascript")
	w.Write([]byte(js))
}

func (h *UserHandler) HandleFavicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/x-icon")
	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) HandleTimelinePlaceholder(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	// Return placeholder timeline data
	tweets := []map[string]interface{}{
		{
			"id":         "1",
			"content":    "Welcome to Twitter Clone! This is a placeholder tweet.",
			"user":       map[string]string{"username": "system", "id": "system"},
			"created_at": "2026-01-28T19:45:00Z",
			"likes":      0,
			"retweets":   0,
		},
		{
			"id":         "2", 
			"content":    "Timeline service will be connected soon for real tweets.",
			"user":       map[string]string{"username": "admin", "id": "admin"},
			"created_at": "2026-01-28T19:44:00Z",
			"likes":      5,
			"retweets":   2,
		},
	}

	middleware.WriteJSON(w, http.StatusOK, tweets)
}

func (h *UserHandler) HandleTweetPlaceholder(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		middleware.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Return placeholder tweet response
	tweet := map[string]interface{}{
		"id":         "placeholder-" + userID,
		"content":    req.Content,
		"user":       map[string]string{"username": "user", "id": userID},
		"created_at": "2026-01-28T19:46:00Z",
		"likes":      0,
		"retweets":   0,
	}

	middleware.WriteJSON(w, http.StatusCreated, tweet)
}
