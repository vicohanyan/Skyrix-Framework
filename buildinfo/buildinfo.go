// Package buildinfo exposes the application version set at build time.
//
// Set it with the linker:
//
//	go build -ldflags "-X gitlab.com/skyrix-lib/framework/buildinfo.Version=v1.2.3" ./cmd/http
package buildinfo

// Version is the application version. It is empty when not set at build time.
var Version string
