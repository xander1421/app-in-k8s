package moderation

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/alexprut/twitter-clone/pkg/cache"
)

// ContentModerator handles content moderation
type ContentModerator struct {
	cache           *cache.RedisCache
	profanityList   []string
	spamPatterns    []*regexp.Regexp
	urlShorteners   []string
	maxMentions     int
	maxHashtags     int
	maxURLs         int
	maxCaps         float64 // percentage
	minWordLength   int
	duplicateWindow time.Duration
}

// NewContentModerator creates a new content moderator
func NewContentModerator(cache *cache.RedisCache) *ContentModerator {
	return &ContentModerator{
		cache:           cache,
		profanityList:   loadProfanityList(),
		spamPatterns:    loadSpamPatterns(),
		urlShorteners:   loadURLShorteners(),
		maxMentions:     10,
		maxHashtags:     10,
		maxURLs:         5,
		maxCaps:         0.5, // 50% caps
		minWordLength:   2,
		duplicateWindow: 1 * time.Hour,
	}
}

// ModerationResult contains the results of content moderation
type ModerationResult struct {
	IsClean        bool
	Issues         []string
	Score          float64 // 0-1, higher is more problematic
	RequiresReview bool
	SuggestedAction string
}

// ModerateContent performs comprehensive content moderation
func (m *ContentModerator) ModerateContent(ctx context.Context, content string, userID string) (*ModerationResult, error) {
	result := &ModerationResult{
		IsClean: true,
		Issues:  []string{},
		Score:   0.0,
	}

	// Check for profanity
	if hasProfanity, words := m.checkProfanity(content); hasProfanity {
		result.Issues = append(result.Issues, fmt.Sprintf("Contains profanity: %v", words))
		result.Score += 0.5
	}

	// Check for spam patterns
	if isSpam, patterns := m.checkSpamPatterns(content); isSpam {
		result.Issues = append(result.Issues, fmt.Sprintf("Matches spam patterns: %v", patterns))
		result.Score += 0.4
	}

	// Check for excessive mentions
	mentions := extractMentions(content)
	if len(mentions) > m.maxMentions {
		result.Issues = append(result.Issues, fmt.Sprintf("Too many mentions: %d (max %d)", len(mentions), m.maxMentions))
		result.Score += 0.3
	}

	// Check for excessive hashtags
	hashtags := extractHashtags(content)
	if len(hashtags) > m.maxHashtags {
		result.Issues = append(result.Issues, fmt.Sprintf("Too many hashtags: %d (max %d)", len(hashtags), m.maxHashtags))
		result.Score += 0.2
	}

	// Check for excessive URLs
	urls := extractURLs(content)
	if len(urls) > m.maxURLs {
		result.Issues = append(result.Issues, fmt.Sprintf("Too many URLs: %d (max %d)", len(urls), m.maxURLs))
		result.Score += 0.3
	}

	// Check for suspicious URLs
	if hasSuspicious, suspiciousURLs := m.checkSuspiciousURLs(urls); hasSuspicious {
		result.Issues = append(result.Issues, fmt.Sprintf("Suspicious URLs: %v", suspiciousURLs))
		result.Score += 0.4
	}

	// Check for excessive caps
	if capsRatio := m.calculateCapsRatio(content); capsRatio > m.maxCaps {
		result.Issues = append(result.Issues, fmt.Sprintf("Excessive caps: %.0f%%", capsRatio*100))
		result.Score += 0.1
	}

	// Check for duplicate content
	if isDuplicate, err := m.checkDuplicate(ctx, content, userID); err == nil && isDuplicate {
		result.Issues = append(result.Issues, "Duplicate content detected")
		result.Score += 0.6
	}

	// Check for gibberish
	if isGibberish := m.checkGibberish(content); isGibberish {
		result.Issues = append(result.Issues, "Content appears to be gibberish")
		result.Score += 0.3
	}

	// Check user's spam history
	if isSpammer, err := m.checkUserSpamHistory(ctx, userID); err == nil && isSpammer {
		result.Issues = append(result.Issues, "User has spam history")
		result.Score += 0.2
	}

	// Determine final status
	if result.Score > 0 {
		result.IsClean = false
	}

	if result.Score >= 0.7 {
		result.SuggestedAction = "block"
		result.RequiresReview = false
	} else if result.Score >= 0.4 {
		result.SuggestedAction = "review"
		result.RequiresReview = true
	} else if result.Score >= 0.2 {
		result.SuggestedAction = "warning"
		result.RequiresReview = false
	} else {
		result.SuggestedAction = "allow"
		result.RequiresReview = false
	}

	return result, nil
}

