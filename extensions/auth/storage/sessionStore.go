package storage

import (
	"context"
	"time"

	engineredis "gitlab.com/skyrix-lib/framework/engine/redis"
	"gitlab.com/skyrix-lib/framework/extensions/auth/contracts"

	"github.com/redis/go-redis/v9"
)

// SessionStore stores live sessions at <prefix>:auth:sess:<jti>.
type SessionStore struct {
	client *redis.Client
	keys   engineredis.Keyspace
}

var _ contracts.SessionStore = (*SessionStore)(nil)

// NewSessionStore returns the session store under <prefix>:auth.
func NewSessionStore(client *redis.Client, root engineredis.Keyspace) (*SessionStore, error) {
	if err := checkDeps("session", client, root); err != nil {
		return nil, err
	}
	return &SessionStore{client: client, keys: root.Sub(AuthNamespace)}, nil
}

func (s *SessionStore) key(jti string) string { return s.keys.Key("sess", jti) }

// SaveSession stores the session identifier with ttl.
func (s *SessionStore) SaveSession(ctx context.Context, jti string, ttl time.Duration) error {
	return s.client.Set(ctx, s.key(jti), "1", ttl).Err()
}

// SessionExists reports whether the session is stored and not expired.
func (s *SessionStore) SessionExists(ctx context.Context, jti string) (bool, error) {
	val, err := s.client.Get(ctx, s.key(jti)).Result()
	if isNil(err) {
		return false, nil
	}
	return val == "1", err
}
