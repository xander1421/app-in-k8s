package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/alexprut/twitter-clone/pkg/models"
)

// HTTPClient wraps http.Client with common functionality
type HTTPClient struct {
	client  *http.Client
	baseURL string
}

func NewHTTPClient(baseURL string, timeout time.Duration) *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: timeout,
		},
		baseURL: baseURL,
	}
}

func (c *HTTPClient) do(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errResp models.ErrorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("request failed: %s", errResp.Error)
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// UserServiceClient is a client for the user service
type UserServiceClient struct {
	*HTTPClient
}

func NewUserServiceClient(baseURL string) *UserServiceClient {
	return &UserServiceClient{
		HTTPClient: NewHTTPClient(baseURL, 10*time.Second),
	}
}

func (c *UserServiceClient) GetUser(ctx context.Context, userID string) (*models.User, error) {
	var user models.User
	if err := c.do(ctx, "GET", "/api/v1/users/"+userID, nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *UserServiceClient) GetFollowers(ctx context.Context, userID string, limit, offset int) ([]models.User, error) {
	var resp models.FollowersResponse
	path := fmt.Sprintf("/api/v1/users/%s/followers?limit=%d&offset=%d", userID, limit, offset)
	if err := c.do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Users, nil
}

func (c *UserServiceClient) GetFollowing(ctx context.Context, userID string, limit, offset int) ([]models.User, error) {
	var resp models.FollowersResponse
	path := fmt.Sprintf("/api/v1/users/%s/following?limit=%d&offset=%d", userID, limit, offset)
	if err := c.do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Users, nil
}

func (c *UserServiceClient) GetFollowerIDs(ctx context.Context, userID string) ([]string, error) {
	var ids []string
	if err := c.do(ctx, "GET", "/api/v1/users/"+userID+"/follower-ids", nil, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

// TweetServiceClient is a client for the tweet service
type TweetServiceClient struct {
	*HTTPClient
}

func NewTweetServiceClient(baseURL string) *TweetServiceClient {
	return &TweetServiceClient{
		HTTPClient: NewHTTPClient(baseURL, 10*time.Second),
	}
}

func (c *TweetServiceClient) GetTweet(ctx context.Context, tweetID string) (*models.Tweet, error) {
	var tweet models.Tweet
	if err := c.do(ctx, "GET", "/api/v1/tweets/"+tweetID, nil, &tweet); err != nil {
		return nil, err
	}
	return &tweet, nil
}

func (c *TweetServiceClient) GetTweets(ctx context.Context, tweetIDs []string) ([]models.Tweet, error) {
	var tweets []models.Tweet
	body := map[string][]string{"ids": tweetIDs}
	if err := c.do(ctx, "POST", "/api/v1/tweets/batch", body, &tweets); err != nil {
		return nil, err
	}
	return tweets, nil
}

// TimelineServiceClient is a client for the timeline service
type TimelineServiceClient struct {
	*HTTPClient
}

func NewTimelineServiceClient(baseURL string) *TimelineServiceClient {
	return &TimelineServiceClient{
		HTTPClient: NewHTTPClient(baseURL, 10*time.Second),
	}
}

func (c *TimelineServiceClient) AddToTimeline(ctx context.Context, userID, tweetID string, score float64) error {
	body := map[string]interface{}{
		"user_id":  userID,
		"tweet_id": tweetID,
		"score":    score,
	}
	return c.do(ctx, "POST", "/api/v1/timeline/add", body, nil)
}

// MediaServiceClient is a client for the media service
type MediaServiceClient struct {
	*HTTPClient
}

func NewMediaServiceClient(baseURL string) *MediaServiceClient {
	return &MediaServiceClient{
		HTTPClient: NewHTTPClient(baseURL, 30*time.Second),
	}
}

func (c *MediaServiceClient) GetMedia(ctx context.Context, mediaID string) (*models.Media, error) {
	var media models.Media
	if err := c.do(ctx, "GET", "/api/v1/media/"+mediaID, nil, &media); err != nil {
		return nil, err
	}
	return &media, nil
}

func (c *MediaServiceClient) GetUploadURL(ctx context.Context, contentType string) (string, string, error) {
	body := map[string]string{"content_type": contentType}
	var resp struct {
		UploadURL string `json:"upload_url"`
		MediaID   string `json:"media_id"`
	}
	if err := c.do(ctx, "POST", "/api/v1/media/presign", body, &resp); err != nil {
		return "", "", err
	}
	return resp.UploadURL, resp.MediaID, nil
}