// checkProfanity checks for profane words
func (m *ContentModerator) checkProfanity(content string) (bool, []string) {
	found := []string{}
	lowerContent := strings.ToLower(content)
	
	for _, word := range m.profanityList {
		if strings.Contains(lowerContent, word) {
			found = append(found, word)
		}
	}
	
	return len(found) > 0, found
}

// checkSpamPatterns checks for common spam patterns
func (m *ContentModerator) checkSpamPatterns(content string) (bool, []string) {
	matches := []string{}
	
	for _, pattern := range m.spamPatterns {
		if pattern.MatchString(content) {
			matches = append(matches, pattern.String())
		}
	}
	
	return len(matches) > 0, matches
}

// checkSuspiciousURLs checks for suspicious URLs
func (m *ContentModerator) checkSuspiciousURLs(urls []string) (bool, []string) {
	suspicious := []string{}
	
	for _, url := range urls {
		// Check for URL shorteners
		for _, shortener := range m.urlShorteners {
			if strings.Contains(url, shortener) {
				suspicious = append(suspicious, url)
				break
			}
		}
		
		// Check for suspicious TLDs
		suspiciousTLDs := []string{".tk", ".ml", ".ga", ".cf"}
		for _, tld := range suspiciousTLDs {
			if strings.Contains(url, tld) {
				suspicious = append(suspicious, url)
				break
			}
		}
	}
	
	return len(suspicious) > 0, suspicious
}

// calculateCapsRatio calculates the ratio of capital letters
func (m *ContentModerator) calculateCapsRatio(content string) float64 {
	if len(content) == 0 {
		return 0
	}
	
	totalLetters := 0
	capsLetters := 0
	
	for _, r := range content {
		if unicode.IsLetter(r) {
			totalLetters++
			if unicode.IsUpper(r) {
				capsLetters++
			}
		}
	}
	
	if totalLetters == 0 {
		return 0
	}
	
	return float64(capsLetters) / float64(totalLetters)
}

// checkDuplicate checks if content is duplicate
func (m *ContentModerator) checkDuplicate(ctx context.Context, content string, userID string) (bool, error) {
	// Create content hash
	hash := m.hashContent(content)
	key := fmt.Sprintf("content:hash:%s:%s", userID, hash)
	
	// Check if hash exists
	exists, err := m.cache.Exists(ctx, key)
	if err != nil {
		return false, err
	}
	
	if exists {
		return true, nil
	}
	
	// Store hash with expiration
	err = m.cache.Set(ctx, key, "1", m.duplicateWindow)
	return false, err
}

// checkGibberish checks if content appears to be gibberish
func (m *ContentModerator) checkGibberish(content string) bool {
	words := strings.Fields(content)
	if len(words) == 0 {
		return true
	}
	
	shortWords := 0
	consonantClusters := 0
	
	for _, word := range words {
		// Check for too many short words
		if len(word) < m.minWordLength {
			shortWords++
		}
		
		// Check for unusual consonant clusters
		if hasUnusualConsonants(word) {
			consonantClusters++
		}
	}
	
	// If more than 50% are short words or have consonant clusters
	shortWordRatio := float64(shortWords) / float64(len(words))
	clusterRatio := float64(consonantClusters) / float64(len(words))
	
	return shortWordRatio > 0.5 || clusterRatio > 0.3
}

// checkUserSpamHistory checks user's spam history
func (m *ContentModerator) checkUserSpamHistory(ctx context.Context, userID string) (bool, error) {
	key := fmt.Sprintf("spam:history:%s", userID)
	count, err := m.cache.GetCounter(ctx, key)
	if err != nil {
		return false, err
	}
	
	// If user has more than 5 spam incidents in history
	return count > 5, nil
}

