package config

import (
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
	t.Setenv("TENANT_CACHE_TTL", "invalid-secret")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Redis.Port != 0 || cfg.JWT.Expiration != 0 || cfg.TenantCache.TTL != 0 {
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
