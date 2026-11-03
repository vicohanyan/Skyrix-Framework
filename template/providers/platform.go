package providers

import (
	"gitlab.com/skyrix-lib/framework/kernel"
)

//go:generate go run gitlab.com/skyrix-lib/framework/cmd/platformgen

// Directives generate independent config adapters, not the binary composition.
// Several driver sets can live in this package. Select the required sets in
// each binary's wire.Build and request their typed modules in its provider below.
//
// To connect a platform driver, for example EventBus (NATS):
//  1. go get gitlab.com/skyrix-lib/eventbus@<version>
//  2. configure the driver as its documentation describes (see also
//     "Platform modules" in the framework ARCHITECTURE.md; the driver
//     configuration mechanism is being revised);
//  3. uncomment the driver's directive below; select EventbusplatformSet in the
//     chosen Wire graphs and add *eventbus.Module to their platform providers;
//     select console commands separately in ProvideConsolePlatformCommands;
//  4. go generate ./providers && go tool wire gen ./cmd/http ./cmd/cobra ./cmd/worker
//
// //skyrix:platform gitlab.com/skyrix-lib/eventbus/platform config=EventBus

// ProvideHTTPPlatform lists HTTP modules in lifecycle order. Driver config and
// components are built only when requested by this graph.
func ProvideHTTPPlatform() (*kernel.Platform, error) {
	return kernel.NewPlatform()
}

// ProvideWorkerPlatform selects the worker's modules independently of HTTP.
func ProvideWorkerPlatform() (*kernel.Platform, error) {
	return kernel.NewPlatform()
}