// ReportSpam marks content as spam
func (m *ContentModerator) ReportSpam(ctx context.Context, contentID string, userID string, reporterID string) error {
	// Increment spam report count
	reportKey := fmt.Sprintf("spam:reports:%s", contentID)
	count, err := m.cache.IncrCounter(ctx, reportKey)
	if err != nil {
		return err
	}
	
	// If multiple reports, add to user's spam history
	if count >= 3 {
		historyKey := fmt.Sprintf("spam:history:%s", userID)
		m.cache.IncrCounter(ctx, historyKey)
	}
	
	// Store reporter to prevent duplicate reports
	reporterKey := fmt.Sprintf("spam:reporters:%s:%s", contentID, reporterID)
	m.cache.Set(ctx, reporterKey, "1", 7*24*time.Hour)
	
	return nil
}

// RateLimitCheck checks if user is rate limited
func (m *ContentModerator) RateLimitCheck(ctx context.Context, userID string, action string) (bool, error) {
	// Different limits for different actions
	limits := map[string]struct {
		Max    int
		Window time.Duration
	}{
		"tweet":  {Max: 100, Window: 1 * time.Hour},
		"follow": {Max: 50, Window: 1 * time.Hour},
		"like":   {Max: 200, Window: 1 * time.Hour},
		"dm":     {Max: 50, Window: 1 * time.Hour},
	}
	
	limit, exists := limits[action]
	if !exists {
		limit = struct {
			Max    int
			Window time.Duration
		}{Max: 100, Window: 1 * time.Hour}
	}
	
	key := fmt.Sprintf("ratelimit:%s:%s:%d", action, userID, time.Now().Unix()/int64(limit.Window.Seconds()))
	count, err := m.cache.IncrCounter(ctx, key)
	if err != nil {
		return false, err
	}
	
	// Set expiration on first increment
	if count == 1 {
		m.cache.Client().Expire(ctx, key, limit.Window)
	}
	
	return count > int64(limit.Max), nil
}

// Helper functions

func (m *ContentModerator) hashContent(content string) string {
	h := md5.New()
	h.Write([]byte(strings.TrimSpace(strings.ToLower(content))))
	return hex.EncodeToString(h.Sum(nil))
}

func hasUnusualConsonants(word string) bool {
	consonants := 0
	maxConsecutive := 0
	current := 0
	
	for _, r := range strings.ToLower(word) {
		if isConsonant(r) {
			consonants++
			current++
			if current > maxConsecutive {
				maxConsecutive = current
			}
		} else {
			current = 0
		}
	}
	
	// Flag if more than 4 consecutive consonants
	return maxConsecutive > 4
}

func isConsonant(r rune) bool {
	vowels := "aeiou"
	return unicode.IsLetter(r) && !strings.ContainsRune(vowels, r)
}

func extractMentions(content string) []string {
	re := regexp.MustCompile(`@[\w]+`)
	return re.FindAllString(content, -1)
}

func extractHashtags(content string) []string {
	re := regexp.MustCompile(`#[\w]+`)
	return re.FindAllString(content, -1)
}

func extractURLs(content string) []string {
	re := regexp.MustCompile(`https?://[^\s]+`)
	return re.FindAllString(content, -1)
}

func loadProfanityList() []string {
	// In production, load from file or database
	return []string{
		"spam", "scam", "phishing", 
		// Add more words
	}
}

func loadSpamPatterns() []*regexp.Regexp {
	patterns := []string{
		`(?i)click here now`,
		`(?i)limited time offer`,
		`(?i)act now`,
		`(?i)100% free`,
		`(?i)no credit card`,
		`(?i)congratulations you (have )?won`,
		`(?i)claim your (free )?prize`,
		`(?i)make money fast`,
		`(?i)work from home`,
		`(?i)lose weight fast`,
	}
	
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}
	
	return compiled
}

func loadURLShorteners() []string {
	return []string{
		"bit.ly", "tinyurl.com", "goo.gl", "ow.ly",
		"t.co", "buff.ly", "short.link", "is.gd",
	}
}