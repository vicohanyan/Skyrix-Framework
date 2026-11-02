// Package storage implements the auth stores on Redis. Each store takes the
// go-redis client and the application root keyspace (REDIS_KEY_PREFIX) and
// builds its keys in its own namespace:
//
//	<prefix>:sub:namespace:<id>   passports by namespace (hash)
//	<prefix>:sub:domain:<id>      passports by domain (hash)
//	<prefix>:auth:bl:<token>      blacklisted access tokens
//	<prefix>:auth:sess:<jti>      sessions
//	<prefix>:auth:refresh:<token> refresh tokens (value: customer ID)
//
// The layout below the prefix is the one used by the former RedisAuthStore,
// so existing data stays readable when REDIS_KEY_PREFIX equals the prefix the
// service used before.
package storage

import (
	"errors"
	"fmt"

	engineredis "gitlab.com/skyrix-lib/framework/engine/redis"

	"github.com/redis/go-redis/v9"
)

const (
	// AuthNamespace holds blacklist, session and refresh token keys.
	AuthNamespace = "auth"
	// PassportNamespace holds subscriber passports.
	PassportNamespace = "sub"
)

func checkDeps(store string, client *redis.Client, root engineredis.Keyspace) error {
	if client == nil {
		return fmt.Errorf("auth %s store: redis client is nil", store)
	}
	if root.IsZero() {
		return fmt.Errorf("auth %s store: %w", store, engineredis.ErrEmptyKeyPrefix)
	}
	return nil
}

// isNil reports whether err is a missing key.
func isNil(err error) bool { return errors.Is(err, redis.Nil) }
