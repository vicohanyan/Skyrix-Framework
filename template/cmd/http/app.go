package main

import (
	enginepostgres "gitlab.com/skyrix-lib/framework/engine/postgres"
	"gitlab.com/skyrix-lib/framework/httpserver"
	"gitlab.com/skyrix-lib/framework/kernel"
	"gitlab.com/skyrix-lib/framework/logger"
)

// httpApp is the HTTP binary run by one kernel runtime. Components start from
// the root of the dependency graph to the leaves and stop in reverse:
//  1. engine infrastructure (PostgreSQL),
//  2. platform modules (EventBus, later tenant, auth, ...),
//  3. the HTTP host that serves the domains.
type httpApp struct {
	Runtime *kernel.Runtime
	Logger  logger.Interface
}

func newHTTPApp(
	database *enginepostgres.Component,
	platform *kernel.Platform,
	host *httpserver.Host,
	log logger.Interface,
) *httpApp {
	components := []kernel.Component{database}
	components = append(components, platform.Components()...)
	components = append(components, host)
	return &httpApp{Runtime: kernel.NewRuntime(components...), Logger: log}
}
