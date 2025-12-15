package engine

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Cache interface {
	// Get returns cached bytes by key. The bool indicates cache hit.
	Get(ctx context.Context, key string) ([]byte, bool, error)
	// Set stores bytes with TTL; ttl<=0 semantics depend on implementation.
	Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
	// Del removes a cache entry.
	Del(ctx context.Context, key string) error
	// Exists reports whether a key is present.
	Exists(ctx context.Context, key string) (bool, error)
}

type DB interface {
	// WithContext returns a new session bound to the supplied context
	// (search_path/schema adjustments are applied by implementations).
	WithContext(ctx context.Context) *gorm.DB
	// Main returns the name of the primary schema/database.
	Main() string
}

type TransactionManager interface {
	// Execute runs fn inside a transaction, committing on success and rolling back on errors/panics.
	Execute(ctx context.Context, fn func(tx *gorm.DB) error) error
}
