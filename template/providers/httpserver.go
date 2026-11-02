package providers

import (
	enginepostgres "gitlab.com/skyrix-lib/framework/engine/postgres"
	"gitlab.com/skyrix-lib/framework/httpserver"
	"gitlab.com/skyrix-lib/template/internal/config"

	"github.com/goforj/wire"
)

func ProvideHTTPServerOptions(cfg *config.HttpServer) httpserver.Options {
	return httpserver.Options{Address: cfg.Address, Port: cfg.Port}
}

// ProvideHealthChecks lists the dependencies checked by GET /health. Add the
// component of every infrastructure set the binary includes, for example
// {Name: "redis", Checker: redisComponent} with RedisSet.
func ProvideHealthChecks(database *enginepostgres.Component) httpserver.HealthChecks {
	return httpserver.HealthChecks{
		{Name: "postgres", Checker: database},
	}
}

// HTTPProviderSet assembles all components specific to the HTTP server itself,
// including the router and the final http.Server instance.
var HTTPProviderSet = wire.NewSet(
	ProvideHTTPServerOptions,
	ProvideHealthChecks,
	httpserver.NewServer,
	httpserver.NewHost,
)
