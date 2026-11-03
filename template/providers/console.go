package providers

import (
	"gitlab.com/skyrix-lib/framework/database/dbcmd"
	"gitlab.com/skyrix-lib/template/internal/database"

	"github.com/goforj/wire"
	"github.com/spf13/cobra"
)

// DatabaseCommandSet provides the dependencies of db automigrate and db seed
// (dbcmd.Deps): the application models and seeders. It needs ConfigSet and
// PostgresSet.
var DatabaseCommandSet = wire.NewSet(
	database.ProviderSet,
	dbcmd.NewDeps,
)

// ProvideConsolePlatformCommands explicitly selects driver command factories.
// Add frameworkcobra.ModuleCommands(new(driver.Module)) for selected drivers.
// Factories must work on zero values without loading config, building a graph
// or starting background components merely to register commands or show help.
func ProvideConsolePlatformCommands() []*cobra.Command {
	return nil
}
