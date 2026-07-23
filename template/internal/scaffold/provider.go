// Package scaffold composes framework-internal console commands.
package scaffold

import (
	eventbuscmd "gitlab.com/skyrix-lib/framework/cobra/eventbus"
	"gitlab.com/skyrix-lib/template/internal/database"
	"gitlab.com/skyrix-lib/template/internal/scaffold/db"
	jobcmd "gitlab.com/skyrix-lib/template/internal/scaffold/job"
	makecmd "gitlab.com/skyrix-lib/template/internal/scaffold/make"

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
