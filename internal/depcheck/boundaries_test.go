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
