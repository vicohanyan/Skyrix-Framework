package providers

import (
	"gitlab.com/skyrix-lib/framework/engine"
	"gitlab.com/skyrix-lib/framework/engine/scope"
	"gitlab.com/skyrix-lib/framework/logging"
	"gitlab.com/skyrix-lib/template/internal/config"
	"time"

	"github.com/goforj/wire"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// ProviderSet wires raw clients into engine services (wrappers/adapters).
// This is infrastructure microcore, should be stable.
var EngineProviderSet = wire.NewSet(
	ProvideDatabaseService,
	ProvideRedisService,

	// Bind Cache interface to chosen implementation.
	wire.Bind(new(engine.Cache), new(*engine.Redis)),
)

func ProvideDatabaseService(db *gorm.DB, cfg *config.Config) (*engine.Database, error) {
	if err := db.Use(&scope.Plugin{}); err != nil {
		return nil, err
	}
	return engine.NewDatabaseService(db, cfg.Database.MainSchema), nil
}

func ProvideRedisService(redisClient *redis.Client, log logger.Interface, cfg *config.Config) *engine.Redis {
	redisOpts := engine.RedisOpts{
		KeyPrefix: cfg.TenantCache.KeyPrefix,
		StatusTTL: 5 * time.Minute,
	}
	return engine.NewRedisService(redisClient, log, redisOpts)
}
