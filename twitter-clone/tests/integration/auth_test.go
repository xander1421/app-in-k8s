package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexprut/twitter-clone/pkg/auth"
	"github.com/alexprut/twitter-clone/pkg/models"
	"github.com/alexprut/twitter-clone/services/user-service/internal/handlers"
	"github.com/alexprut/twitter-clone/services/user-service/internal/service"
)

// TestAuthFlow tests complete authentication flow
func TestAuthFlow(t *testing.T) {
	// Setup
	ctx := context.Background()
	testDB := setupTestDatabase(t)
	defer testDB.Close()

	jwtManager := auth.NewJWTManager(
		[]byte("test-secret"),
		15*time.Minute,
		7*24*time.Hour,
		"test-issuer",
	)

	authRepo := setupAuthRepository(testDB)
	authService := service.NewAuthService(authRepo, jwtManager, nil)
	authHandler := handlers.NewAuthHandler(authService, jwtManager)

	// Create test server
	mux := http.NewServeMux()
	authHandler.RegisterRoutes(mux)
	server := httptest.NewServer(mux)
	defer server.Close()

	t.Run("Register New User", func(t *testing.T) {
		// Prepare request
		registerReq := models.RegisterRequest{
			Username: "testuser",
			Email:    "test@example.com",
			Password: "SecureP@ss123",
			Name:     "Test User",
		}

		body, _ := json.Marshal(registerReq)
		req, err := http.NewRequest("POST", server.URL+"/api/v1/auth/register", bytes.NewBuffer(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")

		// Send request
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		// Check response
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", resp.StatusCode)
		}

		var authResp models.AuthResponse
		if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
			t.Fatal(err)
		}

		// Validate response
		if authResp.AccessToken == "" {
			t.Error("Expected access token")
		}
		if authResp.RefreshToken == "" {
			t.Error("Expected refresh token")
		}
		if authResp.User.Username != "testuser" {
			t.Errorf("Expected username testuser, got %s", authResp.User.Username)
		}
	})

	t.Run("Login with Credentials", func(t *testing.T) {
		// First register a user
		registerUser(t, server.URL, "loginuser", "login@example.com", "SecureP@ss123")

		// Login request
		loginReq := models.LoginRequest{
			Email:    "login@example.com",
			Password: "SecureP@ss123",
		}

		body, _ := json.Marshal(loginReq)
		req, err := http.NewRequest("POST", server.URL+"/api/v1/auth/login", bytes.NewBuffer(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")

		// Send request
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		// Check response
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var authResp models.AuthResponse
		if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
			t.Fatal(err)
		}

		// Validate tokens
		claims, err := jwtManager.ValidateAccessToken(authResp.AccessToken)
		if err != nil {
			t.Errorf("Invalid access token: %v", err)
		}
		if claims.Email != "login@example.com" {
			t.Errorf("Expected email login@example.com, got %s", claims.Email)
		}
	})

	t.Run("Refresh Token", func(t *testing.T) {
		// Register and login
		authResp := registerUser(t, server.URL, "refreshuser", "refresh@example.com", "SecureP@ss123")

		// Wait a bit
		time.Sleep(100 * time.Millisecond)

		// Refresh token request
		refreshReq := map[string]string{
			"refresh_token": authResp.RefreshToken,
		}

		body, _ := json.Marshal(refreshReq)
		req, err := http.NewRequest("POST", server.URL+"/api/v1/auth/refresh", bytes.NewBuffer(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")

		// Send request
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		// Check response
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		var newAuthResp models.AuthResponse
		if err := json.NewDecoder(resp.Body).Decode(&newAuthResp); err != nil {
			t.Fatal(err)
		}

		// New access token should be different
		if newAuthResp.AccessToken == authResp.AccessToken {
			t.Error("Expected new access token")
		}
	})

	t.Run("Invalid Credentials", func(t *testing.T) {
		loginReq := models.LoginRequest{
			Email:    "nonexistent@example.com",
			Password: "WrongPassword",
		}

		body, _ := json.Marshal(loginReq)
		req, err := http.NewRequest("POST", server.URL+"/api/v1/auth/login", bytes.NewBuffer(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		// Should return 401
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", resp.StatusCode)
		}
	})

	t.Run("Password Requirements", func(t *testing.T) {
		testCases := []struct {
			password string
			valid    bool
		}{
			{"short", false},           // Too short
			{"nouppercase123", false},  // No uppercase
			{"NOLOWERCASE123", false},  // No lowercase
			{"NoDigits!", false},       // No digits
			{"ValidP@ss123", true},     // Valid
		}

		for _, tc := range testCases {
			err := auth.ValidatePassword(tc.password, auth.DefaultPasswordConfig())
			if tc.valid && err != nil {
				t.Errorf("Password %s should be valid but got error: %v", tc.password, err)
			}
			if !tc.valid && err == nil {
				t.Errorf("Password %s should be invalid but got no error", tc.password)
			}
		}
	})

	t.Run("Account Lockout", func(t *testing.T) {
		// Register user
		registerUser(t, server.URL, "lockuser", "lock@example.com", "SecureP@ss123")

		// Attempt login with wrong password 5 times
		for i := 0; i < 5; i++ {
			loginReq := models.LoginRequest{
				Email:    "lock@example.com",
				Password: "WrongPassword",
			}

			body, _ := json.Marshal(loginReq)
			req, err := http.NewRequest("POST", server.URL+"/api/v1/auth/login", bytes.NewBuffer(body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
		}

		// Now try with correct password - should be locked
		loginReq := models.LoginRequest{
			Email:    "lock@example.com",
			Password: "SecureP@ss123",
		}

		body, _ := json.Marshal(loginReq)
		req, err := http.NewRequest("POST", server.URL+"/api/v1/auth/login", bytes.NewBuffer(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		// Should be locked
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected account to be locked, got status %d", resp.StatusCode)
		}
	})
}

// TestPasswordHashing tests password hashing functionality
func TestPasswordHashing(t *testing.T) {
	password := "MySecureP@ssw0rd"

	t.Run("Hash and Verify", func(t *testing.T) {
		hash, err := auth.HashPassword(password)
		if err != nil {
			t.Fatal(err)
		}

		// Hash should not be the same as password
		if hash == password {
			t.Error("Hash should not equal password")
		}

		// Should verify correctly
		if !auth.CheckPassword(password, hash) {
			t.Error("Password verification failed")
		}

		// Wrong password should not verify
		if auth.CheckPassword("WrongPassword", hash) {
			t.Error("Wrong password should not verify")
		}
	})

	t.Run("Different Hashes", func(t *testing.T) {
		hash1, _ := auth.HashPassword(password)
		hash2, _ := auth.HashPassword(password)

		// Same password should produce different hashes (due to salt)
		if hash1 == hash2 {
			t.Error("Same password produced identical hashes")
		}

		// Both should verify
		if !auth.CheckPassword(password, hash1) || !auth.CheckPassword(password, hash2) {
			t.Error("Hash verification failed")
		}
	})

	t.Run("Password Strength", func(t *testing.T) {
		testCases := []struct {
			password string
			minScore int
		}{
			{"weak", 20},
			{"moderate123", 40},
			{"Strong123!", 60},
			{"VeryStr0ng!P@ssw0rd", 80},
		}

		for _, tc := range testCases {
			score := auth.GetPasswordStrength(tc.password)
			if score < tc.minScore {
				t.Errorf("Password %s expected min score %d, got %d", tc.password, tc.minScore, score)
			}
		}
	})
}

// TestJWTTokens tests JWT token functionality
func TestJWTTokens(t *testing.T) {
	jwtManager := auth.NewJWTManager(
		[]byte("test-secret"),
		15*time.Minute,
		7*24*time.Hour,
		"test-issuer",
	)

	userID := "user123"
	username := "testuser"
	email := "test@example.com"

	t.Run("Generate and Validate Access Token", func(t *testing.T) {
		token, err := jwtManager.GenerateAccessToken(userID, username, email)
		if err != nil {
			t.Fatal(err)
		}

		// Validate token
		claims, err := jwtManager.ValidateAccessToken(token)
		if err != nil {
			t.Fatal(err)
		}

		// Check claims
		if claims.UserID != userID {
			t.Errorf("Expected UserID %s, got %s", userID, claims.UserID)
		}
		if claims.Username != username {
			t.Errorf("Expected Username %s, got %s", username, claims.Username)
		}
		if claims.Email != email {
			t.Errorf("Expected Email %s, got %s", email, claims.Email)
		}
	})

	t.Run("Expired Token", func(t *testing.T) {
		// Create manager with very short expiry
		shortJWT := auth.NewJWTManager(
			[]byte("test-secret"),
			1*time.Millisecond,
			1*time.Millisecond,
			"test-issuer",
		)

		token, err := shortJWT.GenerateAccessToken(userID, username, email)
		if err != nil {
			t.Fatal(err)
		}

		// Wait for expiry
		time.Sleep(10 * time.Millisecond)

		// Should fail validation
		_, err = shortJWT.ValidateAccessToken(token)
		if err == nil {
			t.Error("Expected expired token error")
		}
	})

	t.Run("Invalid Token", func(t *testing.T) {
		invalidTokens := []string{
			"invalid.token.here",
			"",
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.invalid.signature",
		}

		for _, token := range invalidTokens {
			_, err := jwtManager.ValidateAccessToken(token)
			if err == nil {
				t.Errorf("Expected error for invalid token: %s", token)
			}
		}
	})

	t.Run("Token with Wrong Secret", func(t *testing.T) {
		// Generate with one secret
		token, _ := jwtManager.GenerateAccessToken(userID, username, email)

		// Try to validate with different secret
		otherJWT := auth.NewJWTManager(
			[]byte("different-secret"),
			15*time.Minute,
			7*24*time.Hour,
			"test-issuer",
		)

		_, err := otherJWT.ValidateAccessToken(token)
		if err == nil {
			t.Error("Expected error when validating with wrong secret")
		}
	})
}

// Helper functions

func registerUser(t *testing.T, serverURL, username, email, password string) *models.AuthResponse {
	registerReq := models.RegisterRequest{
		Username: username,
		Email:    email,
		Password: password,
		Name:     fmt.Sprintf("%s User", username),
	}

	body, _ := json.Marshal(registerReq)
	req, err := http.NewRequest("POST", serverURL+"/api/v1/auth/register", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var authResp models.AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		t.Fatal(err)
	}

	return &authResp
}