// Package corecmd assembles the console commands built into the framework
// (db, jobs:run, make). Application commands are registered separately, as
// cobra.CommandGroups.Application.
package corecmd

import (
	"gitlab.com/skyrix-lib/framework/database/dbcmd"
	"gitlab.com/skyrix-lib/framework/database/migrate/migratecmd"
	"gitlab.com/skyrix-lib/framework/engine/jobs/jobscmd"
	"gitlab.com/skyrix-lib/framework/scaffold/makecmd"

	"github.com/spf13/cobra"
)

// Commands are the framework commands of the full console.
type Commands struct {
	All []*cobra.Command
}

// New assembles the framework commands of the full console.
func New(db *dbcmd.Commands, jobsRun *jobscmd.JobsRunCommand, make *makecmd.MakeCommand) *Commands {
	return &Commands{All: []*cobra.Command{
		db.ToCobraCommand(),
		jobsRun.ToCobraCommand(),
		make.ToCobraCommand(),
	}}
}

// StandaloneCommands are the framework commands that run without the
// application container; they open connections only when executed.
type StandaloneCommands struct {
	All []*cobra.Command
}

// NewStandalone assembles the standalone framework commands.
func NewStandalone(migrations *migratecmd.Commands, make *makecmd.MakeCommand) *StandaloneCommands {
	return &StandaloneCommands{All: []*cobra.Command{
		migrations.ToCobraCommand(),
		make.ToCobraCommand(),
	}}
}
