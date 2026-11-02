package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"gitlab.com/skyrix-lib/framework/kernel"
)

// Component manages the lifecycle of a PostgreSQL pool in a kernel.Runtime:
// Start pings the server, Stop closes the pool. Place it before the
// components that use the database so it starts first and stops last.
type Component struct {
	db *sql.DB
}

var (
	_ kernel.Component     = (*Component)(nil)
	_ kernel.HealthChecker = (*Component)(nil)
)

// NewComponent returns the lifecycle component for a database/sql pool.
// gormdb.NewComponent builds it from a *gorm.DB.
func NewComponent(db *sql.DB) *Component {
	return &Component{db: db}
}

// Start verifies the connection.
func (c *Component) Start(ctx context.Context) error {
	if err := c.Check(ctx); err != nil {
		return fmt.Errorf("postgres: start: %w", err)
	}
	return nil
}

// Stop closes the pool. Closing an already closed pool is not an error.
func (c *Component) Stop(context.Context) error {
	if c.db == nil {
		return nil
	}
	return c.db.Close()
}

// Check is the health check: it pings the server within ctx.
func (c *Component) Check(ctx context.Context) error {
	if c.db == nil {
		return errors.New("postgres: db is nil")
	}
	return c.db.PingContext(ctx)
}
