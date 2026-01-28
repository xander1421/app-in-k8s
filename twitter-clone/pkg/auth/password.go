package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrPasswordTooShort     = errors.New("password must be at least 8 characters")
	ErrPasswordTooWeak      = errors.New("password must contain uppercase, lowercase, digit and special character")
	ErrPasswordCompromised  = errors.New("password has been found in data breaches")
	ErrInvalidPasswordHash  = errors.New("invalid password hash format")
)

// PasswordConfig defines password requirements
type PasswordConfig struct {
	MinLength            int
	RequireUppercase     bool
	RequireLowercase     bool
	RequireDigit         bool
	RequireSpecial       bool
	BcryptCost          int
	CheckHaveIBeenPwned bool
}

// DefaultPasswordConfig returns default password configuration
func DefaultPasswordConfig() *PasswordConfig {
	return &PasswordConfig{
		MinLength:            8,
		RequireUppercase:     true,
		RequireLowercase:     true,
		RequireDigit:         true,
		RequireSpecial:       false,
		BcryptCost:          12, // Good balance between security and performance
		CheckHaveIBeenPwned: false,
	}
}

// HashPassword hashes a password using bcrypt with configurable cost
func HashPassword(password string) (string, error) {
	return HashPasswordWithCost(password, bcrypt.DefaultCost)
}

// HashPasswordWithCost hashes a password using bcrypt with specified cost
func HashPasswordWithCost(password string, cost int) (string, error) {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		cost = bcrypt.DefaultCost
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	return string(hash), nil
}

// CheckPassword verifies a password against its bcrypt hash
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// ValidatePassword checks if a password meets security requirements
func ValidatePassword(password string, config *PasswordConfig) error {
	if config == nil {
		config = DefaultPasswordConfig()
	}

	// Check minimum length
	if len(password) < config.MinLength {
		return fmt.Errorf("%w: minimum %d characters required", ErrPasswordTooShort, config.MinLength)
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	// Check requirements
	if config.RequireUppercase && !hasUpper {
		return fmt.Errorf("%w: missing uppercase letter", ErrPasswordTooWeak)
	}
	if config.RequireLowercase && !hasLower {
		return fmt.Errorf("%w: missing lowercase letter", ErrPasswordTooWeak)
	}
	if config.RequireDigit && !hasDigit {
		return fmt.Errorf("%w: missing digit", ErrPasswordTooWeak)
	}
	if config.RequireSpecial && !hasSpecial {
		return fmt.Errorf("%w: missing special character", ErrPasswordTooWeak)
	}

	// Check against common passwords (basic list)
	if isCommonPassword(password) {
		return errors.New("password is too common, please choose a different one")
	}

	return nil
}

// GenerateSecurePassword generates a cryptographically secure random password
func GenerateSecurePassword(length int) (string, error) {
	if length < 8 {
		length = 16 // Default to 16 characters
	}

	// Character sets for password generation
	const (
		uppercase = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		lowercase = "abcdefghijklmnopqrstuvwxyz"
		digits    = "0123456789"
		special   = "!@#$%^&*()_+-=[]{}|;:,.<>?"
	)

	// Combine all character sets
	allChars := uppercase + lowercase + digits + special
	password := make([]byte, length)

	// Ensure at least one character from each set
	password[0] = uppercase[randInt(len(uppercase))]
	password[1] = lowercase[randInt(len(lowercase))]
	password[2] = digits[randInt(len(digits))]
	password[3] = special[randInt(len(special))]

	// Fill the rest randomly
	for i := 4; i < length; i++ {
		password[i] = allChars[randInt(len(allChars))]
	}

	// Shuffle the password
	for i := len(password) - 1; i > 0; i-- {
		j := randInt(i + 1)
		password[i], password[j] = password[j], password[i]
	}

	return string(password), nil
}

// GenerateSecureToken generates a cryptographically secure random token
func GenerateSecureToken() string {
	return GenerateSecureTokenWithLength(32)
}

// GenerateSecureTokenWithLength generates a secure token of specified length
func GenerateSecureTokenWithLength(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		panic(err) // This should never happen
	}
	return base64.URLEncoding.EncodeToString(b)
}

// GetPasswordStrength returns a score from 0-100 indicating password strength
func GetPasswordStrength(password string) int {
	score := 0
	length := len(password)

	// Length score (max 30 points)
	if length >= 8 {
		score += 10
	}
	if length >= 12 {
		score += 10
	}
	if length >= 16 {
		score += 10
	}

	// Character variety score (max 40 points)
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	uniqueChars := make(map[rune]bool)

	for _, char := range password {
		uniqueChars[char] = true
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	if hasUpper {
		score += 10
	}
	if hasLower {
		score += 10
	}
	if hasDigit {
		score += 10
	}
	if hasSpecial {
		score += 10
	}

	// Uniqueness score (max 20 points)
	uniqueRatio := float64(len(uniqueChars)) / float64(length)
	score += int(uniqueRatio * 20)

	// Pattern detection (max 10 points)
	if !hasRepeatingPatterns(password) {
		score += 10
	}

	// Cap at 100
	if score > 100 {
		score = 100
	}

	return score
}

// NeedsRehash checks if a password hash needs to be updated
func NeedsRehash(hash string) bool {
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		return true // If we can't determine the cost, rehash it
	}

	// If the cost is less than our current default, needs rehash
	return cost < 12
}

// Helper functions

func randInt(max int) int {
	b := make([]byte, 1)
	rand.Read(b)
	return int(b[0]) % max
}

func hasRepeatingPatterns(password string) bool {
	// Check for repeating characters (aaa, 111, etc.)
	for i := 0; i < len(password)-2; i++ {
		if password[i] == password[i+1] && password[i] == password[i+2] {
			return true
		}
	}

	// Check for sequential patterns (abc, 123, etc.)
	for i := 0; i < len(password)-2; i++ {
		if password[i+1] == password[i]+1 && password[i+2] == password[i]+2 {
			return true
		}
	}

	return false
}

func isCommonPassword(password string) bool {
	// Basic list of common passwords
	commonPasswords := map[string]bool{
		"password":  true,
		"123456":    true,
		"12345678":  true,
		"qwerty":    true,
		"abc123":    true,
		"monkey":    true,
		"1234567":   true,
		"letmein":   true,
		"trustno1":  true,
		"dragon":    true,
		"baseball":  true,
		"111111":    true,
		"iloveyou":  true,
		"master":    true,
		"sunshine":  true,
		"ashley":    true,
		"bailey":    true,
		"passw0rd":  true,
		"shadow":    true,
		"123123":    true,
		"654321":    true,
		"superman":  true,
		"welcome":   true,
		"football":  true,
		"admin":     true,
	}

	return commonPasswords[password]
}

// PasswordHashInfo provides information about a password hash
type PasswordHashInfo struct {
	Algorithm string
	Cost      int
	Valid     bool
}

// GetHashInfo returns information about a password hash
func GetHashInfo(hash string) (*PasswordHashInfo, error) {
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		return &PasswordHashInfo{
			Algorithm: "unknown",
			Valid:     false,
		}, err
	}

	return &PasswordHashInfo{
		Algorithm: "bcrypt",
		Cost:      cost,
		Valid:     true,
	}, nil
}