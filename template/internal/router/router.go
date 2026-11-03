package router

import (
	"net/http"

	authmiddleware "gitlab.com/skyrix-lib/auth/middleware"
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

// InitRouter builds the HTTP routes. System routes (/health, /metrics) are
// open; the /api/v1 routes require a valid bearer token checked by auth
// (roles from JWT_ALLOWED_ROLES).
func InitRouter(
	cfg *config.HttpServer,
	globalMw *providers.GlobalMiddleware,
	auth *authmiddleware.AuthMiddleware,
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
		r.Use(auth.Handle)
		mountExampleRoutes(r, handlers)
	})

	_ = tenantMw
	return r
}
