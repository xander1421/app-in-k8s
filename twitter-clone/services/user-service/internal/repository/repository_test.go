package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/alexprut/twitter-clone/pkg/models"
	"github.com/alexprut/twitter-clone/pkg/testutil"
)

func TestUserRepository_Create(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := NewUserRepository(pool)

	ctx := context.Background()
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	testutil.CleanupTables(t, pool, "follows", "users")

	user := &models.User{
		ID:          uuid.New().String(),
		Username:    "testuser_" + uuid.New().String()[:8],
		Email:       "test_" + uuid.New().String()[:8] + "@example.com",
		DisplayName: "Test User",
		Bio:         "Test bio",
		CreatedAt:   time.Now(),
	}

	err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Verify user was created
	retrieved, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}

	if retrieved.Username != user.Username {
		t.Errorf("Expected username %s, got %s", user.Username, retrieved.Username)
	}
	if retrieved.Email != user.Email {
		t.Errorf("Expected email %s, got %s", user.Email, retrieved.Email)
	}
}

func TestUserRepository_GetByUsername(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := NewUserRepository(pool)

	ctx := context.Background()
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	testutil.CleanupTables(t, pool, "follows", "users")

	username := "uniqueuser_" + uuid.New().String()[:8]
	user := &models.User{
		ID:          uuid.New().String(),
		Username:    username,
		Email:       "unique_" + uuid.New().String()[:8] + "@example.com",
		DisplayName: "Unique User",
		CreatedAt:   time.Now(),
	}

	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	retrieved, err := repo.GetByUsername(ctx, username)
	if err != nil {
		t.Fatalf("Failed to get user by username: %v", err)
	}

	if retrieved.ID != user.ID {
		t.Errorf("Expected ID %s, got %s", user.ID, retrieved.ID)
	}
}

func TestUserRepository_Update(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := NewUserRepository(pool)

	ctx := context.Background()
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	testutil.CleanupTables(t, pool, "follows", "users")

	user := &models.User{
		ID:          uuid.New().String(),
		Username:    "updateuser_" + uuid.New().String()[:8],
		Email:       "update_" + uuid.New().String()[:8] + "@example.com",
		DisplayName: "Original Name",
		Bio:         "Original bio",
		CreatedAt:   time.Now(),
	}

	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Update user
	user.DisplayName = "Updated Name"
	user.Bio = "Updated bio"

	if err := repo.Update(ctx, user); err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	// Verify update
	retrieved, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}

	if retrieved.DisplayName != "Updated Name" {
		t.Errorf("Expected DisplayName 'Updated Name', got %s", retrieved.DisplayName)
	}
	if retrieved.Bio != "Updated bio" {
		t.Errorf("Expected Bio 'Updated bio', got %s", retrieved.Bio)
	}
}

func TestUserRepository_Follow(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := NewUserRepository(pool)

	ctx := context.Background()
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	testutil.CleanupTables(t, pool, "follows", "users")

	// Create two users
	user1 := &models.User{
		ID:          uuid.New().String(),
		Username:    "follower_" + uuid.New().String()[:8],
		Email:       "follower_" + uuid.New().String()[:8] + "@example.com",
		DisplayName: "Follower",
		CreatedAt:   time.Now(),
	}
	user2 := &models.User{
		ID:          uuid.New().String(),
		Username:    "followee_" + uuid.New().String()[:8],
		Email:       "followee_" + uuid.New().String()[:8] + "@example.com",
		DisplayName: "Followee",
		CreatedAt:   time.Now(),
	}

	if err := repo.Create(ctx, user1); err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}
	if err := repo.Create(ctx, user2); err != nil {
		t.Fatalf("Failed to create user2: %v", err)
	}

	// Follow
	if err := repo.Follow(ctx, user1.ID, user2.ID); err != nil {
		t.Fatalf("Failed to follow: %v", err)
	}

	// Check if following
	isFollowing, err := repo.IsFollowing(ctx, user1.ID, user2.ID)
	if err != nil {
		t.Fatalf("Failed to check following: %v", err)
	}
	if !isFollowing {
		t.Error("Expected user1 to be following user2")
	}

	// Check follower count
	followee, err := repo.GetByID(ctx, user2.ID)
	if err != nil {
		t.Fatalf("Failed to get followee: %v", err)
	}
	if followee.FollowerCount != 1 {
		t.Errorf("Expected follower count 1, got %d", followee.FollowerCount)
	}

	// Check following count
	follower, err := repo.GetByID(ctx, user1.ID)
	if err != nil {
		t.Fatalf("Failed to get follower: %v", err)
	}
	if follower.FollowingCount != 1 {
		t.Errorf("Expected following count 1, got %d", follower.FollowingCount)
	}
}

