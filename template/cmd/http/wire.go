//go:build wireinject
// +build wireinject

package main

import (
	"gitlab.com/skyrix-lib/framework/validation"
	"gitlab.com/skyrix-lib/template/internal/router"
	"gitlab.com/skyrix-lib/template/providers"

	"github.com/goforj/wire"
)

func buildHTTPApp() (*httpApp, func(), error) {
	wire.Build(
		// 1. Configuration and logger
		providers.ConfigSet,

		// 2. Infrastructure used by this binary
		providers.PostgresSet,

		// 3. Compile-time platform modules
		providers.PlatformSet,

		// 4. Example domain
		providers.DomainProviderSet,

		// 5. App layer
		providers.HandlerProviderSet,
		providers.GlobalMiddlewareProviderSet,
		providers.AuthVerifierSet,

		// 6. HTTP layer
		validation.NewValidator,
		router.ProviderSet,
		providers.HTTPProviderSet,

		// 7. Final app: platform components and the HTTP host in one runtime
		newHTTPApp,
	)
	return nil, nil, nil
}
