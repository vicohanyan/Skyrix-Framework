//go:build wireinject
// +build wireinject

package main

import (
	natsv1 "gitlab.com/skyrix-lib/template/internal/transport/nats/v1"
	"gitlab.com/skyrix-lib/template/providers"

	"github.com/goforj/wire"
)

// buildWorkerApp builds the worker graph. It has no HTTP server and no
// database; add providers.PostgresSet or providers.RedisSet and the domain
// sets when subscribers need them.
func buildWorkerApp() (*workerApp, func(), error) {
	wire.Build(
		// 1. Configuration and logger
		providers.ConfigSet,

		// 2. Compile-time platform modules (EventBus)
		providers.PlatformSet,

		// 3. Event subscribers
		natsv1.ProviderSet,

		// 4. Final app: platform components and subscribers in one runtime
		newWorkerApp,
	)
	return nil, nil, nil
}
