package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/alexprut/twitter-clone/pkg/cache"
	"github.com/alexprut/twitter-clone/pkg/models"
	"github.com/alexprut/twitter-clone/pkg/queue"
	"github.com/alexprut/twitter-clone/pkg/search"
	"github.com/alexprut/twitter-clone/services/user-service/internal/repository"
)

type UserService struct {
	repo   *repository.UserRepository
	cache  *cache.RedisCache
	search *search.ElasticsearchClient
	queue  *queue.RabbitMQ
}

func NewUserService(
	repo *repository.UserRepository,
	cache *cache.RedisCache,
	search *search.ElasticsearchClient,
	queue *queue.RabbitMQ,
) *UserService {
	return &UserService{
		repo:   repo,
		cache:  cache,
		search: search,
		queue:  queue,
	}
}

func (s *UserService) CreateUser(ctx context.Context, req models.CreateUserRequest) (*models.User, error) {
	user := &models.User{
		ID:          uuid.New().String(),
		Username:    req.Username,
		Email:       req.Email,
		DisplayName: req.DisplayName,
		CreatedAt:   time.Now(),
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	// Index in Elasticsearch
	if s.search != nil {
		go s.search.IndexUser(context.Background(), user)
	}

	return user, nil
}

func (s *UserService) GetUser(ctx context.Context, userID string) (*models.User, error) {
	// Try cache first
	if s.cache != nil {
		var user models.User
		cacheKey := cache.PrefixUserProfile + userID
		if err := s.cache.Get(ctx, cacheKey, &user); err == nil {
			return &user, nil
		}
	}

	// Get from database
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	// Cache the result
	if s.cache != nil {
		cacheKey := cache.PrefixUserProfile + userID
		s.cache.Set(ctx, cacheKey, user, 5*time.Minute)
	}

	return user, nil
}

func (s *UserService) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	return s.repo.GetByUsername(ctx, username)
}

func (s *UserService) UpdateUser(ctx context.Context, userID string, req models.UpdateUserRequest) (*models.User, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	if req.DisplayName != "" {
		user.DisplayName = req.DisplayName
	}
	if req.Bio != "" {
		user.Bio = req.Bio
	}
	if req.AvatarURL != "" {
		user.AvatarURL = req.AvatarURL
	}

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	// Invalidate cache
	if s.cache != nil {
		s.cache.Delete(ctx, cache.PrefixUserProfile+userID)
	}

	// Update search index
	if s.search != nil {
		go s.search.IndexUser(context.Background(), user)
	}

	return user, nil
}

func (s *UserService) Follow(ctx context.Context, followerID, followeeID string) error {
	if followerID == followeeID {
		return fmt.Errorf("cannot follow yourself")
	}

	// Check if already following
	isFollowing, err := s.repo.IsFollowing(ctx, followerID, followeeID)
	if err != nil {
		return fmt.Errorf("check following: %w", err)
	}
	if isFollowing {
		return nil // Already following
	}

	if err := s.repo.Follow(ctx, followerID, followeeID); err != nil {
		return fmt.Errorf("follow: %w", err)
	}

	// Invalidate cache for both users
	if s.cache != nil {
		s.cache.Delete(ctx, cache.PrefixUserProfile+followerID)
		s.cache.Delete(ctx, cache.PrefixUserProfile+followeeID)
	}

	// Send notification
	if s.queue != nil {
		s.queue.PublishNotification(ctx, followeeID, "follow", followerID, "")
	}

	return nil
}

func (s *UserService) Unfollow(ctx context.Context, followerID, followeeID string) error {
	if err := s.repo.Unfollow(ctx, followerID, followeeID); err != nil {
		return fmt.Errorf("unfollow: %w", err)
	}

	// Invalidate cache for both users
	if s.cache != nil {
		s.cache.Delete(ctx, cache.PrefixUserProfile+followerID)
		s.cache.Delete(ctx, cache.PrefixUserProfile+followeeID)
	}

	return nil
}

func (s *UserService) GetFollowers(ctx context.Context, userID string, limit, offset int) ([]models.User, bool, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// Fetch one extra to determine if there are more
	users, err := s.repo.GetFollowers(ctx, userID, limit+1, offset)
	if err != nil {
		return nil, false, fmt.Errorf("get followers: %w", err)
	}

	hasMore := len(users) > limit
	if hasMore {
		users = users[:limit]
	}

	return users, hasMore, nil
}

func (s *UserService) GetFollowing(ctx context.Context, userID string, limit, offset int) ([]models.User, bool, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	users, err := s.repo.GetFollowing(ctx, userID, limit+1, offset)
	if err != nil {
		return nil, false, fmt.Errorf("get following: %w", err)
	}

	hasMore := len(users) > limit
	if hasMore {
		users = users[:limit]
	}

	return users, hasMore, nil
}

func (s *UserService) GetFollowerIDs(ctx context.Context, userID string) ([]string, error) {
	return s.repo.GetFollowerIDs(ctx, userID)
}

func (s *UserService) GetFollowerCount(ctx context.Context, userID string) (int, error) {
	// Try cache first
	if s.cache != nil {
		count, err := s.cache.GetCounter(ctx, cache.PrefixFollowers+userID)
		if err == nil && count > 0 {
			return int(count), nil
		}
	}

	return s.repo.GetFollowerCount(ctx, userID)
}

func (s *UserService) IsFollowing(ctx context.Context, followerID, followeeID string) (bool, error) {
	return s.repo.IsFollowing(ctx, followerID, followeeID)
}

func (s *UserService) BatchGetUsers(ctx context.Context, ids []string) ([]models.User, error) {
	return s.repo.BatchGetUsers(ctx, ids)
}
