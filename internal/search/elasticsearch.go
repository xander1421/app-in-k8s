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

	"github.com/alexprut/fileshare/internal/models"
)

const (
	IndexFiles = "fileshare-files"
)

type ElasticsearchClient struct {
	client *elasticsearch.Client
}

func NewElasticsearchClient(url string) (*ElasticsearchClient, error) {
	cfg := elasticsearch.Config{
		Addresses: []string{url},
		// Retry on connection failure
		RetryOnStatus: []int{502, 503, 504, 429},
		MaxRetries:    3,
	}

	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	ec := &ElasticsearchClient{client: client}

	// Create index with mapping
	if err := ec.ensureIndex(context.Background()); err != nil {
		return nil, fmt.Errorf("ensure index: %w", err)
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

func (ec *ElasticsearchClient) ensureIndex(ctx context.Context) error {
	// Check if index exists
	res, err := ec.client.Indices.Exists([]string{IndexFiles})
	if err != nil {
		return err
	}
	res.Body.Close()

	if res.StatusCode == 200 {
		return nil // Index exists
	}

	// Create index with mapping
	mapping := `{
		"settings": {
			"number_of_shards": 3,
			"number_of_replicas": 1,
			"analysis": {
				"analyzer": {
					"filename_analyzer": {
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
				"name": {
					"type": "text",
					"analyzer": "filename_analyzer",
					"fields": {
						"keyword": {"type": "keyword"}
					}
				},
				"content_type": {"type": "keyword"},
				"size": {"type": "long"},
				"owner_id": {"type": "keyword"},
				"tags": {"type": "keyword"},
				"created_at": {"type": "date"},
				"updated_at": {"type": "date"}
			}
		}
	}`

	res, err = ec.client.Indices.Create(
		IndexFiles,
		ec.client.Indices.Create.WithBody(strings.NewReader(mapping)),
		ec.client.Indices.Create.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("create index: %s", res.Status())
	}
	return nil
}

// IndexFile adds or updates a file in the search index
func (ec *ElasticsearchClient) IndexFile(ctx context.Context, file *models.File) error {
	doc := map[string]interface{}{
		"id":           file.ID,
		"name":         file.Name,
		"content_type": file.ContentType,
		"size":         file.Size,
		"owner_id":     file.OwnerID,
		"tags":         file.Tags,
		"created_at":   file.CreatedAt,
		"updated_at":   file.UpdatedAt,
	}

	data, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	req := esapi.IndexRequest{
		Index:      IndexFiles,
		DocumentID: file.ID,
		Body:       bytes.NewReader(data),
		Refresh:    "true", // Make immediately searchable
	}

	res, err := req.Do(ctx, ec.client)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("index document: %s", res.Status())
	}
	return nil
}

// DeleteFile removes a file from the search index
func (ec *ElasticsearchClient) DeleteFile(ctx context.Context, fileID string) error {
	req := esapi.DeleteRequest{
		Index:      IndexFiles,
		DocumentID: fileID,
		Refresh:    "true",
	}

	res, err := req.Do(ctx, ec.client)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// 404 is OK - document might not exist
	if res.IsError() && res.StatusCode != 404 {
		return fmt.Errorf("delete document: %s", res.Status())
	}
	return nil
}

// Search performs a full-text search on files
func (ec *ElasticsearchClient) Search(ctx context.Context, query, ownerID string, limit, offset int) (*models.SearchResult, error) {
	start := time.Now()

	// Build query
	searchQuery := map[string]interface{}{
		"from": offset,
		"size": limit,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"multi_match": map[string]interface{}{
							"query":  query,
							"fields": []string{"name^3", "tags^2", "content_type"},
							"type":   "best_fields",
							"fuzziness": "AUTO",
						},
					},
				},
				"filter": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"owner_id": ownerID,
						},
					},
				},
			},
		},
		"sort": []map[string]interface{}{
			{"_score": "desc"},
			{"created_at": "desc"},
		},
		"highlight": map[string]interface{}{
			"fields": map[string]interface{}{
				"name": map[string]interface{}{},
			},
		},
	}

	data, err := json.Marshal(searchQuery)
	if err != nil {
		return nil, err
	}

	res, err := ec.client.Search(
		ec.client.Search.WithContext(ctx),
		ec.client.Search.WithIndex(IndexFiles),
		ec.client.Search.WithBody(bytes.NewReader(data)),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("search: %s", res.Status())
	}

	// Parse response
	var result struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source models.File `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, err
	}

	files := make([]models.File, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		files = append(files, hit.Source)
	}

	return &models.SearchResult{
		Files:  files,
		Total:  result.Hits.Total.Value,
		TookMs: time.Since(start).Milliseconds(),
		Query:  query,
	}, nil
}

// SearchByTags finds files with specific tags
func (ec *ElasticsearchClient) SearchByTags(ctx context.Context, tags []string, ownerID string, limit int) ([]models.File, error) {
	query := map[string]interface{}{
		"size": limit,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"terms": map[string]interface{}{
							"tags": tags,
						},
					},
				},
				"filter": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"owner_id": ownerID,
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	res, err := ec.client.Search(
		ec.client.Search.WithContext(ctx),
		ec.client.Search.WithIndex(IndexFiles),
		ec.client.Search.WithBody(bytes.NewReader(data)),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var result struct {
		Hits struct {
			Hits []struct {
				Source models.File `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, err
	}

	files := make([]models.File, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		files = append(files, hit.Source)
	}

	return files, nil
}
