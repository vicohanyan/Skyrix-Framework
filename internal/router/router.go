package router

import (
	"net/http"
	"skyrix/internal/config"
	"skyrix/internal/providers"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

// TenantMiddleware is a minimal interface required by the router.
// Real tenant middleware and noop middleware both implement it.
type TenantMiddleware interface {
	// Wrap applies tenant logic around the router (or does nothing in noop).
	Wrap(next http.Handler) http.Handler
}

func InitRouter(
	cfg *config.HttpServer,
	globalMw *providers.GlobalMiddleware,
	tenantMw TenantMiddleware,
	handlers *providers.Handlers,
) http.Handler {
	r := chi.NewRouter()

	// ==== Global middleware ====
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(globalMw.Recover.Handle)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Timeout(cfg.Timeout))
	r.Use(globalMw.GzipDecompress.Handle)
	r.Use(chiMiddleware.Compress(5, "application/json", "text/plain", "text/html"))

	// Global OPTIONS responder (handy for preflight)
	r.Options("/*", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// /health
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})

	// ==== Routes ====
	r.Route("/api/v1", func(r chi.Router) {
		// Platform middleware can be applied to a group:

		// Example:
		// r.Post("/subscribers", h.Subscriber.Handle)
	})

	return r
}
