package engine

import (
	"skyrix/internal/config"
	"skyrix/internal/logger"
	"time"

	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// ProviderSet wires raw clients into engine services (wrappers/adapters).
// This is infrastructure microcore, should be stable.
var ProviderSet = wire.NewSet(
	ProvideDatabaseService,
	ProvideRedisService,

	// Bind Cache interface to chosen implementation.
	wire.Bind(new(Cache), new(*Redis)),
)

func ProvideDatabaseService(db *gorm.DB, cfg *config.Config) *Database {
	return NewDatabaseService(db, cfg.Database.MainSchema)
}

func ProvideRedisService(redisClient *redis.Client, log logger.Interface, cfg *config.Config) *Redis {
	redisOpts := RedisOpts{
		KeyPrefix: cfg.TenantCache.KeyPrefix,
		StatusTTL: 5 * time.Minute,
	}
	return NewRedisService(redisClient, log, redisOpts)
}
