package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"time"

	"github.com/alexprut/twitter-clone/pkg/models"
)

// AddBookmark adds a tweet to user's bookmarks
func (s *TweetService) AddBookmark(ctx context.Context, bookmark *models.Bookmark) error {
	// Set timestamp
	bookmark.CreatedAt = time.Now()

	// Store in repository
	if err := s.repo.AddBookmark(ctx, bookmark); err != nil {
		return fmt.Errorf("failed to add bookmark: %w", err)
	}

	// Update cache if available
	if s.cache != nil {
		cacheKey := fmt.Sprintf("bookmarks:%s", bookmark.UserID)
		s.cache.Delete(ctx, cacheKey)
		
		// Also invalidate bookmark check cache
		checkKey := fmt.Sprintf("bookmark:%s:%s", bookmark.UserID, bookmark.TweetID)
		s.cache.Set(ctx, checkKey, "true", 24*time.Hour)
	}

	// Track analytics
	s.trackBookmarkEvent(ctx, bookmark.UserID, bookmark.TweetID, "add")

	return nil
}

// RemoveBookmark removes a tweet from user's bookmarks
func (s *TweetService) RemoveBookmark(ctx context.Context, userID, tweetID string) error {
	// Remove from repository
	if err := s.repo.RemoveBookmark(ctx, userID, tweetID); err != nil {
		return fmt.Errorf("failed to remove bookmark: %w", err)
	}

	// Update cache if available
	if s.cache != nil {
		cacheKey := fmt.Sprintf("bookmarks:%s", userID)
		s.cache.Delete(ctx, cacheKey)
		
		// Also invalidate bookmark check cache
		checkKey := fmt.Sprintf("bookmark:%s:%s", userID, tweetID)
		s.cache.Delete(ctx, checkKey)
	}

	// Track analytics
	s.trackBookmarkEvent(ctx, userID, tweetID, "remove")

	return nil
}

// GetUserBookmarks returns paginated bookmarks for a user
func (s *TweetService) GetUserBookmarks(ctx context.Context, userID string, limit int, cursor string) ([]*models.Bookmark, string, error) {
	// Check cache first if available
	if s.cache != nil && cursor == "" {
		cacheKey := fmt.Sprintf("bookmarks:%s", userID)
		var bookmarks []*models.Bookmark
		if err := s.cache.Get(ctx, cacheKey, &bookmarks); err == nil && len(bookmarks) > 0 {
			// Return cached bookmarks
			nextCursor := ""
			if len(bookmarks) > limit {
				nextCursor = fmt.Sprintf("%d", limit)
				bookmarks = bookmarks[:limit]
			}
			return bookmarks, nextCursor, nil
		}
	}

	// Get from repository
	bookmarks, err := s.repo.GetUserBookmarks(ctx, userID, limit+1, cursor)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get bookmarks: %w", err)
	}

	// Check if there are more results
	var nextCursor string
	if len(bookmarks) > limit {
		nextCursor = bookmarks[limit].ID
		bookmarks = bookmarks[:limit]
	}

	// Cache first page if available
	if s.cache != nil && cursor == "" {
		cacheKey := fmt.Sprintf("bookmarks:%s", userID)
		s.cache.Set(ctx, cacheKey, bookmarks, 5*time.Minute)
	}

	return bookmarks, nextCursor, nil
}

// IsBookmarked checks if a tweet is bookmarked by user
func (s *TweetService) IsBookmarked(ctx context.Context, userID, tweetID string) (bool, error) {
	// Check cache first if available
	if s.cache != nil {
		checkKey := fmt.Sprintf("bookmark:%s:%s", userID, tweetID)
		var isBookmarked string
		if err := s.cache.Get(ctx, checkKey, &isBookmarked); err == nil {
			return isBookmarked == "true", nil
		}
	}

	// Check in repository
	isBookmarked, err := s.repo.IsBookmarked(ctx, userID, tweetID)
	if err != nil {
		return false, fmt.Errorf("failed to check bookmark: %w", err)
	}

	// Cache result if available
	if s.cache != nil {
		checkKey := fmt.Sprintf("bookmark:%s:%s", userID, tweetID)
		value := "false"
		if isBookmarked {
			value = "true"
		}
		s.cache.Set(ctx, checkKey, value, 24*time.Hour)
	}

	return isBookmarked, nil
}

// GetBookmarkCount returns the number of bookmarks for a tweet
func (s *TweetService) GetBookmarkCount(ctx context.Context, tweetID string) (int, error) {
	// Check cache first if available
	if s.cache != nil {
		cacheKey := fmt.Sprintf("bookmark_count:%s", tweetID)
		var count int
		if err := s.cache.Get(ctx, cacheKey, &count); err == nil {
			return count, nil
		}
	}

	// Get from repository
	count, err := s.repo.GetBookmarkCount(ctx, tweetID)
	if err != nil {
		return 0, fmt.Errorf("failed to get bookmark count: %w", err)
	}

	// Cache result if available
	if s.cache != nil {
		cacheKey := fmt.Sprintf("bookmark_count:%s", tweetID)
		s.cache.Set(ctx, cacheKey, count, 5*time.Minute)
	}

	return count, nil
}

