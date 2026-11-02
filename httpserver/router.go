package httpserver

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type RouterOptions struct {
	// Timeout bounds each request: the request context is cancelled after
	// it and the client gets 504 if the handler has not written a response.
	// Zero or negative disables the timeout.
	Timeout    time.Duration
	Recover    func(http.Handler) http.Handler
	Decompress func(http.Handler) http.Handler
}

// NewRouter creates the common HTTP router and preserves the framework
// middleware order. Application routes are registered by the caller.
func NewRouter(options RouterOptions) chi.Router {
	r := chi.NewRouter()

	r.Use(chiMiddleware.RequestID)

	// Replacement for deprecated chiMiddleware.RealIP.
	// Safe default: uses only RemoteAddr and does not trust spoofable headers.
	r.Use(chiMiddleware.ClientIPFromRemoteAddr)

	if options.Recover != nil {
		r.Use(options.Recover)
	}
	r.Use(chiMiddleware.Logger)
	if options.Timeout > 0 {
		r.Use(chiMiddleware.Timeout(options.Timeout))
	}

	// Request gzip decompression. Keep this.
	if options.Decompress != nil {
		r.Use(options.Decompress)
	}

	// Response gzip compression.
	r.Use(chiMiddleware.Compress(5, "application/json", "text/plain", "text/html"))

	return r
}

// RegisterSystemRoutes mounts transport-level endpoints shared by services:
// CORS preflight, /health (the checks of the binary's dependencies, see
// HealthHandler) and /metrics.
func RegisterSystemRoutes(r chi.Router, checks HealthChecks) {
	r.Options("/*", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.Method(http.MethodGet, "/health", HealthHandler(checks, DefaultHealthTimeout))

	r.Handle("/metrics", promhttp.Handler())
}
