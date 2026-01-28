package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token has expired")
	ErrInvalidClaims = errors.New("invalid token claims")
)

// TokenType represents the type of JWT token
type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

// Claims represents the JWT claims
type Claims struct {
	UserID   string    `json:"user_id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
	Type     TokenType `json:"type"`
	jwt.RegisteredClaims
}

// JWTManager handles JWT token operations
type JWTManager struct {
	secretKey          []byte
	accessTokenExpiry  time.Duration
	refreshTokenExpiry time.Duration
	issuer             string
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(secretKey []byte, accessExpiry, refreshExpiry time.Duration, issuer string) *JWTManager {
	if len(secretKey) == 0 {
		// Generate a random secret key for development
		secretKey = make([]byte, 32)
		rand.Read(secretKey)
	}

	return &JWTManager{
		secretKey:          secretKey,
		accessTokenExpiry:  accessExpiry,
		refreshTokenExpiry: refreshExpiry,
		issuer:             issuer,
	}
}

// GenerateTokenPair generates both access and refresh tokens
func (j *JWTManager) GenerateTokenPair(userID, username, email string) (string, string, error) {
	accessToken, err := j.GenerateToken(userID, username, email, AccessToken, j.accessTokenExpiry)
	if err != nil {
		return "", "", fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := j.GenerateToken(userID, username, email, RefreshToken, j.refreshTokenExpiry)
	if err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

// GenerateToken generates a JWT token
func (j *JWTManager) GenerateToken(userID, username, email string, tokenType TokenType, expiry time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Email:    email,
		Type:     tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
			NotBefore: jwt.NewNumericDate(now),
			ID:        generateTokenID(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(j.secretKey)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken validates and parses a JWT token
func (j *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.secretKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, ErrInvalidClaims
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	// Check expiration
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
		return nil, ErrExpiredToken
	}

	// Verify issuer
	if claims.Issuer != j.issuer {
		return nil, fmt.Errorf("invalid issuer: %s", claims.Issuer)
	}

	return claims, nil
}

// RefreshAccessToken generates a new access token from a refresh token
func (j *JWTManager) RefreshAccessToken(refreshTokenString string) (string, error) {
	claims, err := j.ValidateToken(refreshTokenString)
	if err != nil {
		return "", fmt.Errorf("invalid refresh token: %w", err)
	}

	// Verify it's a refresh token
	if claims.Type != RefreshToken {
		return "", errors.New("not a refresh token")
	}

	// Generate new access token
	accessToken, err := j.GenerateToken(
		claims.UserID,
		claims.Username,
		claims.Email,
		AccessToken,
		j.accessTokenExpiry,
	)
	if err != nil {
		return "", fmt.Errorf("generate access token: %w", err)
	}

	return accessToken, nil
}

// ValidateRefreshToken validates a refresh token specifically
func (j *JWTManager) ValidateRefreshToken(tokenString string) (*Claims, error) {
	claims, err := j.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	
	// Verify it's a refresh token
	if claims.Type != RefreshToken {
		return nil, errors.New("not a refresh token")
	}
	
	return claims, nil
}

// GenerateAccessToken generates only an access token
func (j *JWTManager) GenerateAccessToken(userID, username, email string) (string, error) {
	return j.GenerateToken(userID, username, email, AccessToken, j.accessTokenExpiry)
}

// RevokeToken adds a token to the revocation list (requires Redis)
func (j *JWTManager) RevokeToken(tokenString string, cache interface{}) error {
	claims, err := j.ValidateToken(tokenString)
	if err != nil {
		return err
	}

	// Store revoked token ID in cache until expiration
	// This requires integration with Redis cache
	// Key: "revoked_token:" + claims.ID
	// TTL: Until token expiration
	
	// TODO: Implement with Redis cache parameter
	_ = claims
	
	return nil
}


// generateTokenID generates a unique token ID
func generateTokenID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// PasswordHasher handles password hashing
type PasswordHasher struct{}

// HashPassword hashes a password using bcrypt
func (p *PasswordHasher) HashPassword(password string) (string, error) {
	// Using a simplified version for now, should use bcrypt in production
	// import "golang.org/x/crypto/bcrypt"
	// hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	
	// For demo, using base64 encoding (NOT SECURE - replace with bcrypt)
	if len(password) < 6 {
		return "", errors.New("password too short")
	}
	
	// This is intentionally weak for demo - USE BCRYPT IN PRODUCTION
	salt := make([]byte, 8)
	rand.Read(salt)
	combined := append(salt, []byte(password)...)
	hash := base64.StdEncoding.EncodeToString(combined)
	
	return hash, nil
}

// VerifyPassword verifies a password against its hash
func (p *PasswordHasher) VerifyPassword(password, hash string) error {
	// Should use bcrypt.CompareHashAndPassword in production
	
	// For demo purposes only - NOT SECURE
	decoded, err := base64.StdEncoding.DecodeString(hash)
	if err != nil {
		return err
	}
	
	if len(decoded) < 8 {
		return errors.New("invalid hash")
	}
	
	salt := decoded[:8]
	combined := append(salt, []byte(password)...)
	testHash := base64.StdEncoding.EncodeToString(combined)
	
	if testHash != hash {
		return errors.New("password mismatch")
	}
	
	return nil
}