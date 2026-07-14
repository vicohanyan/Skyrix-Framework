package router

import (
	"net/http"

	"skyrix/internal/config"
	"skyrix/internal/providers"

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
) http.Handler {
	r := chi.NewRouter()

	mountGlobalMiddlewares(r, cfg, globalMw)
	mountSystemRoutes(r)
	r.Route("/api/v1", func(r chi.Router) {
		mountExampleRoutes(r, handlers)
	})

	_ = tenantMw
	return r
}
