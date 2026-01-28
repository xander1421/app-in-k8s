package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// Key prefixes
	PrefixTimeline    = "timeline:"
	PrefixUserProfile = "user:profile:"
	PrefixFollowers   = "count:followers:"
	PrefixFollowing   = "count:following:"
	PrefixLikes       = "count:likes:"
	PrefixRetweets    = "count:retweets:"
	PrefixRateLimit   = "ratelimit:"
	PrefixLock        = "lock:"

	// Pub/Sub channels
	ChannelTweetEvents = "twitter:tweets"
	ChannelUserEvents  = "twitter:users"
)

type RedisCache struct {
	client     redis.UniversalClient
	instanceID string
}

// NewRedisCache creates a Redis client with Sentinel support for HA
func NewRedisCache(ctx context.Context, sentinelAddrs []string, masterName, password, instanceID string) (*RedisCache, error) {
	client := redis.NewFailoverClient(&redis.FailoverOptions{
		MasterName:      masterName,
		SentinelAddrs:   sentinelAddrs,
		Password:        password,
		DB:              0,
		DialTimeout:     5 * time.Second,
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    3 * time.Second,
		PoolSize:        20,
		MinIdleConns:    5,
		ConnMaxLifetime: 30 * time.Minute,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return &RedisCache{
		client:     client,
		instanceID: instanceID,
	}, nil
}

// NewRedisCacheSimple creates a simple Redis client (non-sentinel)
func NewRedisCacheSimple(ctx context.Context, addr, password, instanceID string) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:            addr,
		Password:        password,
		DB:              0,
		DialTimeout:     5 * time.Second,
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    3 * time.Second,
		PoolSize:        20,
		MinIdleConns:    5,
		ConnMaxLifetime: 30 * time.Minute,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return &RedisCache{
		client:     client,
		instanceID: instanceID,
	}, nil
}

func (rc *RedisCache) Close() error {
	return rc.client.Close()
}

func (rc *RedisCache) Health(ctx context.Context) error {
	return rc.client.Ping(ctx).Err()
}

func (rc *RedisCache) Client() redis.UniversalClient {
	return rc.client
}

// ============== Timeline Operations ==============

// AddToTimeline adds a tweet to a user's home timeline
func (rc *RedisCache) AddToTimeline(ctx context.Context, userID, tweetID string, score float64) error {
	key := PrefixTimeline + "home:" + userID
	return rc.client.ZAdd(ctx, key, redis.Z{Score: score, Member: tweetID}).Err()
}

// AddToUserTimeline adds a tweet to a user's own tweets timeline
func (rc *RedisCache) AddToUserTimeline(ctx context.Context, userID, tweetID string, score float64) error {
	key := PrefixTimeline + "user:" + userID
	return rc.client.ZAdd(ctx, key, redis.Z{Score: score, Member: tweetID}).Err()
}

// GetTimeline retrieves tweet IDs from a timeline (newest first)
func (rc *RedisCache) GetTimeline(ctx context.Context, key string, offset, limit int64) ([]string, error) {
	return rc.client.ZRevRange(ctx, key, offset, offset+limit-1).Result()
}

// GetHomeTimeline retrieves a user's home timeline
func (rc *RedisCache) GetHomeTimeline(ctx context.Context, userID string, offset, limit int64) ([]string, error) {
	return rc.GetTimeline(ctx, PrefixTimeline+"home:"+userID, offset, limit)
}

// GetUserTimeline retrieves a user's own tweets
func (rc *RedisCache) GetUserTimeline(ctx context.Context, userID string, offset, limit int64) ([]string, error) {
	return rc.GetTimeline(ctx, PrefixTimeline+"user:"+userID, offset, limit)
}

// TrimTimeline keeps only the most recent N entries
func (rc *RedisCache) TrimTimeline(ctx context.Context, key string, maxSize int64) error {
	return rc.client.ZRemRangeByRank(ctx, key, 0, -maxSize-1).Err()
}

// RemoveFromTimeline removes a tweet from a timeline
func (rc *RedisCache) RemoveFromTimeline(ctx context.Context, key, tweetID string) error {
	return rc.client.ZRem(ctx, key, tweetID).Err()
}

// ============== Counter Operations ==============

// IncrCounter increments a counter
func (rc *RedisCache) IncrCounter(ctx context.Context, key string) (int64, error) {
	return rc.client.Incr(ctx, key).Result()
}

// DecrCounter decrements a counter
func (rc *RedisCache) DecrCounter(ctx context.Context, key string) (int64, error) {
	return rc.client.Decr(ctx, key).Result()
}

// GetCounter gets a counter value
func (rc *RedisCache) GetCounter(ctx context.Context, key string) (int64, error) {
	val, err := rc.client.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// SetCounter sets a counter value
func (rc *RedisCache) SetCounter(ctx context.Context, key string, value int64) error {
	return rc.client.Set(ctx, key, value, 0).Err()
}

// ============== Caching ==============

// Set stores a value with TTL
func (rc *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return rc.client.Set(ctx, key, data, ttl).Err()
}

// Get retrieves a value
func (rc *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	data, err := rc.client.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// Delete removes a key
func (rc *RedisCache) Delete(ctx context.Context, key string) error {
	return rc.client.Del(ctx, key).Err()
}

// Exists checks if a key exists
func (rc *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	n, err := rc.client.Exists(ctx, key).Result()
	return n > 0, err
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
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`
	return rc.client.Eval(ctx, script, []string{PrefixLock + key}, rc.instanceID).Err()
}

// ============== Pub/Sub ==============

func (rc *RedisCache) Publish(ctx context.Context, channel string, message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return rc.client.Publish(ctx, channel, data).Err()
}

func (rc *RedisCache) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return rc.client.Subscribe(ctx, channels...)
}

// ============== Set Operations (for followers/following) ==============

func (rc *RedisCache) SAdd(ctx context.Context, key string, members ...interface{}) error {
	return rc.client.SAdd(ctx, key, members...).Err()
}

func (rc *RedisCache) SRem(ctx context.Context, key string, members ...interface{}) error {
	return rc.client.SRem(ctx, key, members...).Err()
}

func (rc *RedisCache) SMembers(ctx context.Context, key string) ([]string, error) {
	return rc.client.SMembers(ctx, key).Result()
}

func (rc *RedisCache) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	return rc.client.SIsMember(ctx, key, member).Result()
}

func (rc *RedisCache) SCard(ctx context.Context, key string) (int64, error) {
	return rc.client.SCard(ctx, key).Result()
}

// ============== List Operations ==============

func (rc *RedisCache) LPush(ctx context.Context, key string, values ...interface{}) error {
	return rc.client.LPush(ctx, key, values...).Err()
}

func (rc *RedisCache) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return rc.client.LRange(ctx, key, start, stop).Result()
}

// ============== Additional Helper Methods ==============

func (rc *RedisCache) Keys(ctx context.Context, pattern string) ([]string, error) {
	return rc.client.Keys(ctx, pattern).Result()
}

// AddToSet is an alias for SAdd for compatibility
func (rc *RedisCache) AddToSet(ctx context.Context, key string, members ...interface{}) error {
	return rc.SAdd(ctx, key, members...)
}

// GetSetMembers is an alias for SMembers for compatibility  
func (rc *RedisCache) GetSetMembers(ctx context.Context, key string) ([]string, error) {
	return rc.SMembers(ctx, key)
}

// AddToList is an alias for LPush for compatibility
func (rc *RedisCache) AddToList(ctx context.Context, key string, values ...interface{}) error {
	return rc.LPush(ctx, key, values...)
}

// GetList returns list elements
func (rc *RedisCache) GetList(ctx context.Context, key string) ([]string, error) {
	return rc.LRange(ctx, key, 0, -1)
}
