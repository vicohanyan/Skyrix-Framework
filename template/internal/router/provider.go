package router

import (
	"fmt"
	"net/http"

	"gitlab.com/skyrix-lib/framework/httpserver"
	"gitlab.com/skyrix-lib/template/internal/config"
	"gitlab.com/skyrix-lib/template/providers"

	"github.com/goforj/wire"
)

// ProvideRouter builds the router; a nil entry in the API chain is an error.
func ProvideRouter(
	cfg *config.HttpServer,
	globalMw *providers.GlobalMiddleware,
	api providers.APIMiddleware,
	handlers *providers.Handlers,
	healthChecks httpserver.HealthChecks,
) (http.Handler, error) {
	for i, mw := range api {
		if mw == nil {
			return nil, fmt.Errorf("router: API middleware %d is nil", i)
		}
	}
	return InitRouter(cfg, globalMw, api, handlers, healthChecks), nil
}

var ProviderSet = wire.NewSet(
	ProvideRouter,
)
