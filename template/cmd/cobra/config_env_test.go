package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigEnvWithoutSettings(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, key := range []string{"SERVICE_NAME", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASS", "DB_NAME", "REDIS_HOST", "REDIS_PORT", "REDIS_KEY_PREFIX", "JWT_PUBLIC_KEY_PATH"} {
		t.Setenv(key, "")
	}
	t.Setenv("APP_ENV", "missing-environment")
	t.Setenv("LOG_LEVEL", "invalid-log-level")
	t.Setenv("APP_PORT", "invalid-live-port")
	t.Setenv("DB_PASS", "live-secret")
	t.Setenv("DB_SSLMODE", "live-sslmode")
	g := newCountingGraphs()
	defer g.Close()
	output, err := execute(t, newConsole(g.graphs), "config:env")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"APP_ENV", "LOG_LEVEL", "APP_PORT", "DB_PASS", "REDIS_KEY_PREFIX", "JWT_EXPIRE", "TENANT_CACHE_TTL", "MIGRATIONS_DEV_URL"} {
		if !strings.Contains(output, key) {
			t.Errorf("missing %s:\n%s", key, output)
		}
	}
	sslMode := false
	for line := range strings.SplitSeq(output, "\n") {
		if fields := strings.Fields(line); len(fields) == 4 && fields[1] == "DB_SSLMODE" {
			sslMode = fields[0] == "database" && fields[2] == "string" && fields[3] == `"disable"`
		}
	}
	if !sslMode {
		t.Errorf("DB_SSLMODE with default disable missing:\n%s", output)
	}
	for _, value := range []string{"missing-environment", "invalid-log-level", "invalid-live-port", "live-secret", "live-sslmode"} {
		if strings.Contains(output, value) {
			t.Fatalf("current value leaked: %s", value)
		}
	}
	if g.app != 0 || g.database != 0 {
		t.Fatalf("graphs built: app %d database %d", g.app, g.database)
	}
	// A malformed YAML file must have no effect on the reference.
	if err := os.Mkdir("config", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("config", "missing-environment.yaml"), []byte("LOGGER: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	again, err := execute(t, newConsole(g.graphs), "config:env")
	if err != nil || again != output {
		t.Fatalf("YAML affected output: %v\n%s", err, again)
	}
	filtered, err := execute(t, newConsole(g.graphs), "config:env", "--section", "redis")
	if err != nil || !strings.Contains(filtered, "REDIS_PASS") || strings.Contains(filtered, "DB_PASS") {
		t.Fatalf("filter: %v\n%s", err, filtered)
	}
	if _, err := execute(t, newConsole(g.graphs), "config:env", "--section", "unknown"); err == nil {
		t.Fatal("unknown section accepted")
	}
	if g.app != 0 || g.database != 0 {
		t.Fatal("command built graphs")
	}
}
