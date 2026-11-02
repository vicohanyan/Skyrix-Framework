package router

import (
	"gitlab.com/skyrix-lib/framework/httpserver"
	"gitlab.com/skyrix-lib/template/internal/config"
	"gitlab.com/skyrix-lib/template/providers"
	"net/http"

	"github.com/goforj/wire"
)

func ProvideRouter(
	cfg *config.HttpServer,
	globalMw *providers.GlobalMiddleware,
	tenantMw TenantMiddleware,
	handlers *providers.Handlers,
	healthChecks httpserver.HealthChecks,
) http.Handler {
	return InitRouter(cfg, globalMw, tenantMw, handlers, healthChecks)
}

var ProviderSet = wire.NewSet(
	// Default tenant middleware (noop)
	NewNoopTenantMiddleware,
	wire.Bind(new(TenantMiddleware), new(*NoopTenantMiddleware)),

	ProvideRouter,
)
