package router

import (
	"net/http"
	"skyrix/internal/config"
	"skyrix/internal/providers"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func mountGlobalMiddlewares(
	r chi.Router,
	cfg *config.HttpServer,
	globalMw *providers.GlobalMiddleware,
) {
	r.Use(chiMiddleware.RequestID)

	// Replacement for deprecated chiMiddleware.RealIP.
	// Safe default: uses only RemoteAddr and does not trust spoofable headers.
	r.Use(chiMiddleware.ClientIPFromRemoteAddr)

	r.Use(globalMw.Recover.Handle)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Timeout(cfg.Timeout))

	// Request gzip decompression. Keep this.
	r.Use(globalMw.GzipDecompress.Handle)

	// Response gzip compression.
	r.Use(chiMiddleware.Compress(5, "application/json", "text/plain", "text/html"))
}

func mountSystemRoutes(r chi.Router) {
	r.Options("/*", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})

	r.Handle("/metrics", promhttp.Handler())
}
