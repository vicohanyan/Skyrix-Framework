package cache_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	engineredis "gitlab.com/skyrix-lib/framework/engine/redis"
	"gitlab.com/skyrix-lib/framework/extensions/tenant/cache"
	"gitlab.com/skyrix-lib/framework/extensions/tenant/entity"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newCache(t *testing.T, opts cache.Options) (*cache.RedisCache, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	root, err := engineredis.NewKeyspace("app")
	if err != nil {
		t.Fatal(err)
	}
	c, err := cache.NewRedisCache(client, root, opts)
	if err != nil {
		t.Fatalf("NewRedisCache() error = %v", err)
	}
	return c, server
}

func strPtr(s string) *string { return &s }

func TestPutAndGetUseTenantNamespaceKeys(t *testing.T) {
	c, server := newCache(t, cache.Options{TTL: time.Minute})
	ctx := context.Background()
	tenant := &entity.Tenant{Namespace: " Acme ", Domain: strPtr("Acme.Example.com"), Schema: strPtr("acme"), IsActive: true}

	if err := c.Put(ctx, tenant); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	// Key format is unchanged from the previous TenantService:
	// <prefix>:tenant:namespace:<ns> and <prefix>:tenant:domain:<domain>.
	for _, key := range []string{"app:tenant:namespace:acme", "app:tenant:domain:acme.example.com"} {
		if !server.Exists(key) {
			t.Fatalf("key %q not written; keys = %v", key, server.Keys())
		}
		if ttl := server.TTL(key); ttl != time.Minute {
			t.Errorf("TTL(%q) = %v, want 1m", key, ttl)
		}
		raw, _ := server.Get(key)
		var stored entity.Tenant
		if err := json.Unmarshal([]byte(raw), &stored); err != nil || stored.Namespace != tenant.Namespace {
			t.Errorf("stored value %q is not the tenant JSON: %v", raw, err)
		}
	}

	got, ok, err := c.ByNamespace(ctx, "ACME")
	if err != nil || !ok || got.Namespace != tenant.Namespace {
		t.Fatalf("ByNamespace() = %+v, %v, %v", got, ok, err)
	}
	got, ok, err = c.ByDomain(ctx, " acme.example.com ")
	if err != nil || !ok || *got.Schema != "acme" {
		t.Fatalf("ByDomain() = %+v, %v, %v", got, ok, err)
	}
}

func TestMissAndDefaultTTL(t *testing.T) {
	c, server := newCache(t, cache.Options{})
	ctx := context.Background()

	if got, ok, err := c.ByNamespace(ctx, "missing"); err != nil || ok || got != nil {
		t.Fatalf("ByNamespace() on miss = %v, %v, %v; want nil, false, nil", got, ok, err)
	}
	if err := c.Put(ctx, &entity.Tenant{Namespace: "ns"}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if ttl := server.TTL("app:tenant:namespace:ns"); ttl != cache.DefaultTTL {
		t.Fatalf("TTL = %v, want %v", ttl, cache.DefaultTTL)
	}
	if len(server.Keys()) != 1 {
		t.Fatalf("keys = %v, want only the namespace key for a tenant without domain", server.Keys())
	}
	if err := c.Put(ctx, nil); err != nil {
		t.Fatalf("Put(nil) error = %v", err)
	}
}

func TestDecodeError(t *testing.T) {
	c, server := newCache(t, cache.Options{})
	server.Set("app:tenant:domain:broken", "{not json")
	if _, ok, err := c.ByDomain(context.Background(), "broken"); err == nil || ok {
		t.Fatalf("ByDomain() = %v, %v; want decode error", ok, err)
	}
}

func TestNewRedisCacheValidates(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:0"})
	defer client.Close()
	if _, err := cache.NewRedisCache(client, engineredis.Keyspace{}, cache.Options{}); !errors.Is(err, engineredis.ErrEmptyKeyPrefix) {
		t.Fatalf("NewRedisCache(zero keyspace) error = %v, want ErrEmptyKeyPrefix", err)
	}
	root, _ := engineredis.NewKeyspace("app")
	if _, err := cache.NewRedisCache(nil, root, cache.Options{}); err == nil {
		t.Fatal("NewRedisCache(nil client) error = nil")
	}
}
