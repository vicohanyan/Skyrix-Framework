package main

import (
	"gitlab.com/skyrix-lib/framework/kernel"
	"gitlab.com/skyrix-lib/framework/logger"
	natsv1 "gitlab.com/skyrix-lib/template/internal/transport/nats/v1"
)

// workerApp is the background binary: no HTTP, only platform modules and event
// subscribers run by one kernel runtime. Components start from the root of the
// dependency graph to the leaves and stop in reverse: engine infrastructure
// (none yet), platform modules, then the subscribers that serve the domains.
type workerApp struct {
	Runtime *kernel.Runtime
	Logger  logger.Interface
}

func newWorkerApp(platform *kernel.Platform, subscribers *natsv1.SubscriberGroup, log logger.Interface) *workerApp {
	components := append(platform.Components(), subscribers)
	return &workerApp{Runtime: kernel.NewRuntime(components...), Logger: log}
}
