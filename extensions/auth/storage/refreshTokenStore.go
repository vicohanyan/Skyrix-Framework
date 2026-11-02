package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	engineredis "gitlab.com/skyrix-lib/framework/engine/redis"
	"gitlab.com/skyrix-lib/framework/extensions/auth/contracts"

	"github.com/redis/go-redis/v9"
)

var (
	// ErrInvalidRefreshToken is returned for an unknown or expired token.
	ErrInvalidRefreshToken = errors.New("invalid or expired refresh token")
	// ErrRefreshTokenOwner is returned when a token being rotated belongs to
	// another customer.
	ErrRefreshTokenOwner = errors.New("refresh token belongs to another customer")
)

// rotateRefreshToken consumes KEYS[1] when it holds ARGV[1] (the customer ID)
// and stores KEYS[2] = ARGV[1] with a TTL of ARGV[2] milliseconds (no expiry
// when ARGV[2] is 0, like SET with a zero TTL in CreateRefreshToken), in one
// atomic step. Returns 1 on success, 0 when KEYS[1] is missing, -1 when it
// belongs to another customer. Two concurrent rotations of the same token
// cannot both succeed.
var rotateRefreshToken = redis.NewScript(`
local current = redis.call("GET", KEYS[1])
if not current then
	return 0
end
if current ~= ARGV[1] then
	return -1
end
redis.call("DEL", KEYS[1])
if tonumber(ARGV[2]) > 0 then
	redis.call("SET", KEYS[2], ARGV[1], "PX", ARGV[2])
else
	redis.call("SET", KEYS[2], ARGV[1])
end
return 1
`)

// RefreshTokenStore stores refresh tokens at <prefix>:auth:refresh:<token>
// with the customer ID as value.
type RefreshTokenStore struct {
	client *redis.Client
	keys   engineredis.Keyspace
}

var _ contracts.RefreshTokenStore = (*RefreshTokenStore)(nil)

// NewRefreshTokenStore returns the refresh token store under <prefix>:auth.
func NewRefreshTokenStore(client *redis.Client, root engineredis.Keyspace) (*RefreshTokenStore, error) {
	if err := checkDeps("refresh token", client, root); err != nil {
		return nil, err
	}
	return &RefreshTokenStore{client: client, keys: root.Sub(AuthNamespace)}, nil
}

func (s *RefreshTokenStore) key(token string) string { return s.keys.Key("refresh", token) }

// CreateRefreshToken issues a token for customerID valid for ttl.
func (s *RefreshTokenStore) CreateRefreshToken(ctx context.Context, customerID int64, ttl time.Duration) (string, error) {
	token, err := generateRefreshToken()
	if err != nil {
		return "", err
	}
	if err := s.client.Set(ctx, s.key(token), strconv.FormatInt(customerID, 10), ttl).Err(); err != nil {
		return "", err
	}
	return token, nil
}

// ValidateRefreshToken returns the customer ID stored for token.
func (s *RefreshTokenStore) ValidateRefreshToken(ctx context.Context, token string) (int64, error) {
	val, err := s.client.Get(ctx, s.key(token)).Result()
	if err != nil {
		return 0, ErrInvalidRefreshToken
	}
	var id int64
	if _, err := fmt.Sscan(val, &id); err != nil {
		return 0, errors.New("invalid refresh token value")
	}
	return id, nil
}

// RotateRefreshToken atomically deletes oldToken and issues a new token for
// customerID. It fails with ErrInvalidRefreshToken when oldToken is unknown
// or expired (including when it was already rotated) and with
// ErrRefreshTokenOwner when it belongs to another customer; in both cases
// nothing is changed.
func (s *RefreshTokenStore) RotateRefreshToken(ctx context.Context, customerID int64, oldToken string, ttl time.Duration) (string, error) {
	token, err := generateRefreshToken()
	if err != nil {
		return "", err
	}
	var ttlMillis int64
	if ttl > 0 {
		ttlMillis = max(ttl.Milliseconds(), 1)
	}
	result, err := rotateRefreshToken.Run(ctx, s.client,
		[]string{s.key(oldToken), s.key(token)},
		strconv.FormatInt(customerID, 10), ttlMillis,
	).Int64()
	if err != nil {
		return "", err
	}
	switch result {
	case 1:
		return token, nil
	case -1:
		return "", ErrRefreshTokenOwner
	default:
		return "", ErrInvalidRefreshToken
	}
}

// generateRefreshToken returns a cryptographically random 256-bit token hex string.
func generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
