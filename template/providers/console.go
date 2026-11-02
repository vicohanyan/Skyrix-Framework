package providers

import (
	frameworkcobra "gitlab.com/skyrix-lib/framework/cobra"
	"gitlab.com/skyrix-lib/framework/cobra/corecmd"
	"gitlab.com/skyrix-lib/framework/database/dbcmd"
	"gitlab.com/skyrix-lib/framework/engine/jobs/jobscmd"
	"gitlab.com/skyrix-lib/framework/kernel"
	"gitlab.com/skyrix-lib/framework/scaffold/makecmd"
	"gitlab.com/skyrix-lib/template/internal/commands"
	"gitlab.com/skyrix-lib/template/internal/database"

	"github.com/goforj/wire"
)

// ProvideCobraCommandGroups registers the framework commands (built into the
// framework: db, jobs:run, make, platform module commands) and the
// application commands (internal/commands).
func ProvideCobraCommandGroups(
	application *commands.Commands,
	framework *corecmd.Commands,
	platform *kernel.Platform,
) *frameworkcobra.CommandGroups {
	return &frameworkcobra.CommandGroups{
		Application: application.All,
		Framework:   append(frameworkcobra.ModuleCommands(platform.Modules()...), framework.All...),
	}
}

// CoreCommandSet provides the framework commands of the full console. It
// needs MigrationSet, PostgresSet and the jobs registry.
var CoreCommandSet = wire.NewSet(
	database.ProviderSet,
	dbcmd.New,
	jobscmd.NewJobsRunCommand,
	makecmd.NewMakeCommand,
	corecmd.New,
)

// StandaloneCommandSet provides the framework commands that run without the
// application container. It needs MigrationSet.
var StandaloneCommandSet = wire.NewSet(
	makecmd.NewMakeCommand,
	corecmd.NewStandalone,
)

var ConsoleProviderSet = wire.NewSet(ProvideCobraCommandGroups)
