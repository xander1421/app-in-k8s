package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/alexprut/twitter-clone/pkg/middleware"
	"github.com/alexprut/twitter-clone/pkg/models"
	"github.com/alexprut/twitter-clone/pkg/testutil"
	"github.com/alexprut/twitter-clone/services/user-service/internal/repository"
	"github.com/alexprut/twitter-clone/services/user-service/internal/service"
)

// TestE2E_FullUserJourney tests complete user registration to update flow
func TestE2E_FullUserJourney(t *testing.T) {
	mux := setupIntegrationService(t)

	// Step 1: Register user
	createReq := models.CreateUserRequest{
		Username:    "e2euser_" + uuid.New().String()[:8],
		Email:       "e2e_" + uuid.New().String()[:8] + "@test.com",
		DisplayName: "E2E Test User",
	}
	body, _ := json.Marshal(createReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create user failed: %d - %s", w.Code, w.Body.String())
	}

	var user models.User
	json.NewDecoder(w.Body).Decode(&user)

	t.Logf("Step 1: Created user %s (%s)", user.Username, user.ID)

	// Step 2: Get user profile
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users/"+user.ID, nil)
	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("get user failed: %d - %s", w.Code, w.Body.String())
	}

	var fetchedUser models.User
	json.NewDecoder(w.Body).Decode(&fetchedUser)

	if fetchedUser.Username != createReq.Username {
		t.Errorf("username mismatch: got %s, want %s", fetchedUser.Username, createReq.Username)
	}

	t.Logf("Step 2: Fetched user profile")

	// Step 3: Get user by username
	req = httptest.NewRequest(http.MethodGet, "/api/v1/lookup/username/"+user.Username, nil)
	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("get by username failed: %d - %s", w.Code, w.Body.String())
	}

	t.Logf("Step 3: Fetched user by username")

	// Step 4: Update user profile
	updateReq := models.UpdateUserRequest{
		DisplayName: "Updated E2E User",
		Bio:         "This is my updated bio",
	}
	body, _ = json.Marshal(updateReq)

	req = httptest.NewRequest(http.MethodPut, "/api/v1/users/"+user.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserID(req.Context(), user.ID)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("update user failed: %d - %s", w.Code, w.Body.String())
	}

	var updatedUser models.User
	json.NewDecoder(w.Body).Decode(&updatedUser)

	if updatedUser.DisplayName != "Updated E2E User" {
		t.Errorf("display name not updated: got %s", updatedUser.DisplayName)
	}
	if updatedUser.Bio != "This is my updated bio" {
		t.Errorf("bio not updated: got %s", updatedUser.Bio)
	}

	t.Logf("Step 4: Updated user profile")
	t.Log("E2E Full User Journey: PASSED")
}

// TestE2E_SocialGraph tests complete follow/unfollow social graph operations
func TestE2E_SocialGraph(t *testing.T) {
	mux := setupIntegrationService(t)

	// Create three users
	alice := createE2EUser(t, mux, "alice")
	bob := createE2EUser(t, mux, "bob")
	charlie := createE2EUser(t, mux, "charlie")

	t.Logf("Created users: %s, %s, %s", alice.Username, bob.Username, charlie.Username)

	// Alice follows Bob and Charlie
	followUser(t, mux, alice.ID, bob.ID)
	followUser(t, mux, alice.ID, charlie.ID)

	t.Log("Alice followed Bob and Charlie")

	// Bob follows Alice
	followUser(t, mux, bob.ID, alice.ID)

	t.Log("Bob followed Alice")

	// Verify Alice's following count
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+alice.ID+"/following", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var followingResp models.FollowersResponse
	json.NewDecoder(w.Body).Decode(&followingResp)

	if len(followingResp.Users) != 2 {
		t.Errorf("Alice should be following 2 users, got %d", len(followingResp.Users))
	}

	// Verify Bob's followers
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users/"+bob.ID+"/followers", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var followersResp models.FollowersResponse
	json.NewDecoder(w.Body).Decode(&followersResp)

	if len(followersResp.Users) != 1 {
		t.Errorf("Bob should have 1 follower, got %d", len(followersResp.Users))
	}

	// Verify Alice's followers
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users/"+alice.ID+"/followers", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	json.NewDecoder(w.Body).Decode(&followersResp)

	if len(followersResp.Users) != 1 {
		t.Errorf("Alice should have 1 follower, got %d", len(followersResp.Users))
	}

	// Alice unfollows Bob
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/users/"+bob.ID+"/follow", nil)
	ctx := middleware.SetUserID(req.Context(), alice.ID)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unfollow failed: %d", w.Code)
	}

	t.Log("Alice unfollowed Bob")

	// Verify Alice's following is now 1
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users/"+alice.ID+"/following", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	json.NewDecoder(w.Body).Decode(&followingResp)

	if len(followingResp.Users) != 1 {
		t.Errorf("Alice should be following 1 user after unfollow, got %d", len(followingResp.Users))
	}

	// Verify Bob has 0 followers now
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users/"+bob.ID+"/followers", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	json.NewDecoder(w.Body).Decode(&followersResp)

	if len(followersResp.Users) != 0 {
		t.Errorf("Bob should have 0 followers after unfollow, got %d", len(followersResp.Users))
	}

	t.Log("E2E Social Graph: PASSED")
}

