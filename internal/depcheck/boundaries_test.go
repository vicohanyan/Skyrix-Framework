package depcheck_test

import (
	"testing"

	"gitlab.com/skyrix-lib/framework/internal/depcheck"
)

const module = "gitlab.com/skyrix-lib/framework"

// TestCoreDoesNotImportExtensions keeps the dependency direction
// extensions -> engine -> kernel: neither engine/... nor kernel/... may reach
// an extension, directly or through another package.
func TestCoreDoesNotImportExtensions(t *testing.T) {
	extensions := depcheck.HasPrefix(module + "/extensions")
	for _, root := range []string{module + "/engine", module + "/kernel"} {
		t.Run(root, func(t *testing.T) {
			depcheck.Forbid(t, root, extensions)
		})
	}
}

// TestGORMStaysInGormdb keeps GORM in engine/postgres/gormdb: the kernel, the
// engine root and the GORM-free engine packages may not reach gorm.io/...,
// directly or transitively.
func TestGORMStaysInGormdb(t *testing.T) {
	gorm := depcheck.HasPrefix("gorm.io")
	for _, root := range []string{
		module + "/kernel",
		module + "/engine/scope",
		module + "/engine/redis",
		module + "/engine/jobs",
	} {
		t.Run(root, func(t *testing.T) {
			depcheck.Forbid(t, root, gorm)
		})
	}
	for _, path := range []string{
		module + "/engine",
		module + "/engine/postgres",
	} {
		t.Run(path, func(t *testing.T) {
			depcheck.ForbidPackage(t, path, gorm)
		})
	}
}

// Kernel is also foundation, but its existing standard-library-only test is
// stricter. Keep that rule in kernel/deps_test.go rather than duplicating it.
var foundationForbidden = depcheck.HasPrefix(
	module+"/engine",
	module+"/extensions",
	module+"/httpserver",
	module+"/cobra",
	module+"/database",
	module+"/scaffold",
	module+"/cmd",
	"gitlab.com/skyrix-lib/template",
	"gitlab.com/skyrix-lib/eventbus",
	"github.com/jackc/pgx",
	"github.com/redis/go-redis",
	"gorm.io",
	"github.com/nats-io",
)

// TestFoundationDoesNotImportInfrastructure extends the former config boundary
// to logger and to concrete PostgreSQL, Redis, GORM and EventBus/NATS drivers.
func TestFoundationDoesNotImportInfrastructure(t *testing.T) {
	for _, path := range []string{module + "/config", module + "/logger"} {
		t.Run(path, func(t *testing.T) {
			depcheck.Forbid(t, path, foundationForbidden)
		})
	}
}

func TestFoundationForbiddenPaths(t *testing.T) {
	for _, path := range []string{
		module + "/engine", module + "/engine/postgres/gormdb",
		module + "/extensions/auth", module + "/httpserver",
		module + "/cobra/console", module + "/database/migrate",
		module + "/scaffold/makecmd", module + "/cmd/platformgen",
		"gitlab.com/skyrix-lib/template/providers",
		"gitlab.com/skyrix-lib/eventbus/platform",
		"github.com/jackc/pgx/v5/stdlib", "github.com/redis/go-redis/v9",
		"gorm.io/gorm", "gorm.io/driver/postgres", "github.com/nats-io/nats.go",
	} {
		if !foundationForbidden(path) {
			t.Errorf("foundation must reject %s", path)
		}
	}
	for _, path := range []string{
		"log/slog", "go.yaml.in/yaml/v3", module + "/kernel/jobs",
		module + "/config", module + "/logger",
		module + "/engineered", "gitlab.com/skyrix-lib/eventbus-example",
		"gitlab.com/skyrix-lib/template-example",
	} {
		if foundationForbidden(path) {
			t.Errorf("unexpected forbidden path %s", path)
		}
	}
}