func TestUserRepository_Unfollow(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := NewUserRepository(pool)

	ctx := context.Background()
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	testutil.CleanupTables(t, pool, "follows", "users")

	// Create two users
	user1 := &models.User{
		ID:          uuid.New().String(),
		Username:    "unfollower_" + uuid.New().String()[:8],
		Email:       "unfollower_" + uuid.New().String()[:8] + "@example.com",
		DisplayName: "Unfollower",
		CreatedAt:   time.Now(),
	}
	user2 := &models.User{
		ID:          uuid.New().String(),
		Username:    "unfollowee_" + uuid.New().String()[:8],
		Email:       "unfollowee_" + uuid.New().String()[:8] + "@example.com",
		DisplayName: "Unfollowee",
		CreatedAt:   time.Now(),
	}

	if err := repo.Create(ctx, user1); err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}
	if err := repo.Create(ctx, user2); err != nil {
		t.Fatalf("Failed to create user2: %v", err)
	}

	// Follow then unfollow
	if err := repo.Follow(ctx, user1.ID, user2.ID); err != nil {
		t.Fatalf("Failed to follow: %v", err)
	}
	if err := repo.Unfollow(ctx, user1.ID, user2.ID); err != nil {
		t.Fatalf("Failed to unfollow: %v", err)
	}

	// Check if still following
	isFollowing, err := repo.IsFollowing(ctx, user1.ID, user2.ID)
	if err != nil {
		t.Fatalf("Failed to check following: %v", err)
	}
	if isFollowing {
		t.Error("Expected user1 to not be following user2 after unfollow")
	}
}

func TestUserRepository_GetFollowers(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := NewUserRepository(pool)

	ctx := context.Background()
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	testutil.CleanupTables(t, pool, "follows", "users")

	// Create a user with followers
	mainUser := &models.User{
		ID:          uuid.New().String(),
		Username:    "popular_" + uuid.New().String()[:8],
		Email:       "popular_" + uuid.New().String()[:8] + "@example.com",
		DisplayName: "Popular User",
		CreatedAt:   time.Now(),
	}
	if err := repo.Create(ctx, mainUser); err != nil {
		t.Fatalf("Failed to create main user: %v", err)
	}

	// Create followers
	for i := 0; i < 3; i++ {
		follower := &models.User{
			ID:          uuid.New().String(),
			Username:    "follower_" + uuid.New().String()[:8],
			Email:       "follower_" + uuid.New().String()[:8] + "@example.com",
			DisplayName: "Follower",
			CreatedAt:   time.Now(),
		}
		if err := repo.Create(ctx, follower); err != nil {
			t.Fatalf("Failed to create follower: %v", err)
		}
		if err := repo.Follow(ctx, follower.ID, mainUser.ID); err != nil {
			t.Fatalf("Failed to follow: %v", err)
		}
	}

	// Get followers
	followers, err := repo.GetFollowers(ctx, mainUser.ID, 10, 0)
	if err != nil {
		t.Fatalf("Failed to get followers: %v", err)
	}

	if len(followers) != 3 {
		t.Errorf("Expected 3 followers, got %d", len(followers))
	}
}

func TestUserRepository_GetFollowerIDs(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := NewUserRepository(pool)

	ctx := context.Background()
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	testutil.CleanupTables(t, pool, "follows", "users")

	mainUser := &models.User{
		ID:          uuid.New().String(),
		Username:    "main_" + uuid.New().String()[:8],
		Email:       "main_" + uuid.New().String()[:8] + "@example.com",
		DisplayName: "Main User",
		CreatedAt:   time.Now(),
	}
	if err := repo.Create(ctx, mainUser); err != nil {
		t.Fatalf("Failed to create main user: %v", err)
	}

	// Create and follow
	followerIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		follower := &models.User{
			ID:          uuid.New().String(),
			Username:    "f_" + uuid.New().String()[:8],
			Email:       "f_" + uuid.New().String()[:8] + "@example.com",
			DisplayName: "Follower",
			CreatedAt:   time.Now(),
		}
		followerIDs[i] = follower.ID
		if err := repo.Create(ctx, follower); err != nil {
			t.Fatalf("Failed to create follower: %v", err)
		}
		if err := repo.Follow(ctx, follower.ID, mainUser.ID); err != nil {
			t.Fatalf("Failed to follow: %v", err)
		}
	}

	// Get follower IDs
	ids, err := repo.GetFollowerIDs(ctx, mainUser.ID)
	if err != nil {
		t.Fatalf("Failed to get follower IDs: %v", err)
	}

	if len(ids) != 3 {
		t.Errorf("Expected 3 follower IDs, got %d", len(ids))
	}
}
