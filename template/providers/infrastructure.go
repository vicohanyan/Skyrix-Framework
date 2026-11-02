package providers

import (
	"gitlab.com/skyrix-lib/framework/logger"
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

func ProvideLogger(cfg *config.Config) (logger.Interface, func()) {
	return logger.NewLogger(cfg.Logger)
}

// ProvideLoggerConfig Helper providers to extract specific config structs from the main Config.
func ProvideLoggerConfig(cfg *config.Config) *config.Logger {
	return &cfg.Logger
}

func ProvideHttpServerConfig(cfg *config.Config) (*config.HttpServer, error) {
	err := cfg.LoadSection("http", []string{"HTTP_SERVER"}, &cfg.HttpServer)
	return &cfg.HttpServer, err
}

func ProvideJWTConfig(cfg *config.Config) (*config.JWT, error) {
	err := cfg.LoadSection("auth", []string{"JWT"}, &cfg.JWT)
	return &cfg.JWT, err
}
