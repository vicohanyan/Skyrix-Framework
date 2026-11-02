package migrate_test

import (
	"testing"

	"gitlab.com/skyrix-lib/framework/internal/depcheck"
)

// TestMigrateImportsOnlyStandardLibrary keeps the migration core independent
// of GORM and every other non-standard package; drivers are supplied by the
// application through database/sql. The CLI subpackages (migratecmd, atlas)
// are allowed to use Cobra and the engine.
func TestMigrateImportsOnlyStandardLibrary(t *testing.T) {
	depcheck.CheckPackage(t, "gitlab.com/skyrix-lib/framework/database/migrate", depcheck.StandardLibraryOnly)
}
