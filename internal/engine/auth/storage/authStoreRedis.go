package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"skyrix/internal/engine/auth/contracts"
	"skyrix/internal/logger"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisAuthStore provides a high-level interface for Redis operations including passport management,
// token blacklisting, session management, and generic key-value operations.
type RedisAuthStore struct {
	client    *redis.Client
	logger    logger.Interface
	keyPrefix string
	statusTTL time.Duration
}

func NewRedisAuthStore(client *redis.Client, lg logger.Interface, storeOpts contracts.StoreOpts) *RedisAuthStore {
	prefix := strings.TrimSuffix(strings.TrimSpace(storeOpts.KeyPrefix), ":")
	if prefix == "" {
		prefix = "skyrix-catalog"
	}
	ttl := storeOpts.StatusTTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &RedisAuthStore{client: client, logger: lg, keyPrefix: prefix, statusTTL: ttl}
}

// norm normalizes a string by converting to lowercase and trimming whitespace.
func norm(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// kPassport generates a Redis key for user passport storage.
// Format: "<prefix>:sub:<scope>:<normalized_id>"
func (r *RedisAuthStore) kPassport(scope contracts.Scope, id string) string {
	switch scope {
	case contracts.ScopeNamespace:
		return r.keyPrefix + ":sub:namespace:" + norm(id)
	case contracts.ScopeDomain:
		return r.keyPrefix + ":sub:domain:" + norm(id)
	default:
		return r.keyPrefix + ":" + norm(id)
	}
}

// kBlacklist generates a Redis key for token blacklist storage.
// Format: "<prefix>:auth:bl:<token>"
func (r *RedisAuthStore) kBlacklist(token string) string {
	return r.keyPrefix + ":auth:bl:" + token
}

// ttlForPassport calculates the appropriate TTL for a passport based on activeTo.
// Returns default statusTTL if activeTo is nil, 1 minute if in the past,
// or min(time until activeTo, statusTTL) otherwise.
func (r *RedisAuthStore) ttlForPassport(passport *contracts.Passport) time.Duration {
	if passport.ActiveTo == nil {
		return r.statusTTL
	}
	now := time.Now().UTC()
	if !passport.ActiveTo.After(now) {
		return time.Minute
	}
	left := time.Until(*passport.ActiveTo)
	if left < r.statusTTL {
		return left
	}
	return r.statusTTL
}

// bool01 converts a boolean to a string representation ("1" for true, "0" for false).
func bool01(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// passportToHash converts a SubscriberPassport DTO to a Redis hash map.
// Timestamps are converted to RFC3339 strings; booleans to "1" or "0".
func passportToHash(passport *contracts.Passport) map[string]any {
	updated := passport.UpdatedAt
	if updated.IsZero() {
		updated = time.Now().UTC()
	}
	activeTo := ""
	if passport.ActiveTo != nil {
		activeTo = passport.ActiveTo.UTC().Format(time.RFC3339)
	}
	return map[string]any{
		"v":         "1",
		"namespace": passport.Namespace,
		"domain":    passport.Domain,
		"schema":    passport.Schema,
		"isActive":  bool01(passport.IsActive),
		"activeTo":  activeTo,
		"updatedAt": updated.UTC().Format(time.RFC3339),
	}
}

// hashToPassport converts a Redis hash map to a SubscriberPassport DTO.
// Parses RFC3339 timestamp strings back to time.Time values.
// Returns nil if the map is empty.
func hashToPassport(m map[string]string) *contracts.Passport {
	if len(m) == 0 {
		return nil
	}
	pp := &contracts.Passport{
		Namespace: m["namespace"],
		Domain:    m["domain"],
		Schema:    m["schema"],
		IsActive:  m["isActive"] == "1",
	}
	if s := m["updatedAt"]; s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			pp.UpdatedAt = t
		}
	}
	if s := m["activeTo"]; s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			pp.ActiveTo = &t
		}
	}
	return pp
}

// GetPassport retrieves a subscriber passport from Redis by scope and identifier.
// Returns nil if not found.
func (r *RedisAuthStore) GetPassport(ctx context.Context, scope contracts.Scope, id string) (*contracts.Passport, error) {
	m, err := r.client.HGetAll(ctx, r.kPassport(scope, id)).Result()
	if err != nil {
		return nil, err
	}
	return hashToPassport(m), nil
}

