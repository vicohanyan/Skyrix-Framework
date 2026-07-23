//go:build wireinject
// +build wireinject

package main

import (
	"gitlab.com/skyrix-lib/framework/httpserver"
	"gitlab.com/skyrix-lib/framework/validation"
	"gitlab.com/skyrix-lib/template/internal/router"
	"gitlab.com/skyrix-lib/template/providers"

	"github.com/google/wire"
)

func buildHTTPApp() (*httpserver.HTTPApp, func(), error) {
	wire.Build(
		// 1. Microkernel bootstrap (config, logger, raw db/redis)
		providers.InfrastructureSet,

		// 2. Engine (wrap raw db/redis → Database, Cache)
		providers.EngineProviderSet,

		// 3. Build Kernel AFTER engine
		providers.KernelProviderSet,

		// 4. Platform policies
		providers.PlatformProviderSet,

		// 5. Example domain
		providers.DomainProviderSet,

		// 6. App layer
		providers.HandlerProviderSet,
		providers.JobProviderSet,
		providers.GlobalMiddlewareProviderSet,

		// 7. HTTP layer
		validation.NewValidator,
		router.ProviderSet,
		providers.HTTPProviderSet,

		// 8. Final app
		httpserver.NewHTTPApp,
	)
	return nil, nil, nil
}
