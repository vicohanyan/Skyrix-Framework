package kernel

import (
	"fmt"
	"net/http"
	"skyrix/internal/config"

	"github.com/google/wire"
)

// ProvideHTTPServer creates a new *http.Server.
// It takes the router (as an http.Handler) and the server config.
func ProvideHTTPServer(handler http.Handler, cfg *config.HttpServer) *http.Server {
	// Construct the address string from host and port
	addr := cfg.Address
	if addr == "localhost" || addr == "" {
		addr = "0.0.0.0" // Default to 0.0.0.0 for container compatibility
	}
	fullAddr := fmt.Sprintf("%s:%d", addr, cfg.Port)

	return &http.Server{
		Addr:    fullAddr,
		Handler: handler,
	}
}

// HTTPProviderSet assembles all components specific to the HTTP server itself,
// including the router and the final http.Server instance.
var HTTPProviderSet = wire.NewSet(
	ProvideHTTPServer, // Needs http.Handler (from RouterProviderSet) and *config.HttpServer
)
