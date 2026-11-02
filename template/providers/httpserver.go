package providers

import (
	"gitlab.com/skyrix-lib/framework/httpserver"
	"gitlab.com/skyrix-lib/template/internal/config"

	"github.com/goforj/wire"
)

func ProvideHTTPServerOptions(cfg *config.HttpServer) httpserver.Options {
	return httpserver.Options{Address: cfg.Address, Port: cfg.Port}
}

// HTTPProviderSet assembles all components specific to the HTTP server itself,
// including the router and the final http.Server instance.
var HTTPProviderSet = wire.NewSet(
	ProvideHTTPServerOptions,
	httpserver.NewServer,
	httpserver.NewHost,
)
