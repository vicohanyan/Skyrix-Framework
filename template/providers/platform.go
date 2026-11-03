package providers

import (
	"gitlab.com/skyrix-lib/framework/kernel"
)

//go:generate go run gitlab.com/skyrix-lib/framework/cmd/platformgen

// Directives generate independent config adapters, not the binary composition.
// Several driver sets can live in this package. Select the required sets in
// each binary's wire.Build and request their typed modules in its provider below.
//
// To connect a driver, first add its dependency explicitly, then run:
//   go run ./cmd/cobra platform add <driver-package>
// Select the generated set in each required Wire graph, add the typed module
// to its application provider, and select console commands separately.
// Regenerate Wire explicitly with the installed tool.
// EventBus needs the manifest, typed Config/NewConfig and console.Command
// contract described in the framework ARCHITECTURE.md before it can be added.
//
// //skyrix:platform gitlab.com/skyrix-lib/eventbus/platform

// ProvideHTTPPlatform lists HTTP modules in lifecycle order. Driver config and
// components are built only when requested by this graph.
func ProvideHTTPPlatform() (*kernel.Platform, error) {
	return kernel.NewPlatform()
}

// ProvideWorkerPlatform selects the worker's modules independently of HTTP.
func ProvideWorkerPlatform() (*kernel.Platform, error) {
	return kernel.NewPlatform()
}