// TestE2E_FollowerIDs tests getting follower IDs for fanout
func TestE2E_FollowerIDs(t *testing.T) {
	mux := setupIntegrationService(t)

	// Create a user with followers
	celeb := createE2EUser(t, mux, "celebrity")

	// Create 5 followers
	followers := make([]models.User, 5)
	for i := 0; i < 5; i++ {
		followers[i] = createE2EUser(t, mux, fmt.Sprintf("fan%d", i))
		followUser(t, mux, followers[i].ID, celeb.ID)
	}

	t.Logf("Created celebrity %s with 5 followers", celeb.Username)

	// Get follower IDs
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+celeb.ID+"/follower-ids", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("get follower IDs failed: %d - %s", w.Code, w.Body.String())
	}

	var ids []string
	json.NewDecoder(w.Body).Decode(&ids)

	if len(ids) != 5 {
		t.Errorf("expected 5 follower IDs, got %d", len(ids))
	}

	// Verify all follower IDs are present
	idMap := make(map[string]bool)
	for _, id := range ids {
		idMap[id] = true
	}

	for _, follower := range followers {
		if !idMap[follower.ID] {
			t.Errorf("follower %s ID not in response", follower.Username)
		}
	}

	t.Log("E2E Follower IDs: PASSED")
}

// TestE2E_Pagination tests pagination for followers/following
func TestE2E_Pagination(t *testing.T) {
	pool := testutil.TestDB(t)

	repo := repository.NewUserRepository(pool)
	if err := repo.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	testutil.CleanupTables(t, pool, "follows", "users")

	svc := service.NewUserService(repo, nil, nil, nil)
	handler := NewUserHandler(svc)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Create a user with many followers
	target := createE2EUser(t, mux, "popular")

	// Create 25 followers
	for i := 0; i < 25; i++ {
		follower := createE2EUser(t, mux, fmt.Sprintf("follower%02d", i))
		followUser(t, mux, follower.ID, target.ID)
		time.Sleep(time.Millisecond) // Ensure different timestamps
	}

	t.Logf("Created user %s with 25 followers", target.Username)

	// Get first page (default limit 20)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+target.ID+"/followers", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var page1 models.FollowersResponse
	json.NewDecoder(w.Body).Decode(&page1)

	if len(page1.Users) != 20 {
		t.Errorf("expected 20 users in page 1, got %d", len(page1.Users))
	}
	if !page1.HasMore {
		t.Error("expected HasMore=true for page 1")
	}

	// Get second page
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users/"+target.ID+"/followers?offset=20", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var page2 models.FollowersResponse
	json.NewDecoder(w.Body).Decode(&page2)

	if len(page2.Users) != 5 {
		t.Errorf("expected 5 users in page 2, got %d", len(page2.Users))
	}
	if page2.HasMore {
		t.Error("expected HasMore=false for last page")
	}

	// Verify no overlap between pages
	page1IDs := make(map[string]bool)
	for _, u := range page1.Users {
		page1IDs[u.ID] = true
	}

	for _, u := range page2.Users {
		if page1IDs[u.ID] {
			t.Errorf("user %s appears in both pages", u.ID)
		}
	}

	t.Log("E2E Pagination: PASSED")
}

// TestE2E_AuthorizationChecks tests that auth is properly enforced
func TestE2E_AuthorizationChecks(t *testing.T) {
	mux := setupIntegrationService(t)

	alice := createE2EUser(t, mux, "alice")
	bob := createE2EUser(t, mux, "bob")

	// Try to update Alice's profile without auth
	updateReq := models.UpdateUserRequest{
		DisplayName: "Hacked Name",
	}
	body, _ := json.Marshal(updateReq)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/"+alice.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated update, got %d", w.Code)
	}

	// Try to update Alice's profile as Bob
	body, _ = json.Marshal(updateReq)
	req = httptest.NewRequest(http.MethodPut, "/api/v1/users/"+alice.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserID(req.Context(), bob.ID)
	req = req.WithContext(ctx)
	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for unauthorized update, got %d", w.Code)
	}

	// Try to follow without auth
	req = httptest.NewRequest(http.MethodPost, "/api/v1/users/"+bob.ID+"/follow", nil)
	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated follow, got %d", w.Code)
	}

	t.Log("E2E Authorization Checks: PASSED")
}

// Helper functions

func createE2EUser(t *testing.T, mux *http.ServeMux, prefix string) models.User {
	t.Helper()

	createReq := models.CreateUserRequest{
		Username:    fmt.Sprintf("%s_%s", prefix, uuid.New().String()[:8]),
		Email:       fmt.Sprintf("%s_%s@test.com", prefix, uuid.New().String()[:8]),
		DisplayName: prefix + " User",
	}
	body, _ := json.Marshal(createReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create user %s: %d - %s", prefix, w.Code, w.Body.String())
	}

	var user models.User
	json.NewDecoder(w.Body).Decode(&user)
	return user
}

func followUser(t *testing.T, mux *http.ServeMux, followerID, followeeID string) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/"+followeeID+"/follow", nil)
	ctx := middleware.SetUserID(req.Context(), followerID)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("follow failed: %d - %s", w.Code, w.Body.String())
	}
}
