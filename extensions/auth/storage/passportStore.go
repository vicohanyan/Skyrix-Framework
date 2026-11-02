package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	engineredis "gitlab.com/skyrix-lib/framework/engine/redis"
	"gitlab.com/skyrix-lib/framework/extensions/auth/contracts"

	"github.com/redis/go-redis/v9"
)

// DefaultPassportTTL is used when PassportOpts.StatusTTL is not positive.
const DefaultPassportTTL = 10 * time.Minute

// ErrUnknownScope is returned for a passport scope other than ScopeNamespace
// and ScopeDomain.
var ErrUnknownScope = errors.New("unknown passport scope")

// PassportStore caches subscriber passports as Redis hashes.
type PassportStore struct {
	client    *redis.Client
	keys      engineredis.Keyspace
	statusTTL time.Duration
}

var _ contracts.PassportStore = (*PassportStore)(nil)

// NewPassportStore returns a passport store under <prefix>:sub.
func NewPassportStore(client *redis.Client, root engineredis.Keyspace, opts contracts.PassportOpts) (*PassportStore, error) {
	if err := checkDeps("passport", client, root); err != nil {
		return nil, err
	}
	ttl := opts.StatusTTL
	if ttl <= 0 {
		ttl = DefaultPassportTTL
	}
	return &PassportStore{client: client, keys: root.Sub(PassportNamespace), statusTTL: ttl}, nil
}

// norm normalizes a string by converting to lowercase and trimming whitespace.
func norm(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// key returns <prefix>:sub:<scope>:<normalized id>.
func (s *PassportStore) key(scope contracts.Scope, id string) (string, error) {
	switch scope {
	case contracts.ScopeNamespace:
		return s.keys.Key("namespace", norm(id)), nil
	case contracts.ScopeDomain:
		return s.keys.Key("domain", norm(id)), nil
	default:
		return "", fmt.Errorf("%w: %d", ErrUnknownScope, scope)
	}
}

// ttlFor returns statusTTL when activeTo is nil, 1 minute when it is in the
// past, otherwise min(time until activeTo, statusTTL).
func (s *PassportStore) ttlFor(passport *contracts.Passport) time.Duration {
	if passport.ActiveTo == nil {
		return s.statusTTL
	}
	if !passport.ActiveTo.After(time.Now().UTC()) {
		return time.Minute
	}
	if left := time.Until(*passport.ActiveTo); left < s.statusTTL {
		return left
	}
	return s.statusTTL
}

// GetPassport returns the passport, or nil when it is not cached.
func (s *PassportStore) GetPassport(ctx context.Context, scope contracts.Scope, id string) (*contracts.Passport, error) {
	key, err := s.key(scope, id)
	if err != nil {
		return nil, err
	}
	m, err := s.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	return hashToPassport(m), nil
}

// SetPassport caches the passport. The TTL is ttlOverride when given and
// positive, otherwise derived from ActiveTo. HSET and EXPIRE run in one
// MULTI/EXEC transaction.
func (s *PassportStore) SetPassport(ctx context.Context, scope contracts.Scope, id string, passport *contracts.Passport, ttlOverride ...time.Duration) error {
	if passport == nil {
		return errors.New("passport cannot be nil")
	}
	key, err := s.key(scope, id)
	if err != nil {
		return err
	}
	ttl := s.ttlFor(passport)
	if len(ttlOverride) > 0 && ttlOverride[0] > 0 {
		ttl = ttlOverride[0]
	}
	pipe := s.client.TxPipeline()
	pipe.HSet(ctx, key, passportToHash(passport))
	pipe.Expire(ctx, key, ttl)
	_, err = pipe.Exec(ctx)
	return err
}

// SetPassportBoth caches the passport under its namespace and domain keys in
// one MULTI/EXEC transaction.
func (s *PassportStore) SetPassportBoth(ctx context.Context, namespace, domain string, passport *contracts.Passport) error {
	if passport == nil {
		return errors.New("passport cannot be nil")
	}
	ttl := s.ttlFor(passport)
	namespaceKey, _ := s.key(contracts.ScopeNamespace, namespace)
	domainKey, _ := s.key(contracts.ScopeDomain, domain)
	pipe := s.client.TxPipeline()
	pipe.HSet(ctx, namespaceKey, passportToHash(passport))
	pipe.Expire(ctx, namespaceKey, ttl)
	pipe.HSet(ctx, domainKey, passportToHash(passport))
	pipe.Expire(ctx, domainKey, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// InvalidatePassport removes the cached passport.
func (s *PassportStore) InvalidatePassport(ctx context.Context, scope contracts.Scope, id string) error {
	key, err := s.key(scope, id)
	if err != nil {
		return err
	}
	return s.client.Del(ctx, key).Err()
}

// bool01 converts a boolean to "1" or "0".
func bool01(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// passportToHash converts a passport to a Redis hash. Timestamps are RFC3339
// strings; booleans are "1" or "0".
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

// hashToPassport converts a Redis hash to a passport; nil when m is empty.
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
