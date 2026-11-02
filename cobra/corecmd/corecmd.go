// Package corecmd assembles the console commands built into the framework
// (db, jobs:run, make). Application commands are registered next to them on
// the same console.Console.
package corecmd

import (
	"gitlab.com/skyrix-lib/framework/cobra/console"
	"gitlab.com/skyrix-lib/framework/database/dbcmd"
	"gitlab.com/skyrix-lib/framework/database/migrate/migratecmd"
	engineJobs "gitlab.com/skyrix-lib/framework/engine/jobs"
	"gitlab.com/skyrix-lib/framework/engine/jobs/jobscmd"
	"gitlab.com/skyrix-lib/framework/scaffold/makecmd"
)

// Options are what the framework commands get when they run. Describing the
// commands builds none of the graphs.
type Options struct {
	// Migrations are the standalone db migrate* commands; nil leaves them
	// out of the db group.
	Migrations *migratecmd.Commands
	// Database is the database graph of db automigrate and db seed
	// (console.NeedsDatabase).
	Database *console.Graph[*dbcmd.Deps]
	// Jobs is the job registry of jobs:run, usually derived from the
	// application graph (console.NeedsApp).
	Jobs *console.Graph[engineJobs.Registry]
	// Make configures the standalone make commands; nil means
	// makecmd.NewMakeCommand().
	Make *makecmd.MakeCommand
}

// Commands describes the framework commands: the db group (migrate*,
// automigrate, seed), jobs:run and make.
func Commands(options Options) []*console.Command {
	make := options.Make
	if make == nil {
		make = makecmd.NewMakeCommand()
	}
	return []*console.Command{
		dbcmd.Command(options.Migrations, options.Database),
		jobscmd.Command(options.Jobs),
		make.Command(),
	}
}
