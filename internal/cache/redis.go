package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/alexprut/fileshare/internal/models"
)

const (
	// Pub/Sub channels
	ChannelFileEvents = "fileshare:events"
	ChannelHeartbeat  = "fileshare:heartbeat"

	// Key prefixes
	PrefixSession     = "session:"
	PrefixRateLimit   = "ratelimit:"
	PrefixLock        = "lock:"
	PrefixActiveNodes = "nodes:"
)

type RedisCache struct {
	client     redis.UniversalClient
	instanceID string
	pubsub     *redis.PubSub

	// Cluster awareness
	activePeers   map[string]time.Time
	peersMu       sync.RWMutex
	eventHandlers []func(models.FileEvent)
	handlersMu    sync.RWMutex
}

// NewRedisCache creates a Redis client with Sentinel support for HA
func NewRedisCache(ctx context.Context, sentinelAddrs []string, masterName, password, instanceID string) (*RedisCache, error) {
	// Use Sentinel for HA Redis (what Spotahome Redis Operator provides)
	client := redis.NewFailoverClient(&redis.FailoverOptions{
		MasterName:       masterName,
		SentinelAddrs:    sentinelAddrs,
		Password:         password,
		DB:               0,
		DialTimeout:      5 * time.Second,
		ReadTimeout:      3 * time.Second,
		WriteTimeout:     3 * time.Second,
		PoolSize:         20,
		MinIdleConns:     5,
		ConnMaxLifetime:  30 * time.Minute,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	rc := &RedisCache{
		client:      client,
		instanceID:  instanceID,
		activePeers: make(map[string]time.Time),
	}

	// Subscribe to cluster events
	rc.pubsub = client.Subscribe(ctx, ChannelFileEvents, ChannelHeartbeat)

	// Start background goroutines
	go rc.listenPubSub(ctx)
	go rc.sendHeartbeats(ctx)
	go rc.cleanupStaleNodes(ctx)

	return rc, nil
}

func (rc *RedisCache) Close() error {
	if rc.pubsub != nil {
		rc.pubsub.Close()
	}
	return rc.client.Close()
}

func (rc *RedisCache) Health(ctx context.Context) error {
	return rc.client.Ping(ctx).Err()
}

// ============== Pub/Sub for Cluster Awareness ==============

// PublishFileEvent broadcasts a file event to all instances
func (rc *RedisCache) PublishFileEvent(ctx context.Context, event models.FileEvent) error {
	event.InstanceID = rc.instanceID
	event.Timestamp = time.Now()

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return rc.client.Publish(ctx, ChannelFileEvents, data).Err()
}

// OnFileEvent registers a handler for file events from all instances
func (rc *RedisCache) OnFileEvent(handler func(models.FileEvent)) {
	rc.handlersMu.Lock()
	rc.eventHandlers = append(rc.eventHandlers, handler)
	rc.handlersMu.Unlock()
}

func (rc *RedisCache) listenPubSub(ctx context.Context) {
	ch := rc.pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			if msg == nil {
				continue
			}
			switch msg.Channel {
			case ChannelFileEvents:
				var event models.FileEvent
				if err := json.Unmarshal([]byte(msg.Payload), &event); err == nil {
					rc.handlersMu.RLock()
					for _, handler := range rc.eventHandlers {
						go handler(event)
					}
					rc.handlersMu.RUnlock()
				}
			case ChannelHeartbeat:
				var hb struct {
					InstanceID string    `json:"instance_id"`
					Timestamp  time.Time `json:"timestamp"`
				}
				if err := json.Unmarshal([]byte(msg.Payload), &hb); err == nil {
					rc.peersMu.Lock()
					rc.activePeers[hb.InstanceID] = hb.Timestamp
					rc.peersMu.Unlock()
				}
			}
		}
	}
}

func (rc *RedisCache) sendHeartbeats(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hb, _ := json.Marshal(map[string]interface{}{
				"instance_id": rc.instanceID,
				"timestamp":   time.Now(),
			})
			rc.client.Publish(ctx, ChannelHeartbeat, hb)
		}
	}
}

func (rc *RedisCache) cleanupStaleNodes(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rc.peersMu.Lock()
			threshold := time.Now().Add(-30 * time.Second)
			for id, lastSeen := range rc.activePeers {
				if lastSeen.Before(threshold) {
					delete(rc.activePeers, id)
				}
			}
			rc.peersMu.Unlock()
		}
	}
}

// GetActivePeers returns list of active cluster nodes
func (rc *RedisCache) GetActivePeers() []string {
	rc.peersMu.RLock()
	defer rc.peersMu.RUnlock()

	peers := make([]string, 0, len(rc.activePeers))
	for id := range rc.activePeers {
		peers = append(peers, id)
	}
	return peers
}

// ============== Session Management ==============

func (rc *RedisCache) SetSession(ctx context.Context, sessionID string, data interface{}, ttl time.Duration) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return rc.client.Set(ctx, PrefixSession+sessionID, jsonData, ttl).Err()
}

func (rc *RedisCache) GetSession(ctx context.Context, sessionID string, dest interface{}) error {
	data, err := rc.client.Get(ctx, PrefixSession+sessionID).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func (rc *RedisCache) DeleteSession(ctx context.Context, sessionID string) error {
	return rc.client.Del(ctx, PrefixSession+sessionID).Err()
}

// ============== Rate Limiting ==============

func (rc *RedisCache) CheckRateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, int, error) {
	fullKey := PrefixRateLimit + key

	pipe := rc.client.Pipeline()
	incr := pipe.Incr(ctx, fullKey)
	pipe.Expire(ctx, fullKey, window)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, err
	}

	count := int(incr.Val())
	return count <= limit, limit - count, nil
}

// ============== Distributed Locking ==============

func (rc *RedisCache) AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return rc.client.SetNX(ctx, PrefixLock+key, rc.instanceID, ttl).Result()
}

func (rc *RedisCache) ReleaseLock(ctx context.Context, key string) error {
	// Only release if we own the lock
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`
	return rc.client.Eval(ctx, script, []string{PrefixLock + key}, rc.instanceID).Err()
}

// ============== Caching ==============

func (rc *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return rc.client.Set(ctx, key, data, ttl).Err()
}

func (rc *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	data, err := rc.client.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func (rc *RedisCache) Delete(ctx context.Context, key string) error {
	return rc.client.Del(ctx, key).Err()
}
