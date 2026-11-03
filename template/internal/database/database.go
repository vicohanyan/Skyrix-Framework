// Package database holds the application's data for the framework db
// commands (database/dbcmd): the GORM models synchronised by
// "db automigrate" and the seeders run by "db seed".
package database

import (
	"gitlab.com/skyrix-lib/framework/database/dbcmd"

	"github.com/goforj/wire"
)

// Models returns the GORM entities synchronised by "db automigrate". The
// Template lists none, so no command creates tables on its own. To run the
// example domain against a database, add &exampleEntity.Task{} (package
// internal/domain/example/entity) here or to tools/schemaloader and generate
// a migration (see README "Migrations"). The optional
// gitlab.com/skyrix-lib/tenant model is not listed either: an application
// that enables tenants adds it here (see README "Tenants").
func Models() dbcmd.Models {
	return dbcmd.Models{}
}

// Seeders returns the seeders run by "db seed" and after "db automigrate".
// Add a parameter per seeder to let Wire build it with its dependencies.
func Seeders() dbcmd.Seeders {
	return dbcmd.Seeders{}
}

// ProviderSet provides the models and seeders.
var ProviderSet = wire.NewSet(Models, Seeders)
