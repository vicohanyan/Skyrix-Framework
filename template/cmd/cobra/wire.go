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
		// 1) Bootstrap (config, logger, raw db/redis)
		providers.InfrastructureSet,

		// 2) Engine (db/redis wrappers)
		providers.EngineProviderSet,

		// 3) Build the Kernel
		providers.KernelProviderSet,

		// 4) Console layer
		providers.ConsoleJobProviderSet,
		commands.ProviderSet,
		scaffold.ProviderSet,
		providers.ConsoleProviderSet,

		// 5) Final console app
		frameworkcobra.NewConsoleApp,
	)
	return nil, nil, nil
}
