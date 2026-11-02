package storage

import (
	"context"
	"errors"
	"time"

	engineredis "gitlab.com/skyrix-lib/framework/engine/redis"
	"gitlab.com/skyrix-lib/framework/extensions/auth/contracts"

	"github.com/redis/go-redis/v9"
)

// TokenBlacklist stores revoked access tokens at <prefix>:auth:bl:<token>.
type TokenBlacklist struct {
	client *redis.Client
	keys   engineredis.Keyspace
}

var _ contracts.TokenBlacklist = (*TokenBlacklist)(nil)

// NewTokenBlacklist returns the blacklist under <prefix>:auth.
func NewTokenBlacklist(client *redis.Client, root engineredis.Keyspace) (*TokenBlacklist, error) {
	if err := checkDeps("blacklist", client, root); err != nil {
		return nil, err
	}
	return &TokenBlacklist{client: client, keys: root.Sub(AuthNamespace)}, nil
}

func (b *TokenBlacklist) key(token string) string { return b.keys.Key("bl", token) }

// BlacklistToken revokes token for expiration, which must be positive.
func (b *TokenBlacklist) BlacklistToken(ctx context.Context, token string, expiration time.Duration) error {
	if expiration <= 0 {
		return errors.New("expiration must be > 0")
	}
	return b.client.Set(ctx, b.key(token), "1", expiration).Err()
}

// IsTokenBlacklisted reports whether token is revoked.
func (b *TokenBlacklist) IsTokenBlacklisted(ctx context.Context, token string) (bool, error) {
	val, err := b.client.Get(ctx, b.key(token)).Result()
	if isNil(err) {
		return false, nil
	}
	return val == "1", err
}
