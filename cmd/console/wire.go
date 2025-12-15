//go:build wireinject
// +build wireinject

package main

import (
	"skyrix/internal/engine"
	"skyrix/internal/kernel"
	"skyrix/internal/providers"

	"github.com/google/wire"
)

func buildConsoleApp() (*kernel.ConsoleApp, func(), error) {
	wire.Build(
		// 1) Bootstrap (config, logger, raw db/redis)
		kernel.ProviderSet,

		// 2) Engine (db/redis wrappers)
		engine.ProviderSet,

		// 3) Build the Kernel
		kernel.NewKernel,

		// 4) Console layer
		providers.JobProviderSet,
		providers.CommandProviderSet,

		// 5) Final console app
		kernel.NewConsoleApp,
	)
	return nil, nil, nil
}
