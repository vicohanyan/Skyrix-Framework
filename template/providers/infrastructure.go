package providers

import (
	"gitlab.com/skyrix-lib/framework/logging"
	"gitlab.com/skyrix-lib/template/internal/config"

	"github.com/goforj/wire"
)

// ConfigSet provides the configuration, its sections and the logger. Every
// binary includes it; infrastructure sets are added per binary.
var ConfigSet = wire.NewSet(
	ProvideConfig,
	ProvideLoggerConfig,
	ProvideHttpServerConfig,
	ProvideJWTConfig,
	ProvideLogger,
)

func ProvideConfig() (*config.Config, error) {
	return config.LoadConfig()
}

func ProvideLogger(cfg *config.Config) logger.Interface {
	return logger.NewLogger(cfg.Logger)
}

// ProvideLoggerConfig Helper providers to extract specific config structs from the main Config.
func ProvideLoggerConfig(cfg *config.Config) *config.Logger {
	return &cfg.Logger
}

func ProvideHttpServerConfig(cfg *config.Config) *config.HttpServer {
	return &cfg.HttpServer
}

func ProvideJWTConfig(cfg *config.Config) *config.JWT {
	return &cfg.JWT
}
