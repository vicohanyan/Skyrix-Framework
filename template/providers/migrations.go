package providers

import (
	"io/fs"

	"gitlab.com/skyrix-lib/framework/database/migrate/migratecmd"
	"gitlab.com/skyrix-lib/framework/logger"
	"gitlab.com/skyrix-lib/template/internal/config"
	"gitlab.com/skyrix-lib/template/migrations"
)

// ProvideMigrationSettings defers configuration loading until a migration
// command runs, so building the commands and showing help need no config.
func ProvideMigrationSettings() migratecmd.SettingsLoader {
	return func() (migratecmd.Settings, error) {
		cfg, err := config.LoadConfig()
		if err != nil {
			return migratecmd.Settings{}, err
		}
		return migratecmd.Settings{
			Postgres:   ProvidePostgresOptions(&cfg.Database),
			AppVersion: cfg.Version,
			Logger:     logger.NewLogger(cfg.Logger),
			AtlasEnv:   cfg.Env,
		}, nil
	}
}

// ProvideMigrationSource returns the embedded SQL migrations.
func ProvideMigrationSource() fs.FS {
	return migrations.FS
}

// ProvideMigrationCommands returns the standalone db migrate* commands for the
// embedded migrations. Building them reads no configuration and opens no
// connection: the settings are loaded when a migration command runs.
func ProvideMigrationCommands() *migratecmd.Commands {
	return migratecmd.New(ProvideMigrationSettings(), ProvideMigrationSource())
}
