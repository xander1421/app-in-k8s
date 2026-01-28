package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"

	"github.com/alexprut/twitter-clone/pkg/models"
)

const (
	IndexTweets = "tweets"
	IndexUsers  = "users"
)

type ElasticsearchClient struct {
	client *elasticsearch.Client
}

func NewElasticsearchClient(url string) (*ElasticsearchClient, error) {
	cfg := elasticsearch.Config{
		Addresses:     []string{url},
		RetryOnStatus: []int{502, 503, 504, 429},
		MaxRetries:    3,
	}

	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	ec := &ElasticsearchClient{client: client}

	if err := ec.ensureIndices(context.Background()); err != nil {
		return nil, fmt.Errorf("ensure indices: %w", err)
	}

	return ec, nil
}

func (ec *ElasticsearchClient) Health(ctx context.Context) error {
	res, err := ec.client.Cluster.Health(
		ec.client.Cluster.Health.WithContext(ctx),
		ec.client.Cluster.Health.WithTimeout(5*time.Second),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("health check failed: %s", res.Status())
	}
	return nil
}

func (ec *ElasticsearchClient) ensureIndices(ctx context.Context) error {
	// Tweet index mapping
	tweetMapping := `{
		"settings": {
			"number_of_shards": 3,
			"number_of_replicas": 1,
			"analysis": {
				"analyzer": {
					"tweet_analyzer": {
						"type": "custom",
						"tokenizer": "standard",
						"filter": ["lowercase", "asciifolding", "porter_stem"]
					}
				}
			}
		},
		"mappings": {
			"properties": {
				"id": {"type": "keyword"},
				"author_id": {"type": "keyword"},
				"content": {
					"type": "text",
					"analyzer": "tweet_analyzer",
					"fields": {
						"keyword": {"type": "keyword"}
					}
				},
				"hashtags": {"type": "keyword"},
				"mentions": {"type": "keyword"},
				"like_count": {"type": "integer"},
				"retweet_count": {"type": "integer"},
				"reply_count": {"type": "integer"},
				"created_at": {"type": "date"}
			}
		}
	}`

	// User index mapping
	userMapping := `{
		"settings": {
			"number_of_shards": 1,
			"number_of_replicas": 1,
			"analysis": {
				"analyzer": {
					"username_analyzer": {
						"type": "custom",
						"tokenizer": "standard",
						"filter": ["lowercase", "asciifolding"]
					}
				}
			}
		},
		"mappings": {
			"properties": {
				"id": {"type": "keyword"},
				"username": {
					"type": "text",
					"analyzer": "username_analyzer",
					"fields": {
						"keyword": {"type": "keyword"}
					}
				},
				"display_name": {
					"type": "text",
					"analyzer": "username_analyzer"
				},
				"bio": {"type": "text"},
				"follower_count": {"type": "integer"},
				"is_verified": {"type": "boolean"},
				"created_at": {"type": "date"}
			}
		}
	}`

	indices := map[string]string{
		IndexTweets: tweetMapping,
		IndexUsers:  userMapping,
	}

	for indexName, mapping := range indices {
		res, err := ec.client.Indices.Exists([]string{indexName})
		if err != nil {
			return err
		}
		res.Body.Close()

		if res.StatusCode == 200 {
			continue
		}

		res, err = ec.client.Indices.Create(
			indexName,
			ec.client.Indices.Create.WithBody(strings.NewReader(mapping)),
			ec.client.Indices.Create.WithContext(ctx),
		)
		if err != nil {
			return err
		}
		defer res.Body.Close()

		if res.IsError() {
			return fmt.Errorf("create index %s: %s", indexName, res.Status())
		}
	}

	return nil
}

// IndexTweet adds or updates a tweet in the search index
func (ec *ElasticsearchClient) IndexTweet(ctx context.Context, tweet *models.Tweet) error {
	// Extract hashtags and mentions
	hashtags := extractHashtags(tweet.Content)
	mentions := extractMentions(tweet.Content)

	doc := map[string]interface{}{
		"id":            tweet.ID,
		"author_id":     tweet.AuthorID,
		"content":       tweet.Content,
		"hashtags":      hashtags,
		"mentions":      mentions,
		"like_count":    tweet.LikeCount,
		"retweet_count": tweet.RetweetCount,
		"reply_count":   tweet.ReplyCount,
		"created_at":    tweet.CreatedAt,
	}

	data, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	req := esapi.IndexRequest{
		Index:      IndexTweets,
		DocumentID: tweet.ID,
		Body:       bytes.NewReader(data),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, ec.client)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("index tweet: %s", res.Status())
	}
	return nil
}

// IndexUser adds or updates a user in the search index
func (ec *ElasticsearchClient) IndexUser(ctx context.Context, user *models.User) error {
	doc := map[string]interface{}{
		"id":             user.ID,
		"username":       user.Username,
		"display_name":   user.DisplayName,
		"bio":            user.Bio,
		"follower_count": user.FollowerCount,
		"is_verified":    user.IsVerified,
		"created_at":     user.CreatedAt,
	}

	data, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	req := esapi.IndexRequest{
		Index:      IndexUsers,
		DocumentID: user.ID,
		Body:       bytes.NewReader(data),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, ec.client)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("index user: %s", res.Status())
	}
	return nil
}

