package providers

import (
	"gitlab.com/skyrix-lib/framework/database/dbcmd"
	"gitlab.com/skyrix-lib/template/internal/database"

	"github.com/goforj/wire"
)

// DatabaseCommandSet provides the dependencies of db automigrate and db seed
// (dbcmd.Deps): the application models and seeders. It needs ConfigSet and
// PostgresSet.
var DatabaseCommandSet = wire.NewSet(
	database.ProviderSet,
	dbcmd.NewDeps,
)
