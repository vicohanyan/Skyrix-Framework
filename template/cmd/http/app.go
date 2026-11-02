package main

import (
	"gitlab.com/skyrix-lib/framework/httpserver"
	"gitlab.com/skyrix-lib/framework/kernel"
	"gitlab.com/skyrix-lib/framework/logging"
)

// httpApp is the HTTP binary: the platform modules and the HTTP host run by one
// kernel runtime. Platform components start first and stop last.
type httpApp struct {
	Runtime *kernel.Runtime
	Logger  logger.Interface
}

func newHTTPApp(platform *kernel.Platform, host *httpserver.Host, log logger.Interface) *httpApp {
	components := append(platform.Components(), host)
	return &httpApp{Runtime: kernel.NewRuntime(components...), Logger: log}
}
