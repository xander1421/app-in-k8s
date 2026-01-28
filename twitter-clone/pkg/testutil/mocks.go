package testutil

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/alexprut/twitter-clone/pkg/models"
)

// MockCache implements a mock Redis cache for testing
type MockCache struct {
	mu        sync.RWMutex
	data      map[string]interface{}
	timelines map[string][]string
	counters  map[string]int64
}

func NewMockCache() *MockCache {
	return &MockCache{
		data:      make(map[string]interface{}),
		timelines: make(map[string][]string),
		counters:  make(map[string]int64),
	}
}

func (m *MockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func (m *MockCache) Get(ctx context.Context, key string, dest interface{}) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if v, ok := m.data[key]; ok {
		switch d := dest.(type) {
		case *models.User:
			if u, ok := v.(*models.User); ok {
				*d = *u
			}
		case *models.Tweet:
			if t, ok := v.(*models.Tweet); ok {
				*d = *t
			}
		}
		return nil
	}
	return fmt.Errorf("key not found: %s", key)
}

func (m *MockCache) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *MockCache) AddToTimeline(ctx context.Context, userID, tweetID string, score float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := "timeline:home:" + userID
	m.timelines[key] = append([]string{tweetID}, m.timelines[key]...)
	return nil
}

func (m *MockCache) AddToUserTimeline(ctx context.Context, userID, tweetID string, score float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := "timeline:user:" + userID
	m.timelines[key] = append([]string{tweetID}, m.timelines[key]...)
	return nil
}

func (m *MockCache) GetHomeTimeline(ctx context.Context, userID string, offset, limit int64) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := "timeline:home:" + userID
	tl := m.timelines[key]
	if int(offset) >= len(tl) {
		return nil, nil
	}
	end := int(offset + limit)
	if end > len(tl) {
		end = len(tl)
	}
	return tl[offset:end], nil
}

func (m *MockCache) GetUserTimeline(ctx context.Context, userID string, offset, limit int64) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := "timeline:user:" + userID
	tl := m.timelines[key]
	if int(offset) >= len(tl) {
		return nil, nil
	}
	end := int(offset + limit)
	if end > len(tl) {
		end = len(tl)
	}
	return tl[offset:end], nil
}

func (m *MockCache) IncrCounter(ctx context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[key]++
	return m.counters[key], nil
}

func (m *MockCache) DecrCounter(ctx context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[key]--
	return m.counters[key], nil
}

func (m *MockCache) GetCounter(ctx context.Context, key string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.counters[key], nil
}

func (m *MockCache) RemoveFromTimeline(ctx context.Context, key, tweetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tl := m.timelines[key]
	for i, id := range tl {
		if id == tweetID {
			m.timelines[key] = append(tl[:i], tl[i+1:]...)
			break
		}
	}
	return nil
}

func (m *MockCache) Health(ctx context.Context) error {
	return nil
}

func (m *MockCache) Close() error {
	return nil
}

// MockQueue implements a mock RabbitMQ queue for testing
type MockQueue struct {
	mu       sync.Mutex
	jobs     []models.FanoutJob
	handlers map[string]func(models.FanoutJob) error
}

func NewMockQueue() *MockQueue {
	return &MockQueue{
		jobs:     make([]models.FanoutJob, 0),
		handlers: make(map[string]func(models.FanoutJob) error),
	}
}

func (m *MockQueue) Publish(ctx context.Context, queueName string, job models.FanoutJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs = append(m.jobs, job)
	return nil
}

func (m *MockQueue) PublishFanout(ctx context.Context, tweetID, authorID string, followerCount int) error {
	return m.Publish(ctx, "twitter.fanout.normal", models.FanoutJob{
		Type:     "fanout",
		TweetID:  tweetID,
		AuthorID: authorID,
	})
}

func (m *MockQueue) PublishSearchIndex(ctx context.Context, tweetID, content string) error {
	return m.Publish(ctx, "twitter.search.index", models.FanoutJob{
		Type:    "index",
		TweetID: tweetID,
	})
}

func (m *MockQueue) PublishNotification(ctx context.Context, userID, notifType, actorID, tweetID string) error {
	return m.Publish(ctx, "twitter.notify.push", models.FanoutJob{
		Type:    "notify",
		TweetID: tweetID,
	})
}

func (m *MockQueue) GetJobs() []models.FanoutJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]models.FanoutJob{}, m.jobs...)
}

func (m *MockQueue) ClearJobs() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs = m.jobs[:0]
}

func (m *MockQueue) Health(ctx context.Context) error {
	return nil
}

func (m *MockQueue) Close() error {
	return nil
}