// SetPassport stores a subscriber passport in Redis with automatic TTL calculation.
// TTL is calculated from activeTo, or uses ttlOverride if provided and > 0.
// Uses Redis pipeline for atomic HSET and EXPIRE. Returns error if pp is nil.
func (r *RedisAuthStore) SetPassport(ctx context.Context, scope contracts.Scope, id string, passport *contracts.Passport, ttlOverride ...time.Duration) error {
	if passport == nil {
		return errors.New("passport cannot be nil")
	}
	key := r.kPassport(scope, id)
	ttl := r.statusTTL
	if len(ttlOverride) > 0 && ttlOverride[0] > 0 {
		ttl = ttlOverride[0]
	} else {
		ttl = r.ttlForPassport(passport)
	}
	pipe := r.client.TxPipeline()
	pipe.HSet(ctx, key, passportToHash(passport))
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// SetPassportBoth stores a subscriber passport under both namespace and domain keys.
// Allows lookup by either identifier. Uses Redis pipeline for atomic operations.
// TTL is calculated from activeTo. Returns error if passport is nil.
func (r *RedisAuthStore) SetPassportBoth(ctx context.Context, namespace, domain string, passport *contracts.Passport) error {
	if passport == nil {
		return errors.New("passport cannot be nil")
	}
	ttl := r.ttlForPassport(passport)
	pipe := r.client.TxPipeline()
	pipe.HSet(ctx, r.kPassport(contracts.ScopeNamespace, namespace), passportToHash(passport))
	pipe.Expire(ctx, r.kPassport(contracts.ScopeNamespace, namespace), ttl)
	pipe.HSet(ctx, r.kPassport(contracts.ScopeDomain, domain), passportToHash(passport))
	pipe.Expire(ctx, r.kPassport(contracts.ScopeDomain, domain), ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// InvalidatePassport removes a subscriber passport from Redis.
// Typically called when a subscription is deactivated or deleted.
func (r *RedisAuthStore) InvalidatePassport(ctx context.Context, scope contracts.Scope, id string) error {
	return r.client.Del(ctx, r.kPassport(scope, id)).Err()
}

// BlacklistToken adds a JWT token to the blacklist, preventing its use for authentication.
// Returns error if expiration <= 0.
func (r *RedisAuthStore) BlacklistToken(ctx context.Context, token string, expiration time.Duration) error {
	if expiration <= 0 {
		return errors.New("expiration must be > 0")
	}
	return r.client.Set(ctx, r.kBlacklist(token), "1", expiration).Err()
}

// IsTokenBlacklisted checks if a JWT token is in the blacklist.
// Returns false if not blacklisted or if the entry has expired.
func (r *RedisAuthStore) IsTokenBlacklisted(ctx context.Context, token string) (bool, error) {
	val, err := r.client.Get(ctx, r.kBlacklist(token)).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	return val == "1", err
}

// kSession generates a Redis key for session storage.
// Format: "<prefix>:auth:sess:<jti>"
func (r *RedisAuthStore) kSession(jti string) string { return r.keyPrefix + ":auth:sess:" + jti }

// SaveSession stores a session identifier in Redis with the specified TTL.
// Stored as a simple key-value pair with value "1".
func (r *RedisAuthStore) SaveSession(ctx context.Context, jti string, ttl time.Duration) error {
	return r.client.Set(ctx, r.kSession(jti), "1", ttl).Err()
}

// SessionExists checks if a session identifier exists in Redis.
// Returns false if the session doesn't exist or has expired.
func (r *RedisAuthStore) SessionExists(ctx context.Context, jti string) (bool, error) {
	val, err := r.client.Get(ctx, r.kSession(jti)).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	return val == "1", err
}

func (r *RedisAuthStore) CreateRefreshToken(
	ctx context.Context,
	customerID int64,
	ttl time.Duration,
) (string, error) {
	token, err := generateRefreshToken()
	if err != nil {
		return "", err
	}

	key := fmt.Sprintf("%s:auth:refresh:%s", r.keyPrefix, token)

	err = r.client.Set(ctx, key, fmt.Sprintf("%d", customerID), ttl).Err()
	if err != nil {
		return "", err
	}

	return token, nil
}

// ValidateRefreshToken resolves the stored customer ID by token and ensures it has not expired.
func (r *RedisAuthStore) ValidateRefreshToken(
	ctx context.Context,
	token string,
) (int64, error) {
	key := fmt.Sprintf("%s:auth:refresh:%s", r.keyPrefix, token)

	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return 0, errors.New("invalid or expired refresh token")
	}

	var id int64
	_, err = fmt.Sscan(val, &id)
	if err != nil {
		return 0, errors.New("invalid refresh token value")
	}

	return id, nil
}

// RotateRefreshToken deletes the old refresh token and issues a new one for the same customer.
func (r *RedisAuthStore) RotateRefreshToken(
	ctx context.Context,
	customerID int64,
	oldToken string,
	ttl time.Duration,
) (string, error) {
	oldKey := fmt.Sprintf("%s:auth:refresh:%s", r.keyPrefix, oldToken)
	_ = r.client.Del(ctx, oldKey).Err()

	return r.CreateRefreshToken(ctx, customerID, ttl)
}

// generateRefreshToken returns a cryptographically random 256-bit token hex string.
func generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
