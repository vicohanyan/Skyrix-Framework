package kernel

import (
	"skyrix/internal/config"
	"skyrix/internal/kernel/db"
	"skyrix/internal/logger"

	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// ProviderSet is the microkernel bootstrap set: config, logger, raw clients.
// App developers should not edit this set in normal cases.
var ProviderSet = wire.NewSet(
	ProvideConfig,

	ProvideLoggerConfig,
	ProvideDatabaseConfig,
	ProvideRedisConfig,
	ProvideHttpServerConfig,

	ProvideLogger,
	ProvidePostgres,
	ProvideRedis,
)

// ---- Config extractors ----

func ProvideLoggerConfig(cfg *config.Config) *config.Logger {
	return &cfg.Logger
}

func ProvideDatabaseConfig(cfg *config.Config) *config.Database {
	return &cfg.Database
}

func ProvideRedisConfig(cfg *config.Config) *config.Redis {
	return &cfg.Redis
}

func ProvideHttpServerConfig(cfg *config.Config) *config.HttpServer {
	return &cfg.HttpServer
}

// ---- Leaf providers ----

func ProvideConfig() (*config.Config, error) {
	return config.LoadConfig()
}

func ProvideLogger(cfg *config.Logger) logger.Interface {
	return logger.NewLogger(cfg.LogLevel, cfg.LogType, cfg.LogFile)
}

func ProvidePostgres(cfg *config.Database, log logger.Interface) (*gorm.DB, func(), error) {
	postgres, err := db.InitPostgres(cfg)
	if err != nil {
		log.Error("Unable to initialize postgres database", "error", err)
		return nil, nil, err
	}
	cleanup := func() {
		sqlDB, err := postgres.DB()
		if err == nil {
			log.Info("Closing postgres database connection")
			_ = sqlDB.Close()
		}
	}
	return postgres, cleanup, nil
}

func ProvideRedis(cfg *config.Redis, log logger.Interface) (*redis.Client, func(), error) {
	client, err := db.InitRedis(cfg)
	if err != nil {
		log.Error("Unable to initialize redis client", "error", err)
		return nil, nil, err
	}
	cleanup := func() {
		log.Info("Closing redis client connection")
		_ = client.Close()
	}
	return client, cleanup, nil
}
