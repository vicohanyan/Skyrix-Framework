//go:build wireinject
// +build wireinject

package main

import (
	"gitlab.com/skyrix-lib/template/providers"

	"github.com/goforj/wire"
)

// buildWorkerApp builds the worker graph. It has no HTTP server, no database
// and no Redis; add providers.PostgresSet or providers.RedisSet and the domain
// sets when background components need them, and pass their
// *enginepostgres.Component or *engineredis.Component to newWorkerApp as the
// first runtime components.
func buildWorkerApp() (*workerApp, func(), error) {
	wire.Build(
		// 1. Configuration and logger
		providers.ConfigSet,

		// 2. Compile-time platform modules (platform drivers)
		providers.PlatformSet,

		// 3. Final app: platform components in one runtime
		newWorkerApp,
	)
	return nil, nil, nil
}
