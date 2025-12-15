package contracts

import (
	"context"
	"time"

	"skyrix/internal/utils/security"
)

type Scope int

const (
	// ScopeNamespace targets a subscriber namespace.
	ScopeNamespace Scope = iota
	// ScopeDomain targets a domain or host.
	ScopeDomain
)

// StoreOpts holds Redis-related configuration for auth storage.
type StoreOpts struct {
	KeyPrefix string
	StatusTTL time.Duration
}

// Passport describes a subscriber authorization context cached in Redis.
type Passport struct {
	V         int
	Namespace string
	Domain    string
	Schema    string
	IsActive  bool
	ActiveTo  *time.Time
	UpdatedAt time.Time
}

// PassportStore handles caching and invalidation of subscriber passports for namespaces and domains.
type PassportStore interface {
	GetPassport(ctx context.Context, scope Scope, id string) (*Passport, error)
	SetPassport(ctx context.Context, scope Scope, id string, passport *Passport, ttlOverride ...time.Duration) error
	SetPassportBoth(ctx context.Context, namespace, domain string, passport *Passport) error
	InvalidatePassport(ctx context.Context, scope Scope, id string) error
}

type JWT interface {
	GenerateToken(claims security.CustomClaims) (string, error)
	ParseToken(tokenString string) (*security.CustomClaims, error)
	ValidateToken(ctx context.Context, tokenString string) (*security.CustomClaims, error)
}

// Store provides the backing storage for auth concerns: blacklist, sessions, and refresh tokens.
type Store interface {
	BlacklistToken(ctx context.Context, token string, ttl time.Duration) error
	IsTokenBlacklisted(ctx context.Context, token string) (bool, error)

	SaveSession(ctx context.Context, jti string, ttl time.Duration) error
	SessionExists(ctx context.Context, jti string) (bool, error)

	CreateRefreshToken(ctx context.Context, customerID int64, ttl time.Duration) (string, error)
	ValidateRefreshToken(ctx context.Context, token string) (int64, error)
	RotateRefreshToken(ctx context.Context, customerID int64, oldToken string, ttl time.Duration) (string, error)
}
