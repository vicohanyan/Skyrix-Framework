package router

import (
	"net/http"

	"gitlab.com/skyrix-lib/framework/httpserver"
	"gitlab.com/skyrix-lib/template/internal/config"
	"gitlab.com/skyrix-lib/template/providers"

	"github.com/go-chi/chi/v5"
)

// TenantMiddleware is a minimal interface required by the router.
// Real tenant middleware and noop middleware both implement it.
type TenantMiddleware interface {
	// Wrap applies tenant logic around the router or does nothing in noop.
	Wrap(next http.Handler) http.Handler
}

func InitRouter(
	cfg *config.HttpServer,
	globalMw *providers.GlobalMiddleware,
	tenantMw TenantMiddleware,
	handlers *providers.Handlers,
	healthChecks httpserver.HealthChecks,
) http.Handler {
	r := httpserver.NewRouter(httpserver.RouterOptions{
		Timeout:    cfg.Timeout,
		Recover:    globalMw.Recover.Handle,
		Decompress: globalMw.GzipDecompress.Handle,
	})

	httpserver.RegisterSystemRoutes(r, healthChecks)
	r.Route("/api/v1", func(r chi.Router) {
		mountExampleRoutes(r, handlers)
	})

	_ = tenantMw
	return r
}
