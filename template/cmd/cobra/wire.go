//go:build wireinject
// +build wireinject

package main

import (
	frameworkcobra "gitlab.com/skyrix-lib/framework/cobra"
	"gitlab.com/skyrix-lib/template/internal/commands"
	"gitlab.com/skyrix-lib/template/internal/scaffold"
	"gitlab.com/skyrix-lib/template/providers"

	"github.com/goforj/wire"
)

func buildConsoleApp() (*frameworkcobra.ConsoleApp, func(), error) {
	wire.Build(
		// 1) Configuration and logger
		providers.ConfigSet,

		// 2) Infrastructure used by the console
		providers.PostgresSet,

		// 3) Compile-time platform modules
		providers.PlatformSet,

		// 4) Console layer
		providers.MigrationSet,
		providers.ConsoleJobProviderSet,
		commands.ProviderSet,
		scaffold.ProviderSet,
		providers.ConsoleProviderSet,

		// 5) Final console app
		frameworkcobra.NewConsoleApp,
	)
	return nil, nil, nil
}

// buildStandaloneCommands builds the commands that run without the application
// container. Building them reads no configuration and opens no connection.
func buildStandaloneCommands() *scaffold.StandaloneCommands {
	wire.Build(
		providers.MigrationSet,
		scaffold.StandaloneProviderSet,
	)
	return nil
}
