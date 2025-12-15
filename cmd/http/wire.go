//go:build wireinject
// +build wireinject

package main

import (
	"skyrix/internal/engine"
	"skyrix/internal/kernel"
	"skyrix/internal/providers"
	"skyrix/internal/router"
	"skyrix/internal/validation"

	"github.com/google/wire"
)

func buildHTTPApp() (*kernel.HTTPApp, func(), error) {
	wire.Build(
		// 1. Microkernel bootstrap (config, logger, raw db/redis)
		kernel.ProviderSet,

		// 2. Engine (wrap raw db/redis → Database, Cache)
		engine.ProviderSet,

		// 3. Build Kernel AFTER engine
		kernel.NewKernel,

		// 4. Platform policies
		providers.PlatformProviderSet,

		// 5. Business domains
		providers.DomainProviderSet,

		// 6. App layer
		providers.HandlerProviderSet,
		providers.JobProviderSet,
		providers.GlobalMiddlewareProviderSet,

		// 7. HTTP layer
		validation.NewValidator,
		router.ProviderSet,
		kernel.HTTPProviderSet,

		// 8. Final app
		kernel.NewHTTPApp,
	)
	return nil, nil, nil
}
