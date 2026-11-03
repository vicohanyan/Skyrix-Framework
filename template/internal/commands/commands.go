// Package commands contains the application console commands.
//
// Each command is described once, as a console.Command: metadata (use, help
// texts, arguments, flags) and a handler that receives Deps when the command
// runs. Help and completion are built from the metadata, without Deps. To add
// a command, write its description (see Hello) and list it in All.
package commands

import (
	"gitlab.com/skyrix-lib/framework/logger"

	"gitlab.com/skyrix-lib/framework/cobra/console"
	"gitlab.com/skyrix-lib/framework/database/dbcmd"
	"gitlab.com/skyrix-lib/template/internal/config"

	"github.com/goforj/wire"
)

// Deps are the dependencies of the application commands. cmd/cobra builds
// them with Wire, as part of the application graph, only when an application
// command runs. Add a field for every dependency a command needs.
type Deps struct {
	Config *config.Config
	Logger logger.Interface
}

// All describes every application command.
func All(deps *console.Graph[*Deps]) []*console.Command {
	return []*console.Command{
		Hello(deps),
	}
}

// ProviderSet provides Deps for the application graph.
var ProviderSet = wire.NewSet(wire.Struct(new(Deps), "*"))

// DatabaseAll describes commands that share the existing lazy database graph.
func DatabaseAll(deps *console.Graph[*dbcmd.Deps]) []*console.Command {
	return []*console.Command{}
}
