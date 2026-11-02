package config

import (
	"testing"

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
