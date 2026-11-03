package providers

import (
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/skyrix-lib/template/internal/config"
)

func TestProvidePostgresOptionsPassesSSLMode(t *testing.T) {
	for _, mode := range []string{"", "disable", "require", "verify-full"} {
		options := ProvidePostgresOptions(&config.Database{
			Host: "db", Port: 5432, User: "app", Pass: "p", Name: "orders", MainSchema: "core", SSLMode: mode,
		})
		if options.SSLMode != mode || options.Host != "db" || options.Port != 5432 || options.User != "app" ||
			options.Password != "p" || options.Database != "orders" || options.MainSchema != "core" {
			t.Fatalf("options = %+v", options)
		}
		if _, err := options.ConnConfig(); err != nil {
			t.Fatalf("sslmode %q: ConnConfig() = %v", mode, err)
		}
	}
}

// TestDatabaseConsumersShareSSLMode checks that the application graph (also
// used by db automigrate and db seed: ProvideDatabaseConfig and
// ProvidePostgresOptions) and the SQL migration commands receive the same
// connection settings from YAML and environment.
func TestDatabaseConsumersShareSSLMode(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "LOGGER:\n  LOG_OUTPUT: stdout\nDATABASE:\n  DB_HOST: db\n  DB_PORT: 5432\n  DB_NAME: app\n  DB_SSLMODE: verify-full\n"
	if err := os.WriteFile(filepath.Join(dir, "config", "sslmode-test.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("APP_ENV", "sslmode-test")
	t.Setenv("DB_SSLMODE", "")
	if err := os.Unsetenv("DB_SSLMODE"); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"verify-full", "require"} {
		if want == "require" {
			t.Setenv("DB_SSLMODE", want)
		}
		cfg, err := config.LoadConfig()
		if err != nil {
			t.Fatal(err)
		}
		database, err := ProvideDatabaseConfig(cfg)
		if err != nil {
			t.Fatal(err)
		}
		app := ProvidePostgresOptions(database)

		settings, cleanup, err := ProvideMigrationSettings()()
		if err != nil {
			t.Fatal(err)
		}
		cleanup()
		if app.SSLMode != want || settings.Postgres != app {
			t.Fatalf("sslmode %s: app %+v, migrations %+v", want, app, settings.Postgres)
		}
	}
}
