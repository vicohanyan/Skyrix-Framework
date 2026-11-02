//go:build wireinject
// +build wireinject

package main

import (
	"gitlab.com/skyrix-lib/framework/database/dbcmd"
	"gitlab.com/skyrix-lib/template/internal/commands"
	"gitlab.com/skyrix-lib/template/providers"

	"github.com/goforj/wire"
)

// buildApp builds the application graph of the console: the dependencies of
// the application commands and the job registry. It runs only when a command
// that needs the application graph runs (console.NeedsApp).
func buildApp() (*appGraph, func(), error) {
	wire.Build(
		// 1) Configuration and logger
		providers.ConfigSet,

		// 2) Jobs (jobs:run)
		providers.ConsoleJobProviderSet,

		// 3) Application commands
		commands.ProviderSet,

		wire.Struct(new(appGraph), "*"),
	)
	return nil, nil, nil
}

// buildDatabase builds the database graph: configuration, logger, PostgreSQL,
// models and seeders. It runs only for db automigrate and db seed
// (console.NeedsDatabase).
func buildDatabase() (*dbcmd.Deps, func(), error) {
	wire.Build(
		providers.ConfigSet,
		providers.PostgresSet,
		providers.DatabaseCommandSet,
	)
	return nil, nil, nil
}
