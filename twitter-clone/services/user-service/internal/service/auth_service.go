package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alexprut/twitter-clone/pkg/auth"
	"github.com/alexprut/twitter-clone/pkg/models"
	"github.com/alexprut/twitter-clone/pkg/moderation"
	"github.com/alexprut/twitter-clone/services/user-service/internal/repository"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrAccountLocked      = errors.New("account is locked")
	ErrEmailNotVerified   = errors.New("email not verified")
	ErrTokenExpired       = errors.New("token expired")
	ErrInvalidToken       = errors.New("invalid token")
)

// AuthService handles authentication logic
type AuthService struct {
	repo      *repository.UserRepositoryAuth
	jwt       *auth.JWTManager
	moderator *moderation.ContentModerator
}

// NewAuthService creates a new auth service
func NewAuthService(repo *repository.UserRepositoryAuth, jwt *auth.JWTManager, moderator *moderation.ContentModerator) *AuthService {
	return &AuthService{
		repo:      repo,
		jwt:       jwt,
		moderator: moderator,
	}
}

// Register creates a new user account
func (s *AuthService) Register(ctx context.Context, req *models.SignupRequest) (*models.AuthResponse, error) {
	// Validate username
	if result, err := s.moderator.ModerateContent(ctx, req.Username, ""); err == nil && !result.IsClean {
		return nil, fmt.Errorf("username contains inappropriate content: %s", strings.Join(result.Issues, ", "))
	}

	// Check if user exists by email
	existingUser, _ := s.repo.GetUserByUsernameOrEmail(ctx, req.Username)
	if existingUser != nil {
		return nil, errors.New("email already registered")
	}

	// Check if user exists by username  
	existingUser, _ = s.repo.GetUserByUsernameOrEmail(ctx, req.Username)
	if existingUser != nil {
		return nil, errors.New("username already taken")
	}

	// Hash password
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &models.User{
		Username:    req.Username,
		Email:       req.Username,
		DisplayName: req.DisplayName,
	}

	if err := s.repo.CreateUserWithPassword(ctx, user, passwordHash); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Generate tokens
	accessToken, refreshToken, err := s.jwt.GenerateTokenPair(user.ID, user.Username, user.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// Create session
	session := &models.Session{
		UserID:       user.ID,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
	}

	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Send verification email (would be implemented with email service)
	// s.emailService.SendVerificationEmail(user.Email, verificationToken)

	return &models.AuthResponse{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900, // 15 minutes
	}, nil
}

// Login authenticates a user
func (s *AuthService) Login(ctx context.Context, req *models.LoginRequest) (*models.AuthResponse, error) {
	// Get user by username or email
	user, err := s.repo.GetUserByUsernameOrEmail(ctx, req.Username)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	// Check if account is active
	if !user.IsActive {
		return nil, ErrAccountLocked
	}

	// Verify password using the password package
	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		return nil, ErrInvalidCredentials
	}

	// Update last login time
	now := time.Now()
	user.LastLoginAt = &now
	user.LastActiveAt = &now

	// Generate tokens
	accessToken, refreshToken, err := s.jwt.GenerateTokenPair(user.ID, user.Username, user.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// Create session
	session := &models.Session{
		UserID:       user.ID,
		RefreshToken: refreshToken,
		UserAgent:    "",
		IP:           "",
		ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
	}

	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &models.AuthResponse{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900, // 15 minutes
	}, nil
}

// RefreshToken generates a new access token
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*models.AuthResponse, error) {
	// Validate refresh token
	claims, err := s.jwt.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Get session
	session, err := s.repo.GetSessionByToken(ctx, refreshToken)
	if err != nil {
		return nil, ErrInvalidToken
	}

	if session.ExpiresAt.Before(time.Now()) {
		return nil, ErrTokenExpired
	}

	// Get user
	user, err := s.repo.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Generate new access token
	accessToken, err := s.jwt.GenerateAccessToken(user.ID, user.Username, user.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Update session last used
	s.repo.UpdateSession(ctx, session.ID)

	return &models.AuthResponse{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900, // 15 minutes
	}, nil
}

// Logout invalidates a session
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	return s.repo.DeleteSessionByRefreshToken(ctx, refreshToken)
}