// GetBookmarkedTweets returns tweets that user has bookmarked
func (s *TweetService) GetBookmarkedTweets(ctx context.Context, userID string, limit int, cursor string) ([]*models.Tweet, string, error) {
	// Get bookmarks first
	bookmarks, nextCursor, err := s.GetUserBookmarks(ctx, userID, limit, cursor)
	if err != nil {
		return nil, "", err
	}

	// Get tweet IDs
	tweetIDs := make([]string, len(bookmarks))
	for i, bookmark := range bookmarks {
		tweetIDs[i] = bookmark.TweetID
	}

	// Get tweets in batch
	tweets, err := s.repo.GetTweetsByIDs(ctx, tweetIDs)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get tweets: %w", err)
	}

	return tweets, nextCursor, nil
}

// trackBookmarkEvent tracks bookmark analytics
func (s *TweetService) trackBookmarkEvent(ctx context.Context, userID, tweetID, action string) {
	if s.queue != nil {
		event := map[string]interface{}{
			"type":      "bookmark",
			"action":    action,
			"user_id":   userID,
			"tweet_id":  tweetID,
			"timestamp": time.Now().Unix(),
		}
		
		// Send to analytics queue  
		fanoutJob := models.FanoutJob{
			Type: "analytics",
			Payload: event,
		}
		s.queue.Publish(ctx, "analytics.bookmark", fanoutJob)
	}
}

// GetPopularBookmarks returns most bookmarked tweets
func (s *TweetService) GetPopularBookmarks(ctx context.Context, limit int, timeRange time.Duration) ([]*models.Tweet, error) {
	// Check cache first if available
	if s.cache != nil {
		cacheKey := fmt.Sprintf("popular_bookmarks:%s", timeRange.String())
		var tweets []*models.Tweet
		if err := s.cache.Get(ctx, cacheKey, &tweets); err == nil && len(tweets) > 0 {
			if len(tweets) > limit {
				tweets = tweets[:limit]
			}
			return tweets, nil
		}
	}

	// Get from repository
	since := time.Now().Add(-timeRange)
	tweetIDs, err := s.repo.GetMostBookmarkedTweets(ctx, limit, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get popular bookmarks: %w", err)
	}

	// Get tweet details
	tweets, err := s.repo.GetTweetsByIDs(ctx, tweetIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get tweets: %w", err)
	}

	// Cache result if available
	if s.cache != nil {
		cacheKey := fmt.Sprintf("popular_bookmarks:%s", timeRange.String())
		s.cache.Set(ctx, cacheKey, tweets, 15*time.Minute)
	}

	return tweets, nil
}

// ExportBookmarks exports user's bookmarks in various formats
func (s *TweetService) ExportBookmarks(ctx context.Context, userID string, format string) ([]byte, error) {
	// Get all bookmarks (currently unused, getting tweets directly)
	_, _, err := s.GetUserBookmarks(ctx, userID, 1000, "")
	if err != nil {
		return nil, err
	}

	// Get tweet details
	tweets, _, err := s.GetBookmarkedTweets(ctx, userID, 1000, "")
	if err != nil {
		return nil, err
	}

	// Export based on format
	switch format {
	case "json":
		return s.exportBookmarksJSON(tweets)
	case "csv":
		return s.exportBookmarksCSV(tweets)
	case "html":
		return s.exportBookmarksHTML(tweets)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// exportBookmarksJSON exports bookmarks as JSON
func (s *TweetService) exportBookmarksJSON(tweets []*models.Tweet) ([]byte, error) {
	return json.MarshalIndent(tweets, "", "  ")
}

// exportBookmarksCSV exports bookmarks as CSV
func (s *TweetService) exportBookmarksCSV(tweets []*models.Tweet) ([]byte, error) {
	
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	
	// Write header
	writer.Write([]string{"ID", "Author", "Content", "Created At", "Likes", "Retweets"})
	
	// Write tweets
	for _, tweet := range tweets {
		writer.Write([]string{
			tweet.ID,
			tweet.AuthorID,
			tweet.Content,
			tweet.CreatedAt.Format(time.RFC3339),
			fmt.Sprintf("%d", tweet.LikeCount),
			fmt.Sprintf("%d", tweet.RetweetCount),
		})
	}
	
	writer.Flush()
	return buf.Bytes(), nil
}

// exportBookmarksHTML exports bookmarks as HTML
func (s *TweetService) exportBookmarksHTML(tweets []*models.Tweet) ([]byte, error) {
	
	const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
    <title>Bookmarked Tweets</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
        .tweet { border: 1px solid #e1e8ed; padding: 15px; margin-bottom: 10px; border-radius: 8px; }
        .author { font-weight: bold; color: #1da1f2; }
        .content { margin: 10px 0; }
        .meta { color: #657786; font-size: 0.9em; }
    </style>
</head>
<body>
    <h1>Your Bookmarked Tweets</h1>
    {{range .}}
    <div class="tweet">
        <div class="author">@{{.UserID}}</div>
        <div class="content">{{.Content}}</div>
        <div class="meta">
            {{.CreatedAt.Format "Jan 2, 2006 3:04 PM"}} · 
            ❤️ {{.LikeCount}} · 
            🔁 {{.RetweetCount}}
        </div>
    </div>
    {{end}}
</body>
</html>`
	
	tmpl, err := template.New("bookmarks").Parse(htmlTemplate)
	if err != nil {
		return nil, err
	}
	
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, tweets); err != nil {
		return nil, err
	}
	
	return buf.Bytes(), nil
}