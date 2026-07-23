package providers

import (
	"gitlab.com/skyrix-lib/framework/kernel"
	"gitlab.com/skyrix-lib/framework/logging"
	"gitlab.com/skyrix-lib/template/internal/config"

	"github.com/google/wire"
)

// ProvideLoggerConfig Helper providers to extract specific config structs from the main Config.
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

func ProvideJWTConfig(cfg *config.Config) *config.JWT {
	return &cfg.JWT
}

func ProvidePostgresOptions(cfg *config.Database) kernel.PostgresOptions {
	return kernel.PostgresOptions{
		Host: cfg.Host, Port: cfg.Port, User: cfg.User, Password: cfg.Pass,
		Database: cfg.Name, MainSchema: cfg.MainSchema,
	}
}

func ProvideRedisOptions(cfg *config.Redis) kernel.RedisOptions {
	return kernel.RedisOptions{
		Host: cfg.Host, Port: cfg.Port, Password: cfg.Pass, DB: cfg.Db,
	}
}

// InfrastructureSet provides basic, non-business dependencies.
var InfrastructureSet = wire.NewSet(
	ProvideConfig,
	ProvideLoggerConfig,
	ProvideDatabaseConfig,
	ProvideRedisConfig,
	ProvideHttpServerConfig,
	ProvideJWTConfig,
	ProvideLogger,
	ProvidePostgresOptions,
	ProvideRedisOptions,
	kernel.ProvidePostgres,
	kernel.ProvideRedis,
)

func ProvideConfig() (*config.Config, error) {
	return config.LoadConfig()
}

func ProvideLogger(cfg *config.Config) logger.Interface {
	return logger.NewLogger(cfg.Logger)
}
