package engine

import (
	"context"
	"errors"
	"skyrix/internal/logger"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis is a generic Redis wrapper for cache/KV operations.
type Redis struct {
	client    *redis.Client
	logger    logger.Interface
	keyPrefix string
	statusTTL time.Duration
}

// RedisOpts contains configuration options for Redis service initialization.
type RedisOpts struct {
	KeyPrefix string        // Prefix for all Redis keys ()
	StatusTTL time.Duration // Default TTL for status-related keys (default: 10 minutes)
}

// NewRedisService creates a new Redis service instance.
// Key prefix is normalized (trailing colons removed) and defaults to "skyrix-delivery" if empty.
// StatusTTL defaults to 10 minutes if not specified or zero.
func NewRedisService(client *redis.Client, lg logger.Interface, redisOpts RedisOpts) *Redis {
	prefix := strings.TrimSuffix(strings.TrimSpace(redisOpts.KeyPrefix), ":")
	if prefix == "" {
		prefix = "skyrix-delivery"
	}
	ttl := redisOpts.StatusTTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &Redis{client: client, logger: lg, keyPrefix: prefix, statusTTL: ttl}
}

// Get retrieves raw bytes from Redis by key.
// Returns nil, false, nil on cache miss (not an error).
func (r *Redis) Get(ctx context.Context, key string) ([]byte, bool, error) {
	b, err := r.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		if r.logger != nil {
			r.logger.Warn("Get Error", "key", key, "err", err)
		}
		return nil, false, err
	}
	return b, true, nil
}

// Set stores raw bytes in Redis with the specified TTL.
//
// Semantics:
//
//		ttl > 0  -> use this TTL
//	    ttl is 0 -> key is persistent (no expiration).
//		ttl < 0  -> use default statusTTL
func (r *Redis) Set(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	if ttl < 0 {
		ttl = r.statusTTL
	}
	// ttl == 0 => no expiration
	return r.client.Set(ctx, key, data, ttl).Err()
}

// Del removes a key from Redis.
func (r *Redis) Del(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

// Exists checks if a key exists in Redis.
func (r *Redis) Exists(ctx context.Context, key string) (bool, error) {
	n, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// KeyPrefix returns the key prefix used by this Redis service instance.
func (r *Redis) KeyPrefix() string { return r.keyPrefix }

// ScanKeys scans Redis for keys matching a pattern using the SCAN command.
// Returns keys in batches along with the cursor for the next iteration (0 when done).
func (r *Redis) ScanKeys(ctx context.Context, pattern string, count int, cursor uint64) ([]string, uint64, error) {
	return r.client.Scan(ctx, cursor, pattern, int64(count)).Result()
}

// DelMany deletes multiple keys from Redis in a single transaction using pipeline.
// Returns immediately if keys slice is empty.
func (r *Redis) DelMany(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	pipe := r.client.TxPipeline()
	for _, k := range keys {
		pipe.Del(ctx, k)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// Close closes the underlying Redis client connection.
// Should be called during application shutdown. Safe to call multiple times.
func (r *Redis) Close() error {
	if r.client == nil {
		return nil
	}
	return r.client.Close()
}
