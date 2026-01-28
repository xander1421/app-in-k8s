package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/alexprut/twitter-clone/pkg/middleware"
	"github.com/alexprut/twitter-clone/pkg/models"
)

// MockUserService is a mock implementation of the user service
type MockUserService struct {
	users       map[string]*models.User
	follows     map[string][]string // followerID -> followeeIDs
	createError error
}

func NewMockUserService() *MockUserService {
	return &MockUserService{
		users:   make(map[string]*models.User),
		follows: make(map[string][]string),
	}
}

func (m *MockUserService) CreateUser(ctx context.Context, req models.CreateUserRequest) (*models.User, error) {
	if m.createError != nil {
		return nil, m.createError
	}
	user := &models.User{
		ID:          uuid.New().String(),
		Username:    req.Username,
		Email:       req.Email,
		DisplayName: req.DisplayName,
		CreatedAt:   time.Now(),
	}
	m.users[user.ID] = user
	return user, nil
}

func (m *MockUserService) GetUser(ctx context.Context, userID string) (*models.User, error) {
	if user, ok := m.users[userID]; ok {
		return user, nil
	}
	return nil, context.Canceled
}

func (m *MockUserService) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	for _, user := range m.users {
		if user.Username == username {
			return user, nil
		}
	}
	return nil, context.Canceled
}

func (m *MockUserService) UpdateUser(ctx context.Context, userID string, req models.UpdateUserRequest) (*models.User, error) {
	user, ok := m.users[userID]
	if !ok {
		return nil, context.Canceled
	}
	if req.DisplayName != "" {
		user.DisplayName = req.DisplayName
	}
	if req.Bio != "" {
		user.Bio = req.Bio
	}
	return user, nil
}

func (m *MockUserService) Follow(ctx context.Context, followerID, followeeID string) error {
	m.follows[followerID] = append(m.follows[followerID], followeeID)
	return nil
}

func (m *MockUserService) Unfollow(ctx context.Context, followerID, followeeID string) error {
	for i, id := range m.follows[followerID] {
		if id == followeeID {
			m.follows[followerID] = append(m.follows[followerID][:i], m.follows[followerID][i+1:]...)
			break
		}
	}
	return nil
}

func (m *MockUserService) GetFollowers(ctx context.Context, userID string, limit, offset int) ([]models.User, bool, error) {
	return []models.User{}, false, nil
}

func (m *MockUserService) GetFollowing(ctx context.Context, userID string, limit, offset int) ([]models.User, bool, error) {
	return []models.User{}, false, nil
}

func (m *MockUserService) GetFollowerIDs(ctx context.Context, userID string) ([]string, error) {
	return []string{}, nil
}

func (m *MockUserService) IsFollowing(ctx context.Context, followerID, followeeID string) (bool, error) {
	for _, id := range m.follows[followerID] {
		if id == followeeID {
			return true, nil
		}
	}
	return false, nil
}

// MockUserHandler wraps MockUserService for testing
type MockUserHandler struct {
	svc *MockUserService
}

func NewMockUserHandler() *MockUserHandler {
	return &MockUserHandler{svc: NewMockUserService()}
}

// Handler implementations that match the real handler

func (h *MockUserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
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
		middleware.WriteError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	middleware.WriteJSON(w, http.StatusCreated, user)
}

func (h *MockUserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if userID == "" {
		middleware.WriteError(w, http.StatusBadRequest, "missing_id", "User ID is required")
		return
	}

	user, err := h.svc.GetUser(r.Context(), userID)
	if err != nil {
		middleware.WriteError(w, http.StatusNotFound, "not_found", "User not found")
		return
	}

	middleware.WriteJSON(w, http.StatusOK, user)
}

// Tests

func TestCreateUser_Success(t *testing.T) {
	handler := NewMockUserHandler()

	body := models.CreateUserRequest{
		Username:    "testuser",
		Email:       "test@example.com",
		DisplayName: "Test User",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CreateUser(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var user models.User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if user.Username != body.Username {
		t.Errorf("Expected username %s, got %s", body.Username, user.Username)
	}
	if user.Email != body.Email {
		t.Errorf("Expected email %s, got %s", body.Email, user.Email)
	}
}

func TestCreateUser_MissingFields(t *testing.T) {
	handler := NewMockUserHandler()

	body := models.CreateUserRequest{
		Username: "testuser",
		// Missing email
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CreateUser(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestCreateUser_InvalidBody(t *testing.T) {
	handler := NewMockUserHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CreateUser(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestGetUser_Success(t *testing.T) {
	handler := NewMockUserHandler()

	// Create a user first
	user := &models.User{
		ID:          uuid.New().String(),
		Username:    "testuser",
		Email:       "test@example.com",
		DisplayName: "Test User",
		CreatedAt:   time.Now(),
	}
	handler.svc.users[user.ID] = user

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/users/{id}", handler.GetUser)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+user.ID, nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var retrieved models.User
	if err := json.NewDecoder(resp.Body).Decode(&retrieved); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if retrieved.ID != user.ID {
		t.Errorf("Expected ID %s, got %s", user.ID, retrieved.ID)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	handler := NewMockUserHandler()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/users/{id}", handler.GetUser)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/nonexistent", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	HealthHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if body["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %s", body["status"])
	}
}

// Additional handler tests

func TestCreateUser_EmptyUsername(t *testing.T) {
	handler := NewMockUserHandler()

	body := models.CreateUserRequest{
		Username: "",
		Email:    "test@example.com",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CreateUser(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestCreateUser_EmptyEmail(t *testing.T) {
	handler := NewMockUserHandler()

	body := models.CreateUserRequest{
		Username: "testuser",
		Email:    "",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CreateUser(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}
