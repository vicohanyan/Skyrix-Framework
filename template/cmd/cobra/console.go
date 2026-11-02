package main

import (
	"gitlab.com/skyrix-lib/framework/cobra/console"
	"gitlab.com/skyrix-lib/framework/cobra/corecmd"
	"gitlab.com/skyrix-lib/framework/database/dbcmd"
	engineJobs "gitlab.com/skyrix-lib/framework/engine/jobs"
	"gitlab.com/skyrix-lib/template/internal/commands"
	"gitlab.com/skyrix-lib/template/internal/config"
	"gitlab.com/skyrix-lib/template/providers"
)

// appGraph is the application graph of the console (buildApp).
type appGraph struct {
	Commands *commands.Deps
	Jobs     engineJobs.Registry
}

// graphs are the dependency graphs of the console, built on first use.
type graphs struct {
	App      *console.Graph[*appGraph]
	Database *console.Graph[*dbcmd.Deps]
}

// newGraphs returns the console graphs built by the Wire injectors.
func newGraphs() graphs {
	return graphs{
		App:      console.NewGraph(console.NeedsApp, buildApp),
		Database: console.NewGraph(console.NeedsDatabase, buildDatabase),
	}
}

// Close releases the graphs that were built.
func (g graphs) Close() {
	g.App.Close()
	g.Database.Close()
}

// newConsole describes every console command: framework commands (db,
// jobs:run, make), application commands (internal/commands) and platform
// module commands. Describing them builds no graph and opens no connection;
// a command's graph is built only when that command runs:
//   - config:env, make, db migrate* and platform commands are standalone; db migrate*
//     connect only to PostgreSQL;
//   - db automigrate and db seed build the database graph;
//   - jobs:run and the application commands build the application graph.
func newConsole(g graphs) *console.Console {
	app := console.New(corecmd.Commands(corecmd.Options{
		Migrations:     providers.ProvideMigrationCommands(),
		ConfigSections: config.Sections(),
		Database:       g.Database,
		Jobs:           console.Derive(g.App, func(a *appGraph) engineJobs.Registry { return a.Jobs }),
	})...)
	app.Add(commands.All(console.Derive(g.App, func(a *appGraph) *commands.Deps { return a.Commands }))...)
	app.AddCobra(providers.PlatformCommands()...)
	return app
}
