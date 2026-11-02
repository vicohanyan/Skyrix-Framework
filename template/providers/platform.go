package providers

//go:generate go run gitlab.com/skyrix-lib/framework/cmd/platformgen

// Platform modules (platform drivers) compiled into the binaries are declared
// with //skyrix:platform directives below; go generate writes
// platform_gen.go from them. Without directives PlatformSet is empty.
//
// To connect a platform driver, for example EventBus (NATS):
//  1. go get gitlab.com/skyrix-lib/eventbus@<version>
//  2. configure the driver as its documentation describes (see also
//     "Platform modules" in the framework ARCHITECTURE.md; the driver
//     configuration mechanism is being revised);
//  3. uncomment the driver's directive below;
//  4. go generate ./providers && go tool wire gen ./cmd/http ./cmd/cobra ./cmd/worker
//
// //skyrix:platform gitlab.com/skyrix-lib/eventbus/platform config=EventBus
