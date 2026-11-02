package main

import (
	"gitlab.com/skyrix-lib/framework/kernel"
	"gitlab.com/skyrix-lib/framework/logger"
)

// workerApp is the background binary: no HTTP, only platform modules and
// background components run by one kernel runtime. Components start from the
// root of the dependency graph to the leaves and stop in reverse: engine
// infrastructure, platform modules, then the components that serve the
// domains (event subscribers, schedulers). Without them the runtime is empty:
// the worker starts, waits for a signal and stops.
type workerApp struct {
	Runtime *kernel.Runtime
	Logger  logger.Interface
}

func newWorkerApp(platform *kernel.Platform, log logger.Interface) *workerApp {
	return &workerApp{Runtime: kernel.NewRuntime(platform.Components()...), Logger: log}
}