// DeleteTweet removes a tweet from the search index
func (ec *ElasticsearchClient) DeleteTweet(ctx context.Context, tweetID string) error {
	req := esapi.DeleteRequest{
		Index:      IndexTweets,
		DocumentID: tweetID,
		Refresh:    "true",
	}

	res, err := req.Do(ctx, ec.client)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() && res.StatusCode != 404 {
		return fmt.Errorf("delete tweet: %s", res.Status())
	}
	return nil
}

// SearchTweets performs a full-text search on tweets
func (ec *ElasticsearchClient) SearchTweets(ctx context.Context, query string, limit, offset int) (*models.SearchResult, error) {
	start := time.Now()

	searchQuery := map[string]interface{}{
		"from": offset,
		"size": limit,
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":     query,
				"fields":    []string{"content^2", "hashtags^3"},
				"type":      "best_fields",
				"fuzziness": "AUTO",
			},
		},
		"sort": []map[string]interface{}{
			{"_score": "desc"},
			{"created_at": "desc"},
		},
	}

	data, err := json.Marshal(searchQuery)
	if err != nil {
		return nil, err
	}

	res, err := ec.client.Search(
		ec.client.Search.WithContext(ctx),
		ec.client.Search.WithIndex(IndexTweets),
		ec.client.Search.WithBody(bytes.NewReader(data)),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("search tweets: %s", res.Status())
	}

	var result struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source models.Tweet `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, err
	}

	tweets := make([]models.Tweet, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		tweets = append(tweets, hit.Source)
	}

	return &models.SearchResult{
		Tweets: tweets,
		Total:  result.Hits.Total.Value,
		TookMs: time.Since(start).Milliseconds(),
		Query:  query,
	}, nil
}

// SearchUsers performs a full-text search on users
func (ec *ElasticsearchClient) SearchUsers(ctx context.Context, query string, limit, offset int) (*models.SearchResult, error) {
	start := time.Now()

	searchQuery := map[string]interface{}{
		"from": offset,
		"size": limit,
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":     query,
				"fields":    []string{"username^3", "display_name^2", "bio"},
				"type":      "best_fields",
				"fuzziness": "AUTO",
			},
		},
		"sort": []map[string]interface{}{
			{"_score": "desc"},
			{"follower_count": "desc"},
		},
	}

	data, err := json.Marshal(searchQuery)
	if err != nil {
		return nil, err
	}

	res, err := ec.client.Search(
		ec.client.Search.WithContext(ctx),
		ec.client.Search.WithIndex(IndexUsers),
		ec.client.Search.WithBody(bytes.NewReader(data)),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("search users: %s", res.Status())
	}

	var result struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source models.User `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, err
	}

	users := make([]models.User, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		users = append(users, hit.Source)
	}

	return &models.SearchResult{
		Users:  users,
		Total:  result.Hits.Total.Value,
		TookMs: time.Since(start).Milliseconds(),
		Query:  query,
	}, nil
}

// GetTrending returns trending hashtags
func (ec *ElasticsearchClient) GetTrending(ctx context.Context, limit int) ([]models.TrendingTopic, error) {
	// Aggregate hashtags from last 24 hours
	searchQuery := map[string]interface{}{
		"size": 0,
		"query": map[string]interface{}{
			"range": map[string]interface{}{
				"created_at": map[string]interface{}{
					"gte": "now-24h",
				},
			},
		},
		"aggs": map[string]interface{}{
			"trending_hashtags": map[string]interface{}{
				"terms": map[string]interface{}{
					"field": "hashtags",
					"size":  limit,
				},
			},
		},
	}

	data, err := json.Marshal(searchQuery)
	if err != nil {
		return nil, err
	}

	res, err := ec.client.Search(
		ec.client.Search.WithContext(ctx),
		ec.client.Search.WithIndex(IndexTweets),
		ec.client.Search.WithBody(bytes.NewReader(data)),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("get trending: %s", res.Status())
	}

	var result struct {
		Aggregations struct {
			TrendingHashtags struct {
				Buckets []struct {
					Key      string `json:"key"`
					DocCount int64  `json:"doc_count"`
				} `json:"buckets"`
			} `json:"trending_hashtags"`
		} `json:"aggregations"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, err
	}

	trending := make([]models.TrendingTopic, 0, len(result.Aggregations.TrendingHashtags.Buckets))
	for i, bucket := range result.Aggregations.TrendingHashtags.Buckets {
		trending = append(trending, models.TrendingTopic{
			Tag:        bucket.Key,
			TweetCount: bucket.DocCount,
			Rank:       i + 1,
		})
	}

	return trending, nil
}

// Helper functions to extract hashtags and mentions
func extractHashtags(content string) []string {
	var hashtags []string
	words := strings.Fields(content)
	for _, word := range words {
		if strings.HasPrefix(word, "#") && len(word) > 1 {
			tag := strings.TrimPrefix(word, "#")
			tag = strings.Trim(tag, ".,!?;:")
			if len(tag) > 0 {
				hashtags = append(hashtags, strings.ToLower(tag))
			}
		}
	}
	return hashtags
}

func extractMentions(content string) []string {
	var mentions []string
	words := strings.Fields(content)
	for _, word := range words {
		if strings.HasPrefix(word, "@") && len(word) > 1 {
			mention := strings.TrimPrefix(word, "@")
			mention = strings.Trim(mention, ".,!?;:")
			if len(mention) > 0 {
				mentions = append(mentions, strings.ToLower(mention))
			}
		}
	}
	return mentions
}
