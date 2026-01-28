package service

import (
	"context"
	"fmt"

	"github.com/alexprut/twitter-clone/pkg/models"
	"github.com/alexprut/twitter-clone/pkg/search"
)

type SearchService struct {
	es *search.ElasticsearchClient
}

func NewSearchService(es *search.ElasticsearchClient) *SearchService {
	return &SearchService{es: es}
}

func (s *SearchService) SearchTweets(ctx context.Context, query string, limit, offset int) (*models.SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	result, err := s.es.SearchTweets(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("search tweets: %w", err)
	}

	return result, nil
}

func (s *SearchService) SearchUsers(ctx context.Context, query string, limit, offset int) (*models.SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	result, err := s.es.SearchUsers(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}

	return result, nil
}

func (s *SearchService) GetTrending(ctx context.Context, limit int) ([]models.TrendingTopic, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	trending, err := s.es.GetTrending(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("get trending: %w", err)
	}

	return trending, nil
}

func (s *SearchService) IndexTweet(ctx context.Context, tweet *models.Tweet) error {
	return s.es.IndexTweet(ctx, tweet)
}

func (s *SearchService) IndexUser(ctx context.Context, user *models.User) error {
	return s.es.IndexUser(ctx, user)
}

func (s *SearchService) DeleteTweet(ctx context.Context, tweetID string) error {
	return s.es.DeleteTweet(ctx, tweetID)
}
