package main

import (
	"gitlab.com/skyrix-lib/framework/kernel"
	"gitlab.com/skyrix-lib/framework/logging"
	natsv1 "gitlab.com/skyrix-lib/template/internal/transport/nats/v1"
)

// workerApp is the background binary: no HTTP, only platform modules and event
// subscribers run by one kernel runtime. Platform components start first and
// stop last.
type workerApp struct {
	Runtime *kernel.Runtime
	Logger  logger.Interface
}

func newWorkerApp(platform *kernel.Platform, subscribers *natsv1.SubscriberGroup, log logger.Interface) *workerApp {
	components := append(platform.Components(), subscribers)
	return &workerApp{Runtime: kernel.NewRuntime(components...), Logger: log}
}
