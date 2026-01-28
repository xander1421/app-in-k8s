package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/alexprut/twitter-clone/pkg/middleware"
	"github.com/alexprut/twitter-clone/pkg/models"
	"github.com/alexprut/twitter-clone/pkg/testutil"
	"github.com/alexprut/twitter-clone/services/user-service/internal/repository"
	"github.com/alexprut/twitter-clone/services/user-service/internal/service"
)

func setupIntegrationService(t *testing.T) *http.ServeMux {
	pool := testutil.TestDB(t)

	repo := repository.NewUserRepository(pool)
	if err := repo.Migrate(context.Background()); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	testutil.CleanupTables(t, pool, "follows", "users")

	svc := service.NewUserService(repo, nil, nil, nil)
	handler := NewUserHandler(svc)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	mux.HandleFunc("GET /health", HealthHandler)

	return mux
}

func TestIntegration_CreateAndGetUser(t *testing.T) {
	mux := setupIntegrationService(t)

	// Create user
	createReq := models.CreateUserRequest{
		Username:    "integrationuser_" + uuid.New().String()[:8],
		Email:       "integration_" + uuid.New().String()[:8] + "@example.com",
		DisplayName: "Integration Test User",
	}
	body, _ := json.Marshal(createReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var createdUser models.User
	if err := json.NewDecoder(w.Body).Decode(&createdUser); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Get user by ID
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users/"+createdUser.ID, nil)
	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var retrievedUser models.User
	if err := json.NewDecoder(w.Body).Decode(&retrievedUser); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if retrievedUser.ID != createdUser.ID {
		t.Errorf("Expected ID %s, got %s", createdUser.ID, retrievedUser.ID)
	}
	if retrievedUser.Username != createReq.Username {
		t.Errorf("Expected username %s, got %s", createReq.Username, retrievedUser.Username)
	}
}

func TestIntegration_GetUserByUsername(t *testing.T) {
	mux := setupIntegrationService(t)

	// Create user
	username := "usernametest_" + uuid.New().String()[:8]
	createReq := models.CreateUserRequest{
		Username: username,
		Email:    "usernametest_" + uuid.New().String()[:8] + "@example.com",
	}
	body, _ := json.Marshal(createReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Failed to create user: %s", w.Body.String())
	}

	// Get user by username
	req = httptest.NewRequest(http.MethodGet, "/api/v1/lookup/username/"+username, nil)
	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var user models.User
	if err := json.NewDecoder(w.Body).Decode(&user); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if user.Username != username {
		t.Errorf("Expected username %s, got %s", username, user.Username)
	}
}

func TestIntegration_FollowFlow(t *testing.T) {
	mux := setupIntegrationService(t)

	// Create two users
	user1Req := models.CreateUserRequest{
		Username: "follower_" + uuid.New().String()[:8],
		Email:    "follower_" + uuid.New().String()[:8] + "@example.com",
	}
	body, _ := json.Marshal(user1Req)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var user1 models.User
	json.NewDecoder(w.Body).Decode(&user1)

	user2Req := models.CreateUserRequest{
		Username: "followee_" + uuid.New().String()[:8],
		Email:    "followee_" + uuid.New().String()[:8] + "@example.com",
	}
	body, _ = json.Marshal(user2Req)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var user2 models.User
	json.NewDecoder(w.Body).Decode(&user2)

	// User1 follows User2 (need to set auth context)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/users/"+user2.ID+"/follow", nil)
	ctx := middleware.SetUserID(req.Context(), user1.ID)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Get User2's followers
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users/"+user2.ID+"/followers", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var followersResp models.FollowersResponse
	if err := json.NewDecoder(w.Body).Decode(&followersResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(followersResp.Users) != 1 {
		t.Errorf("Expected 1 follower, got %d", len(followersResp.Users))
	}
}

func TestIntegration_UpdateUser(t *testing.T) {
	mux := setupIntegrationService(t)

	// Create user
	createReq := models.CreateUserRequest{
		Username:    "updatetest_" + uuid.New().String()[:8],
		Email:       "updatetest_" + uuid.New().String()[:8] + "@example.com",
		DisplayName: "Original Name",
	}
	body, _ := json.Marshal(createReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var user models.User
	json.NewDecoder(w.Body).Decode(&user)

	// Update user (with auth context)
	updateReq := models.UpdateUserRequest{
		DisplayName: "Updated Name",
		Bio:         "Updated bio",
	}
	body, _ = json.Marshal(updateReq)

	req = httptest.NewRequest(http.MethodPut, "/api/v1/users/"+user.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserID(req.Context(), user.ID)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var updatedUser models.User
	if err := json.NewDecoder(w.Body).Decode(&updatedUser); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if updatedUser.DisplayName != "Updated Name" {
		t.Errorf("Expected display name 'Updated Name', got %s", updatedUser.DisplayName)
	}
	if updatedUser.Bio != "Updated bio" {
		t.Errorf("Expected bio 'Updated bio', got %s", updatedUser.Bio)
	}
}

func TestIntegration_UnfollowFlow(t *testing.T) {
	mux := setupIntegrationService(t)

	// Create two users
	user1Req := models.CreateUserRequest{
		Username: "unfollower_" + uuid.New().String()[:8],
		Email:    "unfollower_" + uuid.New().String()[:8] + "@example.com",
	}
	body, _ := json.Marshal(user1Req)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var user1 models.User
	json.NewDecoder(w.Body).Decode(&user1)

	user2Req := models.CreateUserRequest{
		Username: "unfollowee_" + uuid.New().String()[:8],
		Email:    "unfollowee_" + uuid.New().String()[:8] + "@example.com",
	}
	body, _ = json.Marshal(user2Req)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var user2 models.User
	json.NewDecoder(w.Body).Decode(&user2)

	// Follow
	req = httptest.NewRequest(http.MethodPost, "/api/v1/users/"+user2.ID+"/follow", nil)
	ctx := middleware.SetUserID(req.Context(), user1.ID)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// Unfollow
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/users/"+user2.ID+"/follow", nil)
	ctx = middleware.SetUserID(req.Context(), user1.ID)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Check followers count is 0
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users/"+user2.ID+"/followers", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var followersResp models.FollowersResponse
	json.NewDecoder(w.Body).Decode(&followersResp)

	if len(followersResp.Users) != 0 {
		t.Errorf("Expected 0 followers, got %d", len(followersResp.Users))
	}
}
