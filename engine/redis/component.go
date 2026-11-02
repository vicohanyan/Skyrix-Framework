package redis

import (
	"context"
	"errors"
	"fmt"

	"gitlab.com/skyrix-lib/framework/kernel"

	goredis "github.com/redis/go-redis/v9"
)

// Component manages the lifecycle of a Redis client in a kernel.Runtime:
// Start pings the server, Stop closes the client. Place it before the
// components that use Redis so it starts first and stops last.
type Component struct {
	client *goredis.Client
}

var (
	_ kernel.Component     = (*Component)(nil)
	_ kernel.HealthChecker = (*Component)(nil)
)

// NewComponent returns the lifecycle component for client.
func NewComponent(client *goredis.Client) *Component {
	return &Component{client: client}
}

// Start verifies the connection.
func (c *Component) Start(ctx context.Context) error {
	if err := c.Check(ctx); err != nil {
		return fmt.Errorf("redis: start: %w", err)
	}
	return nil
}

// Stop closes the client. Closing an already closed client is not an error.
func (c *Component) Stop(context.Context) error {
	if c.client == nil {
		return nil
	}
	return closeClient(c.client)
}

// Check is the health check: it pings the server within ctx.
func (c *Component) Check(ctx context.Context) error {
	if c.client == nil {
		return errors.New("redis: client is nil")
	}
	return c.client.Ping(ctx).Err()
}
