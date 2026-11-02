package config

import (
	"slices"
	"testing"

	"github.com/ilyakaznacheev/cleanenv"
)

func TestJWTAllowedRolesFromYAMLAndEnv(t *testing.T) {
	for _, file := range []string{"../../config/local.yaml", "../../config/prod.yaml"} {
		var cfg Config
		if err := cleanenv.ReadConfig(file, &cfg); err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if !slices.Equal(cfg.JWT.AllowedRoles, []string{"customer"}) {
			t.Fatalf("%s: JWT_ALLOWED_ROLES = %v, want [customer]", file, cfg.JWT.AllowedRoles)
		}
		if cfg.Redis.KeyPrefix == "" {
			t.Fatalf("%s: REDIS_KEY_PREFIX is empty", file)
		}
	}

	t.Setenv("JWT_ALLOWED_ROLES", "customer, store_manager")
	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		t.Fatal(err)
	}
	roles := cfg.JWT.Roles()
	if len(roles) != 2 || roles[0] != "customer" || roles[1] != "store_manager" {
		t.Fatalf("roles from env = %v", roles)
	}
}
