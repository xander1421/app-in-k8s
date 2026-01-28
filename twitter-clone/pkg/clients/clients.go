package clients

import (
	"context"
	"fmt"
	"time"

	"github.com/alexprut/twitter-clone/pkg/models"
)

// HTTPClient wraps HTTP/3 client with common functionality
type HTTPClient struct {
	client  *HTTP3Client
	baseURL string
}

func NewHTTPClient(baseURL string, timeout time.Duration) *HTTPClient {
	return &HTTPClient{
		client:  NewHTTP3Client(baseURL, timeout),
		baseURL: baseURL,
	}
}

func (c *HTTPClient) do(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	switch method {
	case "GET":
		if result != nil {
			return c.client.GetJSON(ctx, path, result)
		}
		_, err := c.client.Get(ctx, path)
		return err
	case "POST":
		if result != nil {
			return c.client.PostJSON(ctx, path, body, result)
		}
		_, err := c.client.Post(ctx, path, body)
		return err
	case "PUT":
		_, err := c.client.Put(ctx, path, body)
		return err
	case "DELETE":
		_, err := c.client.Delete(ctx, path)
		return err
	default:
		return fmt.Errorf("unsupported method: %s", method)
	}
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

// NewUserClient creates a new user client (compatibility)
func NewUserClient(baseURL string) *UserServiceClient {
	return NewUserServiceClient(baseURL)
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

func (c *UserServiceClient) GetFollowing(ctx context.Context, userID string, limit, offset int) ([]*models.User, error) {
	var resp models.FollowersResponse
	path := fmt.Sprintf("/api/v1/users/%s/following?limit=%d&offset=%d", userID, limit, offset)
	if err := c.do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	
	// Convert to pointers for compatibility
	result := make([]*models.User, len(resp.Users))
	for i := range resp.Users {
		result[i] = &resp.Users[i]
	}
	return result, nil
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

// NewTweetClient creates a new tweet client (compatibility)
func NewTweetClient(baseURL string) *TweetServiceClient {
	return NewTweetServiceClient(baseURL)
}

// GetTweetsBatch fetches multiple tweets by IDs
func (c *TweetServiceClient) GetTweetsBatch(ctx context.Context, tweetIDs []string) ([]models.Tweet, error) {
	return c.GetTweets(ctx, tweetIDs)
}

// GetUserTweets gets tweets by a specific user
func (c *TweetServiceClient) GetUserTweets(ctx context.Context, userID string, limit, offset int) ([]models.Tweet, error) {
	path := fmt.Sprintf("/api/v1/users/%s/tweets?limit=%d&offset=%d", userID, limit, offset)
	var resp struct {
		Tweets []models.Tweet `json:"tweets"`
	}
	if err := c.do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Tweets, nil
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

// NewNotificationClient creates a notification service client
type NotificationServiceClient struct {
	*HTTPClient
}

func NewNotificationServiceClient(baseURL string) *NotificationServiceClient {
	return &NotificationServiceClient{
		HTTPClient: NewHTTPClient(baseURL, 10*time.Second),
	}
}

// NewNotificationClient creates a new notification client (compatibility)
func NewNotificationClient(baseURL string) *NotificationServiceClient {
	return NewNotificationServiceClient(baseURL)
}

func (c *NotificationServiceClient) SendNotification(ctx context.Context, notification *models.Notification) error {
	return c.do(ctx, "POST", "/api/v1/notifications", notification, nil)
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
