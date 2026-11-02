package storage_test

import (
	"context"
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	engineredis "gitlab.com/skyrix-lib/framework/engine/redis"
	"gitlab.com/skyrix-lib/framework/extensions/auth/contracts"
	"gitlab.com/skyrix-lib/framework/extensions/auth/storage"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setup(t *testing.T) (*redis.Client, engineredis.Keyspace, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	root, err := engineredis.NewKeyspace("svc")
	if err != nil {
		t.Fatal(err)
	}
	return client, root, server
}

func TestConstructorsRejectMissingDeps(t *testing.T) {
	client, root, _ := setup(t)
	constructors := map[string]func(*redis.Client, engineredis.Keyspace) error{
		"passport": func(c *redis.Client, k engineredis.Keyspace) error {
			_, err := storage.NewPassportStore(c, k, contracts.PassportOpts{})
			return err
		},
		"blacklist": func(c *redis.Client, k engineredis.Keyspace) error {
			_, err := storage.NewTokenBlacklist(c, k)
			return err
		},
		"session": func(c *redis.Client, k engineredis.Keyspace) error {
			_, err := storage.NewSessionStore(c, k)
			return err
		},
		"refresh": func(c *redis.Client, k engineredis.Keyspace) error {
			_, err := storage.NewRefreshTokenStore(c, k)
			return err
		},
	}
	for name, construct := range constructors {
		if err := construct(client, engineredis.Keyspace{}); !errors.Is(err, engineredis.ErrEmptyKeyPrefix) {
			t.Errorf("%s: zero keyspace error = %v, want ErrEmptyKeyPrefix", name, err)
		}
		if err := construct(nil, root); err == nil {
			t.Errorf("%s: nil client error = nil", name)
		}
		if err := construct(client, root); err != nil {
			t.Errorf("%s: error = %v", name, err)
		}
	}
}

func TestPassportStore(t *testing.T) {
	client, root, server := setup(t)
	store, err := storage.NewPassportStore(client, root, contracts.PassportOpts{StatusTTL: 5 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	updated := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	passport := &contracts.Passport{Namespace: "acme", Domain: "acme.example.com", Schema: "acme", IsActive: true, UpdatedAt: updated}

	if err := store.SetPassport(ctx, contracts.ScopeNamespace, " ACME ", passport); err != nil {
		t.Fatalf("SetPassport() error = %v", err)
	}
	if !server.Exists("svc:sub:namespace:acme") {
		t.Fatalf("passport key missing; keys = %v", server.Keys())
	}
	if ttl := server.TTL("svc:sub:namespace:acme"); ttl != 5*time.Minute {
		t.Fatalf("TTL = %v, want 5m", ttl)
	}
	if v := server.HGet("svc:sub:namespace:acme", "isActive"); v != "1" {
		t.Fatalf("isActive field = %q, want 1", v)
	}

	got, err := store.GetPassport(ctx, contracts.ScopeNamespace, "acme")
	if err != nil || got == nil || got.Schema != "acme" || !got.IsActive || !got.UpdatedAt.Equal(updated) {
		t.Fatalf("GetPassport() = %+v, %v", got, err)
	}

	if err := store.SetPassport(ctx, contracts.ScopeDomain, "acme.example.com", passport, time.Minute); err != nil {
		t.Fatal(err)
	}
	if ttl := server.TTL("svc:sub:domain:acme.example.com"); ttl != time.Minute {
		t.Fatalf("override TTL = %v, want 1m", ttl)
	}

	activeTo := time.Now().Add(-time.Hour)
	expired := &contracts.Passport{Namespace: "old", Domain: "old.example.com", ActiveTo: &activeTo}
	if err := store.SetPassportBoth(ctx, "old", "old.example.com", expired); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"svc:sub:namespace:old", "svc:sub:domain:old.example.com"} {
		if ttl := server.TTL(key); ttl != time.Minute {
			t.Errorf("TTL(%s) for passport past activeTo = %v, want 1m", key, ttl)
		}
	}

	if err := store.InvalidatePassport(ctx, contracts.ScopeNamespace, "acme"); err != nil {
		t.Fatal(err)
	}
	if got, err := store.GetPassport(ctx, contracts.ScopeNamespace, "acme"); err != nil || got != nil {
		t.Fatalf("GetPassport() after invalidate = %+v, %v; want nil", got, err)
	}
	if err := store.SetPassport(ctx, contracts.ScopeNamespace, "x", nil); err == nil {
		t.Fatal("SetPassport(nil) error = nil")
	}
	if _, err := store.GetPassport(ctx, contracts.Scope(42), "x"); !errors.Is(err, storage.ErrUnknownScope) {
		t.Fatalf("GetPassport(unknown scope) error = %v, want ErrUnknownScope", err)
	}
}

func TestTokenBlacklist(t *testing.T) {
	client, root, server := setup(t)
	blacklist, _ := storage.NewTokenBlacklist(client, root)
	ctx := context.Background()

	if black, err := blacklist.IsTokenBlacklisted(ctx, "tok"); err != nil || black {
		t.Fatalf("IsTokenBlacklisted() = %v, %v; want false", black, err)
	}
	if err := blacklist.BlacklistToken(ctx, "tok", 0); err == nil {
		t.Fatal("BlacklistToken(ttl 0) error = nil")
	}
	if err := blacklist.BlacklistToken(ctx, "tok", time.Minute); err != nil {
		t.Fatal(err)
	}
	if v, _ := server.Get("svc:auth:bl:tok"); v != "1" {
		t.Fatalf("blacklist key value = %q; keys = %v", v, server.Keys())
	}
	if black, err := blacklist.IsTokenBlacklisted(ctx, "tok"); err != nil || !black {
		t.Fatalf("IsTokenBlacklisted() = %v, %v; want true", black, err)
	}
	server.FastForward(2 * time.Minute)
	if black, _ := blacklist.IsTokenBlacklisted(ctx, "tok"); black {
		t.Fatal("token still blacklisted after TTL")
	}
}

func TestSessionStore(t *testing.T) {
	client, root, server := setup(t)
	sessions, _ := storage.NewSessionStore(client, root)
	ctx := context.Background()

	if ok, err := sessions.SessionExists(ctx, "jti-1"); err != nil || ok {
		t.Fatalf("SessionExists() = %v, %v; want false", ok, err)
	}
	if err := sessions.SaveSession(ctx, "jti-1", time.Minute); err != nil {
		t.Fatal(err)
	}
	if !server.Exists("svc:auth:sess:jti-1") {
		t.Fatalf("session key missing; keys = %v", server.Keys())
	}
	if ok, err := sessions.SessionExists(ctx, "jti-1"); err != nil || !ok {
		t.Fatalf("SessionExists() = %v, %v; want true", ok, err)
	}
	server.FastForward(2 * time.Minute)
	if ok, _ := sessions.SessionExists(ctx, "jti-1"); ok {
		t.Fatal("session exists after TTL")
	}
}

func TestRefreshTokenStore(t *testing.T) {
	client, root, server := setup(t)
	store, _ := storage.NewRefreshTokenStore(client, root)
	ctx := context.Background()

	token, err := store.CreateRefreshToken(ctx, 7, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 64 {
		t.Fatalf("token length = %d, want 64 hex chars", len(token))
	}
	if v, _ := server.Get("svc:auth:refresh:" + token); v != "7" {
		t.Fatalf("refresh key value = %q; keys = %v", v, server.Keys())
	}
	if id, err := store.ValidateRefreshToken(ctx, token); err != nil || id != 7 {
		t.Fatalf("ValidateRefreshToken() = %d, %v", id, err)
	}
	if _, err := store.ValidateRefreshToken(ctx, "unknown"); !errors.Is(err, storage.ErrInvalidRefreshToken) {
		t.Fatalf("ValidateRefreshToken(unknown) error = %v", err)
	}

	// Another customer cannot rotate the token; nothing changes.
	if _, err := store.RotateRefreshToken(ctx, 8, token, time.Hour); !errors.Is(err, storage.ErrRefreshTokenOwner) {
		t.Fatalf("RotateRefreshToken(other customer) error = %v", err)
	}
	if len(server.Keys()) != 1 {
		t.Fatalf("keys after rejected rotation = %v", server.Keys())
	}

	rotated, err := store.RotateRefreshToken(ctx, 7, token, 30*time.Minute)
	if err != nil {
		t.Fatalf("RotateRefreshToken() error = %v", err)
	}
	if server.Exists("svc:auth:refresh:" + token) {
		t.Fatal("old refresh token still exists after rotation")
	}
	if ttl := server.TTL("svc:auth:refresh:" + rotated); ttl != 30*time.Minute {
		t.Fatalf("rotated TTL = %v, want 30m", ttl)
	}
	if id, err := store.ValidateRefreshToken(ctx, rotated); err != nil || id != 7 {
		t.Fatalf("ValidateRefreshToken(rotated) = %d, %v", id, err)
	}

	// The consumed token cannot be rotated again.
	if _, err := store.RotateRefreshToken(ctx, 7, token, time.Hour); !errors.Is(err, storage.ErrInvalidRefreshToken) {
		t.Fatalf("second RotateRefreshToken() error = %v, want ErrInvalidRefreshToken", err)
	}

	// TTL 0 keeps the token without expiry, like CreateRefreshToken.
	persistent, err := store.RotateRefreshToken(ctx, 7, rotated, 0)
	if err != nil {
		t.Fatal(err)
	}
	if ttl := server.TTL("svc:auth:refresh:" + persistent); ttl != 0 {
		t.Fatalf("TTL with ttl 0 = %v, want none", ttl)
	}
}

func TestRefreshTokenRotationIsAtomic(t *testing.T) {
	client, root, server := setup(t)
	testConcurrentRotation(t, client, root)
	if n := len(server.Keys()); n != 1 {
		t.Fatalf("keys after concurrent rotation = %d, want 1", n)
	}
}

// TestRefreshTokenRotationAgainstRealRedis runs the rotation script on a real
// server when SKYRIX_TEST_REDIS_ADDR is set.
func TestRefreshTokenRotationAgainstRealRedis(t *testing.T) {
	addr := os.Getenv("SKYRIX_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("SKYRIX_TEST_REDIS_ADDR is not set")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })
	root, _ := engineredis.NewKeyspace("skyrix-test-" + strconv.FormatInt(time.Now().UnixNano(), 36))
	t.Cleanup(func() {
		ctx := context.Background()
		keys, _ := client.Keys(ctx, root.Key("*")).Result()
		if len(keys) > 0 {
			client.Del(ctx, keys...)
		}
	})
	testConcurrentRotation(t, client, root)
}

func testConcurrentRotation(t *testing.T, client *redis.Client, root engineredis.Keyspace) {
	t.Helper()
	store, _ := storage.NewRefreshTokenStore(client, root)
	ctx := context.Background()
	token, err := store.CreateRefreshToken(ctx, 1, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	const workers = 16
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		successes int
	)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.RotateRefreshToken(ctx, 1, token, time.Hour); err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			} else if !errors.Is(err, storage.ErrInvalidRefreshToken) {
				t.Errorf("RotateRefreshToken() error = %v", err)
			}
		}()
	}
	wg.Wait()
	if successes != 1 {
		t.Fatalf("successful concurrent rotations = %d, want 1", successes)
	}
}
