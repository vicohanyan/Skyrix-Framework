package config

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

func TestJWTAllowedRolesFromYAMLAndEnv(t *testing.T) {
	for _, file := range []string{"../../config/local.yaml", "../../config/prod.yaml"} {
		var cfg Config
		if err := cleanenv.ReadConfig(file, &cfg); err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if len(cfg.JWT.Roles()) != 0 {
			t.Fatalf("%s: JWT_ALLOWED_ROLES = %v, want empty", file, cfg.JWT.AllowedRoles)
		}
		if cfg.Redis.KeyPrefix == "" {
			t.Fatalf("%s: REDIS_KEY_PREFIX is empty", file)
		}
	}

	t.Setenv("JWT_ALLOWED_ROLES", "customer, admin")
	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		t.Fatal(err)
	}
	roles := cfg.JWT.Roles()
	if len(roles) != 2 || roles[0] != "customer" || roles[1] != "admin" {
		t.Fatalf("roles from env = %v", roles)
	}
}

func TestHTTPServerTimeoutsFromYAML(t *testing.T) {
	for _, file := range []string{"../../config/local.yaml", "../../config/prod.yaml"} {
		var cfg Config
		if err := cleanenv.ReadConfig(file, &cfg); err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if cfg.HttpServer.Timeout <= 0 || cfg.HttpServer.ReadHeaderTimeout <= 0 {
			t.Fatalf("%s: APP_REQUEST_TIMEOUT = %v, APP_READ_HEADER_TIMEOUT = %v, want both set", file, cfg.HttpServer.Timeout, cfg.HttpServer.ReadHeaderTimeout)
		}
	}
}

func TestSectionsLoadOnlyWhenSelected(t *testing.T) {
	t.Setenv("REDIS_PORT", "invalid-secret")
	t.Setenv("JWT_EXPIRE", "invalid-secret")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Redis.Port != 0 || cfg.JWT.Expiration != 0 {
		t.Fatal("disabled sections loaded")
	}
	if err := cfg.LoadSection("redis", []string{"REDIS"}, &cfg.Redis); err == nil || strings.Contains(err.Error(), "invalid-secret") {
		t.Fatalf("expected safe error, got %v", err)
	}
}

func TestLegacySectionPrecedence(t *testing.T) {
	cfg := &Config{source: []byte("HTTP_SERVER:\n  APP_REQUEST_TIMEOUT: 0s\n  APP_PORT: 0\nJWT:\n  JWT_EXPIRE: 0\n")}
	if err := cfg.LoadSection("http", []string{"HTTP_SERVER"}, &cfg.HttpServer); err != nil {
		t.Fatal(err)
	}
	if cfg.HttpServer.Timeout != 0 || cfg.HttpServer.Port != 0 || cfg.HttpServer.ReadHeaderTimeout != 10*time.Second {
		t.Fatal("legacy defaults overrode YAML")
	}
	t.Setenv("JWT_ALLOWED_ROLES", "customer, admin")
	if err := cfg.LoadSection("auth", []string{"JWT"}, &cfg.JWT); err != nil {
		t.Fatal(err)
	}
	if cfg.JWT.Expiration != 0 || len(cfg.JWT.Roles()) != 2 {
		t.Fatal("legacy settings changed")
	}
	t.Setenv("APP_REQUEST_TIMEOUT", "2s")
	if err := cfg.LoadSection("http", []string{"HTTP_SERVER"}, &cfg.HttpServer); err != nil {
		t.Fatal(err)
	}
	if cfg.HttpServer.Timeout != 2*time.Second {
		t.Fatal("env did not override YAML")
	}
}

func TestMigrationDevURLFromYAMLAndEnv(t *testing.T) {
	cfg := &Config{source: []byte("MIGRATIONS:\n  MIGRATIONS_DEV_URL: docker://postgres/18/yaml\n")}
	if err := cfg.LoadSection("migrations", []string{"MIGRATIONS"}, &cfg.Migrations); err != nil {
		t.Fatal(err)
	}
	if cfg.Migrations.DevURL != "docker://postgres/18/yaml" {
		t.Fatal(cfg.Migrations.DevURL)
	}
	t.Setenv("MIGRATIONS_DEV_URL", "docker://postgres/18/env")
	if err := cfg.LoadSection("migrations", []string{"MIGRATIONS"}, &cfg.Migrations); err != nil {
		t.Fatal(err)
	}
	if cfg.Migrations.DevURL != "docker://postgres/18/env" {
		t.Fatal(cfg.Migrations.DevURL)
	}
}

// unsetEnv removes key for the test and restores it afterwards.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
}

func TestDatabaseSSLModeFromYAMLAndEnv(t *testing.T) {
	unsetEnv(t, "DB_SSLMODE")
	load := func(yaml string) string {
		t.Helper()
		cfg := &Config{source: []byte(yaml)}
		if err := cfg.LoadSection("database", []string{"DATABASE"}, &cfg.Database); err != nil {
			t.Fatal(err)
		}
		return cfg.Database.SSLMode
	}
	if got := load("DATABASE:\n  DB_HOST: db\n"); got != "disable" {
		t.Fatalf("default DB_SSLMODE = %q, want disable", got)
	}
	if got := load(""); got != "disable" {
		t.Fatalf("DB_SSLMODE without YAML = %q, want disable", got)
	}
	if got := load("DATABASE:\n  DB_SSLMODE: verify-full\n"); got != "verify-full" {
		t.Fatalf("YAML DB_SSLMODE = %q", got)
	}
	// An explicit empty value is kept; the engine treats it as disable.
	if got := load("DATABASE:\n  DB_SSLMODE: \"\"\n"); got != "" {
		t.Fatalf("empty YAML DB_SSLMODE = %q, want empty", got)
	}

	t.Setenv("DB_SSLMODE", "require")
	if got := load("DATABASE:\n  DB_SSLMODE: verify-full\n"); got != "require" {
		t.Fatalf("env did not override YAML: %q", got)
	}
	t.Setenv("DB_SSLMODE", "")
	if got := load("DATABASE:\n  DB_SSLMODE: verify-full\n"); got != "" {
		t.Fatalf("empty env DB_SSLMODE = %q, want empty", got)
	}
}

func TestDatabaseSSLModeInExampleConfigs(t *testing.T) {
	unsetEnv(t, "DB_SSLMODE")
	for _, file := range []string{"../../config/local.yaml", "../../config/prod.yaml"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		cfg := &Config{source: data}
		if err := cfg.LoadSection("database", []string{"DATABASE"}, &cfg.Database); err != nil {
			t.Fatal(err)
		}
		if cfg.Database.SSLMode != "disable" {
			t.Fatalf("%s: DB_SSLMODE = %q, want disable", file, cfg.Database.SSLMode)
		}
	}
}
