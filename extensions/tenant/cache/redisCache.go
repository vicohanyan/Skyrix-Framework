// Package cache is the Redis (L2) cache of tenants, looked up by namespace
// and by domain. Keys live in the tenant namespace of the application
// keyspace: <prefix>:tenant:namespace:<namespace> and
// <prefix>:tenant:domain:<domain>, each holding the tenant as JSON.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	engineredis "gitlab.com/skyrix-lib/framework/engine/redis"
	"gitlab.com/skyrix-lib/framework/extensions/tenant/entity"

	"github.com/redis/go-redis/v9"
)

// Namespace is the tenant cache namespace inside the application keyspace.
const Namespace = "tenant"

// DefaultTTL is used when Options.TTL is not positive.
const DefaultTTL = 3 * time.Minute

// Options configures the cache.
type Options struct {
	TTL time.Duration
}

// RedisCache stores tenants in Redis.
type RedisCache struct {
	client *redis.Client
	keys   engineredis.Keyspace
	ttl    time.Duration
}

// NewRedisCache returns a cache under root.Sub(Namespace).
func NewRedisCache(client *redis.Client, root engineredis.Keyspace, opts Options) (*RedisCache, error) {
	if client == nil {
		return nil, errors.New("tenant cache: redis client is nil")
	}
	if root.IsZero() {
		return nil, fmt.Errorf("tenant cache: %w", engineredis.ErrEmptyKeyPrefix)
	}
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &RedisCache{client: client, keys: root.Sub(Namespace), ttl: ttl}, nil
}

func norm(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func (c *RedisCache) namespaceKey(namespace string) string {
	return c.keys.Key("namespace", norm(namespace))
}

func (c *RedisCache) domainKey(domain string) string {
	return c.keys.Key("domain", norm(domain))
}

// ByNamespace returns the cached tenant. ok is false on a cache miss.
func (c *RedisCache) ByNamespace(ctx context.Context, namespace string) (*entity.Tenant, bool, error) {
	return c.get(ctx, c.namespaceKey(namespace))
}

// ByDomain returns the cached tenant. ok is false on a cache miss.
func (c *RedisCache) ByDomain(ctx context.Context, domain string) (*entity.Tenant, bool, error) {
	return c.get(ctx, c.domainKey(domain))
}

// Put caches the tenant under its namespace and, when set, its domain.
func (c *RedisCache) Put(ctx context.Context, t *entity.Tenant) error {
	if t == nil {
		return nil
	}
	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("encode tenant: %w", err)
	}
	var errs []error
	if ns := norm(t.Namespace); ns != "" {
		if err := c.client.Set(ctx, c.namespaceKey(ns), data, c.ttl).Err(); err != nil {
			errs = append(errs, fmt.Errorf("set by namespace %q: %w", ns, err))
		}
	}
	if t.Domain != nil {
		if d := norm(*t.Domain); d != "" {
			if err := c.client.Set(ctx, c.domainKey(d), data, c.ttl).Err(); err != nil {
				errs = append(errs, fmt.Errorf("set by domain %q: %w", d, err))
			}
		}
	}
	return errors.Join(errs...)
}

func (c *RedisCache) get(ctx context.Context, key string) (*entity.Tenant, bool, error) {
	data, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if len(data) == 0 {
		return nil, false, nil
	}
	var t entity.Tenant
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, false, fmt.Errorf("decode tenant %s: %w", key, err)
	}
	return &t, true, nil
}
