// Package redis connects an application to Redis: configuration, opening a
// go-redis client, the lifecycle component and key prefixing.
//
// The package exposes *redis.Client from github.com/redis/go-redis/v9
// directly. Consumers (auth, tenant cache, jobs, ...) take the client and
// implement their own storage on top of it; this package knows nothing about
// them.
package redis

import (
	"gitlab.com/skyrix-lib/framework/logger"

	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Config describes a Redis connection. Zero pool and timeout values keep the
// go-redis defaults.
type Config struct {
	// Addr is host:port of the Redis server. Required.
	Addr     string
	Username string
	Password string
	DB       int

	// TLS enables TLS when not nil. When ServerName is empty it is taken from
	// the host part of Addr.
	TLS *tls.Config

	PoolSize        int
	MinIdleConns    int
	MaxIdleConns    int
	ConnMaxIdleTime time.Duration
	ConnMaxLifetime time.Duration
	DialTimeout     time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	PoolTimeout     time.Duration

	// KeyPrefix is the root of every key the application writes, so services
	// sharing a Redis database do not collide. Required; there is no default.
	KeyPrefix string
}

// ErrEmptyKeyPrefix is returned when Config.KeyPrefix is empty.
var ErrEmptyKeyPrefix = errors.New("redis: key prefix is required")

// Validate reports configuration errors.
func (c Config) Validate() error {
	var errs []error
	if strings.TrimSpace(c.Addr) == "" {
		errs = append(errs, errors.New("redis: addr is required"))
	}
	if normalizePrefix(c.KeyPrefix) == "" {
		errs = append(errs, ErrEmptyKeyPrefix)
	}
	if c.DB < 0 {
		errs = append(errs, fmt.Errorf("redis: db must not be negative, got %d", c.DB))
	}
	return errors.Join(errs...)
}

// Options converts the configuration to go-redis options.
func (c Config) Options() *goredis.Options {
	options := &goredis.Options{
		Addr:            strings.TrimSpace(c.Addr),
		Username:        c.Username,
		Password:        c.Password,
		DB:              c.DB,
		PoolSize:        c.PoolSize,
		MinIdleConns:    c.MinIdleConns,
		MaxIdleConns:    c.MaxIdleConns,
		ConnMaxIdleTime: c.ConnMaxIdleTime,
		ConnMaxLifetime: c.ConnMaxLifetime,
		DialTimeout:     c.DialTimeout,
		ReadTimeout:     c.ReadTimeout,
		WriteTimeout:    c.WriteTimeout,
		PoolTimeout:     c.PoolTimeout,
	}
	if c.TLS != nil {
		tlsConfig := c.TLS.Clone()
		if tlsConfig.ServerName == "" {
			if host, _, err := net.SplitHostPort(options.Addr); err == nil {
				tlsConfig.ServerName = host
			}
		}
		if tlsConfig.MinVersion == 0 {
			tlsConfig.MinVersion = tls.VersionTLS12
		}
		options.TLSConfig = tlsConfig
	}
	return options
}

// Open validates the configuration and creates a client. It does not contact
// the server: the Component pings it on Start. The caller closes the client.
func Open(cfg Config) (*goredis.Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return goredis.NewClient(cfg.Options()), nil
}

// Provide opens a client for Wire and returns a cleanup that closes it,
// like engine.ProvidePostgres. Add NewComponent to the binary's runtime so
// the connection is verified on start.
func Provide(cfg Config, log logger.Interface) (*goredis.Client, func(), error) {
	client, err := Open(cfg)
	if err != nil {
		log.Error("Unable to initialize redis client", "error", err)
		return nil, nil, err
	}
	cleanup := func() {
		log.Info("Closing redis client connection")
		_ = closeClient(client)
	}
	return client, cleanup, nil
}

// ProvideKeyspace returns the application root keyspace from cfg.KeyPrefix.
func ProvideKeyspace(cfg Config) (Keyspace, error) {
	return NewKeyspace(cfg.KeyPrefix)
}

// closeClient closes client and treats an already closed client as success,
// so the Wire cleanup and Component.Stop may both run.
func closeClient(client *goredis.Client) error {
	if err := client.Close(); err != nil && !errors.Is(err, goredis.ErrClosed) {
		return err
	}
	return nil
}
