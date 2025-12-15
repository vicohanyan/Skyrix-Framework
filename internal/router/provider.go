package router

import (
	"net/http"
	"skyrix/internal/config"
	"skyrix/internal/providers"

	"github.com/google/wire"
)

func ProvideRouter(
	cfg *config.HttpServer,
	globalMw *providers.GlobalMiddleware,
	tenantMw TenantMiddleware,
	handlers *providers.Handlers,
) http.Handler {
	return InitRouter(cfg, globalMw, tenantMw, handlers)
}

var ProviderSet = wire.NewSet(
	// Default tenant middleware (noop)
	NewNoopTenantMiddleware,
	wire.Bind(new(TenantMiddleware), new(*NoopTenantMiddleware)),

	ProvideRouter,
)
