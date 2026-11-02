package providers

import (
	"errors"
	"testing"

	engineredis "gitlab.com/skyrix-lib/framework/engine/redis"
	"gitlab.com/skyrix-lib/template/internal/config"
)

func TestProvideRedisOptions(t *testing.T) {
	options := ProvideRedisOptions(&config.Redis{Host: "redis", Port: 6380, Pass: "p", Db: 3, KeyPrefix: "orders"})
	if options.Addr != "redis:6380" || options.Password != "p" || options.DB != 3 || options.KeyPrefix != "orders" {
		t.Fatalf("options = %+v", options)
	}
}

func TestRedisRequiresKeyPrefix(t *testing.T) {
	options := ProvideRedisOptions(&config.Redis{Host: "redis", Port: 6379})
	if _, _, err := engineredis.Provide(options, noopProviderLogger{}); !errors.Is(err, engineredis.ErrEmptyKeyPrefix) {
		t.Fatalf("Provide() without REDIS_KEY_PREFIX error = %v, want ErrEmptyKeyPrefix", err)
	}
}
