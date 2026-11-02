package providers

import (
	"io/fs"

	"gitlab.com/skyrix-lib/framework/database/migrate/migratecmd"
	"gitlab.com/skyrix-lib/framework/logging"
	"gitlab.com/skyrix-lib/template/internal/config"
	"gitlab.com/skyrix-lib/template/migrations"

	"github.com/goforj/wire"
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
			Postgres: ProvidePostgresOptions(&cfg.Database),
			Logger:   logger.NewLogger(cfg.Logger),
			AtlasEnv: cfg.Env,
		}, nil
	}
}

// ProvideMigrationSource returns the embedded SQL migrations.
func ProvideMigrationSource() fs.FS {
	return migrations.FS
}

// MigrationSet provides the framework SQL migration commands for the
// embedded migrations.
var MigrationSet = wire.NewSet(
	ProvideMigrationSettings,
	ProvideMigrationSource,
	migratecmd.New,
)
