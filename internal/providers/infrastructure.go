package providers

import (
	"skyrix/internal/config"
	"skyrix/internal/kernel/db"
	"skyrix/internal/logger"

	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Helper providers to extract specific config structs from the main Config.
func ProvideLoggerConfig(cfg *config.Config) *config.Logger {
	return &cfg.Logger
}

func ProvideDatabaseConfig(cfg *config.Config) *config.Database {
	return &cfg.Database
}

func ProvideRedisConfig(cfg *config.Config) *config.Redis {
	return &cfg.Redis
}

// InfrastructureSet provides basic, non-business dependencies.
var InfrastructureSet = wire.NewSet(
	ProvideConfig,
	ProvideLoggerConfig,
	ProvideDatabaseConfig,
	ProvideRedisConfig,
	ProvideLogger,
	ProvidePostgres,
	ProvideRedis,
)

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
		log.Error("Unable to initialize Redis Client", "error", err)
		return nil, nil, err
	}
	cleanup := func() {
		log.Info("Closing redis client connection")
		_ = client.Close()
	}
	return client, cleanup, nil
}
