package router

import (
	"net/http"

	"gitlab.com/skyrix-lib/framework/httpserver"
	"gitlab.com/skyrix-lib/template/internal/config"
	"gitlab.com/skyrix-lib/template/providers"

	"github.com/go-chi/chi/v5"
)

// InitRouter builds the HTTP routes. System routes (/health, /metrics) are
// open; the /api/v1 routes run behind the api chain, in its order (by default
// bearer-token authentication, see providers.ProvideAPIMiddleware). Every
// element of api must be non-nil.
func InitRouter(
	cfg *config.HttpServer,
	globalMw *providers.GlobalMiddleware,
	api providers.APIMiddleware,
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
		r.Use(api...)
		mountExampleRoutes(r, handlers)
	})
	return r
}
