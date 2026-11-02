package providers

import (
	"time"

	"gitlab.com/skyrix-lib/framework/engine"
	"gitlab.com/skyrix-lib/framework/engine/scope"
	"gitlab.com/skyrix-lib/framework/logging"
	"gitlab.com/skyrix-lib/template/internal/config"

	"github.com/goforj/wire"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// PostgresSet opens PostgreSQL through GORM and provides *engine.Database.
// Include it only in binaries that use the database.
var PostgresSet = wire.NewSet(
	ProvideDatabaseConfig,
	ProvidePostgresOptions,
	engine.ProvidePostgres,
	ProvideDatabaseService,
)

// RedisSet connects to Redis and provides engine.Cache. Include it only in
// binaries that use the cache.
var RedisSet = wire.NewSet(
	ProvideRedisConfig,
	ProvideRedisOptions,
	engine.ProvideRedis,
	ProvideRedisService,
	wire.Bind(new(engine.Cache), new(*engine.Redis)),
)

func ProvideDatabaseConfig(cfg *config.Config) *config.Database {
	return &cfg.Database
}

func ProvidePostgresOptions(cfg *config.Database) engine.PostgresOptions {
	return engine.PostgresOptions{
		Host: cfg.Host, Port: cfg.Port, User: cfg.User, Password: cfg.Pass,
		Database: cfg.Name, MainSchema: cfg.MainSchema,
	}
}

func ProvideDatabaseService(db *gorm.DB, cfg *config.Config) (*engine.Database, error) {
	if err := db.Use(&scope.Plugin{}); err != nil {
		return nil, err
	}
	return engine.NewDatabaseService(db, cfg.Database.MainSchema), nil
}

func ProvideRedisConfig(cfg *config.Config) *config.Redis {
	return &cfg.Redis
}

func ProvideRedisOptions(cfg *config.Redis) engine.RedisOptions {
	return engine.RedisOptions{
		Host: cfg.Host, Port: cfg.Port, Password: cfg.Pass, DB: cfg.Db,
	}
}

func ProvideRedisService(redisClient *redis.Client, log logger.Interface, cfg *config.Config) *engine.Redis {
	redisOpts := engine.RedisOpts{
		KeyPrefix: cfg.TenantCache.KeyPrefix,
		StatusTTL: 5 * time.Minute,
	}
	return engine.NewRedisService(redisClient, log, redisOpts)
}
