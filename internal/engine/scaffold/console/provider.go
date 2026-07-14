// Package console composes the commands exposed by the application CLI.
package console

import (
	"skyrix/internal/commands"
	"skyrix/internal/database"
	"skyrix/internal/engine/scaffold/db"
	eventbuscmd "skyrix/internal/engine/scaffold/eventbus"
	jobcmd "skyrix/internal/engine/scaffold/job"
	makecmd "skyrix/internal/engine/scaffold/make"

	"github.com/google/wire"
	"github.com/spf13/cobra"
)

// Commands contains the Cobra commands registered on the application root.
type Commands struct {
	All []*cobra.Command
}

// NewCommands assembles the application command list.
func NewCommands(
	hello *commands.HelloCommand,
	databaseCommand *db.DBCommand,
	jobsRun *jobcmd.JobsRunCommand,
	eventBus *eventbuscmd.EventBusCommand,
	makeCommand *makecmd.MakeCommand,
) *Commands {
	return &Commands{All: []*cobra.Command{
		hello.ToCobraCommand(),
		databaseCommand.ToCobraCommand(),
		jobsRun.ToCobraCommand(),
		eventBus.ToCobraCommand(),
		makeCommand.ToCobraCommand(),
	}}
}

// ProviderSet contains command constructors used by the console application.
var ProviderSet = wire.NewSet(
	commands.NewHelloCommand,
	database.NewDBSeeder,
	database.NewDBMigrator,
	db.NewDBAutoMigrateCommand,
	db.NewDBSeedCommand,
	db.NewDBCommand,
	jobcmd.NewJobsRunCommand,
	eventbuscmd.NewEventBusCommand,
	makecmd.NewMakeCommand,
	NewCommands,
)
