package redis_test

import (
	"gitlab.com/skyrix-lib/framework/logger"

	"context"
	"crypto/tls"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	engineredis "gitlab.com/skyrix-lib/framework/engine/redis"

	"github.com/alicebob/miniredis/v2"
)

func TestValidateRequiresKeyPrefix(t *testing.T) {
	for _, prefix := range []string{"", "   ", ":", " : "} {
		err := engineredis.Config{Addr: "localhost:6379", KeyPrefix: prefix}.Validate()
		if !errors.Is(err, engineredis.ErrEmptyKeyPrefix) {
			t.Errorf("Validate() with KeyPrefix %q = %v, want ErrEmptyKeyPrefix", prefix, err)
		}
		if _, err := engineredis.Open(engineredis.Config{Addr: "localhost:6379", KeyPrefix: prefix}); !errors.Is(err, engineredis.ErrEmptyKeyPrefix) {
			t.Errorf("Open() with KeyPrefix %q = %v, want ErrEmptyKeyPrefix", prefix, err)
		}
		if _, err := engineredis.ProvideKeyspace(engineredis.Config{KeyPrefix: prefix}); !errors.Is(err, engineredis.ErrEmptyKeyPrefix) {
			t.Errorf("ProvideKeyspace() with KeyPrefix %q = %v, want ErrEmptyKeyPrefix", prefix, err)
		}
	}
}

func TestValidateRejectsMissingAddrAndNegativeDB(t *testing.T) {
	err := engineredis.Config{KeyPrefix: "app", DB: -1}.Validate()
	if err == nil || !strings.Contains(err.Error(), "addr") || !strings.Contains(err.Error(), "db") {
		t.Fatalf("Validate() = %v, want addr and db errors", err)
	}
	if err := (engineredis.Config{Addr: "localhost:6379", KeyPrefix: "app"}).Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestOptions(t *testing.T) {
	cfg := engineredis.Config{
		Addr: " redis.internal:6380 ", Username: "user", Password: "secret", DB: 2,
		TLS:      &tls.Config{},
		PoolSize: 20, MinIdleConns: 2, DialTimeout: time.Second, ReadTimeout: 2 * time.Second,
		WriteTimeout: 3 * time.Second, PoolTimeout: 4 * time.Second, KeyPrefix: "app",
	}
	options := cfg.Options()
	if options.Addr != "redis.internal:6380" || options.Username != "user" || options.Password != "secret" || options.DB != 2 {
		t.Fatalf("connection options = %+v", options)
	}
	if options.PoolSize != 20 || options.MinIdleConns != 2 || options.DialTimeout != time.Second ||
		options.ReadTimeout != 2*time.Second || options.WriteTimeout != 3*time.Second || options.PoolTimeout != 4*time.Second {
		t.Fatalf("pool options = %+v", options)
	}
	if options.TLSConfig == nil || options.TLSConfig.ServerName != "redis.internal" || options.TLSConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("TLSConfig = %+v, want ServerName redis.internal and TLS 1.2", options.TLSConfig)
	}
	if cfg.TLS.ServerName != "" {
		t.Fatal("Options() modified the caller's tls.Config")
	}
	if (engineredis.Config{Addr: "localhost:6379"}).Options().TLSConfig != nil {
		t.Fatal("TLSConfig set without Config.TLS")
	}
}

func TestComponentLifecycle(t *testing.T) {
	server := miniredis.RunT(t)
	client, cleanup, err := engineredis.Provide(engineredis.Config{Addr: server.Addr(), KeyPrefix: "app"}, nopLogger{})
	if err != nil {
		t.Fatalf("Provide() error = %v", err)
	}
	component := engineredis.NewComponent(client)
	ctx := context.Background()

	if err := component.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := component.Check(ctx); err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if err := client.Set(ctx, "k", "v", 0).Err(); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	server.Close()
	if err := component.Check(ctx); err == nil {
		t.Fatal("Check() after server shutdown = nil, want error")
	}

	if err := component.Stop(ctx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if err := component.Stop(ctx); err != nil {
		t.Fatalf("second Stop() error = %v", err)
	}
	cleanup() // closing an already stopped client must not panic
	if err := component.Check(ctx); err == nil {
		t.Fatal("Check() after Stop = nil, want error")
	}
}

func TestComponentStartFailsWithoutServer(t *testing.T) {
	server := miniredis.RunT(t)
	addr := server.Addr()
	server.Close()

	client, err := engineredis.Open(engineredis.Config{Addr: addr, KeyPrefix: "app", DialTimeout: 200 * time.Millisecond})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	component := engineredis.NewComponent(client)
	t.Cleanup(func() { _ = component.Stop(context.Background()) })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := component.Start(ctx); err == nil {
		t.Fatal("Start() = nil, want ping error")
	}
}

func TestProvideRejectsInvalidConfig(t *testing.T) {
	if _, _, err := engineredis.Provide(engineredis.Config{Addr: "localhost:6379"}, nopLogger{}); !errors.Is(err, engineredis.ErrEmptyKeyPrefix) {
		t.Fatalf("Provide() error = %v, want ErrEmptyKeyPrefix", err)
	}
}

// TestComponentAgainstRealRedis runs the lifecycle against a real server when
// SKYRIX_TEST_REDIS_ADDR is set.
func TestComponentAgainstRealRedis(t *testing.T) {
	addr := os.Getenv("SKYRIX_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("SKYRIX_TEST_REDIS_ADDR is not set")
	}
	client, err := engineredis.Open(engineredis.Config{Addr: addr, KeyPrefix: "skyrix-test"})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	component := engineredis.NewComponent(client)
	ctx := context.Background()
	if err := component.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := component.Stop(ctx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

type nopLogger struct{}

func (nopLogger) Error(string, ...interface{}) {}
func (nopLogger) Info(string, ...interface{})  {}
func (nopLogger) Debug(string, ...interface{}) {}
func (nopLogger) Warn(string, ...interface{})  {}

func (nopLogger) With(...interface{}) logger.Interface { return nopLogger{} }
