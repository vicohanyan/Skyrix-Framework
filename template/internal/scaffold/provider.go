// Package scaffold composes the console commands provided by the framework
// with the application's own database commands.
package scaffold

import (
	"gitlab.com/skyrix-lib/framework/database/migrate/migratecmd"
	"gitlab.com/skyrix-lib/framework/engine/jobs/jobscmd"
	"gitlab.com/skyrix-lib/framework/scaffold/makecmd"
	"gitlab.com/skyrix-lib/template/internal/database"
	"gitlab.com/skyrix-lib/template/internal/scaffold/db"

	"github.com/goforj/wire"
	"github.com/spf13/cobra"
)

// Commands contains the scaffold and maintenance commands of the full console.
type Commands struct {
	All []*cobra.Command
}

// NewCommands assembles the scaffold and maintenance commands.
func NewCommands(
	databaseCommand *db.DBCommand,
	jobsRun *jobscmd.JobsRunCommand,
	makeCommand *makecmd.MakeCommand,
) *Commands {
	return &Commands{All: []*cobra.Command{
		databaseCommand.ToCobraCommand(),
		jobsRun.ToCobraCommand(),
		makeCommand.ToCobraCommand(),
	}}
}

// StandaloneCommands are console commands that run without bootstrapping the
// application container; they open connections only when executed.
type StandaloneCommands struct {
	All []*cobra.Command
}

// NewStandaloneCommands assembles the commands that run without the container.
func NewStandaloneCommands(
	migrations *migratecmd.Commands,
	makeCommand *makecmd.MakeCommand,
) *StandaloneCommands {
	return &StandaloneCommands{All: []*cobra.Command{
		migrations.ToCobraCommand(),
		makeCommand.ToCobraCommand(),
	}}
}

// StandaloneProviderSet contains the providers of StandaloneCommands. It needs
// *migratecmd.Commands (providers.MigrationSet).
var StandaloneProviderSet = wire.NewSet(
	makecmd.NewMakeCommand,
	NewStandaloneCommands,
)

// ProviderSet contains the console command providers. It needs
// *migratecmd.Commands (providers.MigrationSet).
var ProviderSet = wire.NewSet(
	database.NewDBSeeder,
	database.NewDBMigrator,
	db.NewDBAutoMigrateCommand,
	db.NewDBSeedCommand,
	db.NewDBCommand,
	jobscmd.NewJobsRunCommand,
	makecmd.NewMakeCommand,
	NewCommands,
)
