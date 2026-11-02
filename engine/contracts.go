package engine

import (
	"context"

	"gorm.io/gorm"
)

type DB interface {
	// WithContext returns a new session bound to the supplied context
	// (search_path/schema adjustments are applied by implementations).
	WithContext(ctx context.Context) *gorm.DB
	// Main returns the name of the primary schema/database.
	Main() string
}
