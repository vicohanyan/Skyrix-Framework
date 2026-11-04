//go:build wireinject
// +build wireinject

package main

import (
	"net/http"

	"gitlab.com/skyrix-lib/framework/validation"
	"gitlab.com/skyrix-lib/template/internal/router"
	"gitlab.com/skyrix-lib/template/providers"

	"github.com/goforj/wire"
)

// httpGraph is the graph of cmd/http/wire.go with tenant resolution:
// RedisSet and TenantSet are added, ProvideTenantAPIMiddleware replaces
// ProvideAPIMiddleware. Everything else is unchanged.
var httpGraph = wire.NewSet(
	// 1. Configuration and logger
	providers.ConfigSet,

	// 2. Infrastructure used by this binary
	providers.PostgresSet,
	providers.RedisSet, // tenant cache

	// 3. Compile-time platform modules
	providers.ProvideHTTPPlatform,

	// 4. Example domain
	providers.DomainProviderSet,

	// 5. App layer
	providers.HandlerProviderSet,
	providers.GlobalMiddlewareProviderSet,
	providers.AuthConfigSet,
	providers.AuthVerifierSet,
	providers.TenantSet,
	// Middleware chain of /api/v1, in order: auth, then tenant.
	providers.ProvideTenantAPIMiddleware,

	// 6. HTTP layer
	validation.NewValidator,
	router.ProviderSet,
	providers.HTTPProviderSet,
)

func buildHTTPApp() (*httpApp, func(), error) {
	// 7. Final app: platform components and the HTTP host in one runtime
	wire.Build(httpGraph, newHTTPApp)
	return nil, nil, nil
}

// buildHTTPHandler builds only the router of the same graph; the fixture
// test serves requests through it without starting the runtime.
func buildHTTPHandler() (http.Handler, func(), error) {
	wire.Build(httpGraph)
	return nil, nil, nil
}
