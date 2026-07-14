// Package scaffold composes framework-internal console commands.
package scaffold

import (
	"skyrix/internal/database"
	"skyrix/internal/engine/scaffold/db"
	eventbuscmd "skyrix/internal/engine/scaffold/eventbus"
	jobcmd "skyrix/internal/engine/scaffold/job"
	makecmd "skyrix/internal/engine/scaffold/make"

	"github.com/google/wire"
	"github.com/spf13/cobra"
)

// Commands contains framework-internal scaffold and maintenance commands.
type Commands struct {
	All []*cobra.Command
}

// NewCommands assembles framework-internal console commands.
func NewCommands(
	databaseCommand *db.DBCommand,
	jobsRun *jobcmd.JobsRunCommand,
	eventBus *eventbuscmd.EventBusCommand,
	makeCommand *makecmd.MakeCommand,
) *Commands {
	return &Commands{All: []*cobra.Command{
		databaseCommand.ToCobraCommand(),
		jobsRun.ToCobraCommand(),
		eventBus.ToCobraCommand(),
		makeCommand.ToCobraCommand(),
	}}
}

// ProviderSet contains framework-internal console command providers.
var ProviderSet = wire.NewSet(
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
