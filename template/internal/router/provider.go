package router

import (
	"net/http"

	authmiddleware "gitlab.com/skyrix-lib/framework/extensions/auth/middleware"
	"gitlab.com/skyrix-lib/framework/httpserver"
	"gitlab.com/skyrix-lib/template/internal/config"
	"gitlab.com/skyrix-lib/template/providers"

	"github.com/goforj/wire"
)

func ProvideRouter(
	cfg *config.HttpServer,
	globalMw *providers.GlobalMiddleware,
	auth *authmiddleware.AuthMiddleware,
	tenantMw TenantMiddleware,
	handlers *providers.Handlers,
	healthChecks httpserver.HealthChecks,
) http.Handler {
	return InitRouter(cfg, globalMw, auth, tenantMw, handlers, healthChecks)
}

var ProviderSet = wire.NewSet(
	// Default tenant middleware (noop)
	NewNoopTenantMiddleware,
	wire.Bind(new(TenantMiddleware), new(*NoopTenantMiddleware)),

	ProvideRouter,
)
