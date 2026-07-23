package router

import (
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
) http.Handler {
	return InitRouter(cfg, globalMw, tenantMw, handlers)
}

var ProviderSet = wire.NewSet(
	// Default tenant middleware (noop)
	NewNoopTenantMiddleware,
	wire.Bind(new(TenantMiddleware), new(*NoopTenantMiddleware)),

	ProvideRouter,
)
