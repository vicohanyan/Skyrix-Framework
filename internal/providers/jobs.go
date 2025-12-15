package providers

import (
	engineJobs "skyrix/internal/engine/jobs"
	"skyrix/internal/jobs"
	kernelJobs "skyrix/internal/kernel/jobs"

	"github.com/google/wire"
)

type Jobs struct {
	SystemPingJob *jobs.SystemPingJob
}

// ProvideJobsInit registers all known jobs into the registry.
// It returns a cleanup func to satisfy Wire and to avoid double-providing *Registry.
func ProvideJobsInit(reg *kernelJobs.Registry, all *Jobs) func() {
	reg.Register(all.SystemPingJob)
	return func() {}
}

// JobDomainDepsSet contains ONLY dependencies required by jobs (domain services, publishers, etc).
// Keep it minimal to avoid pulling entire domains into the console app.
var JobDomainDepsSet = wire.NewSet(
// notifications domain
// notifications.NewRepository,
// notifications.NewService,

// outbox/eventbus dependencies if notifications use them
// outbox.NewPublisher,
)

// JobProviderSet wires the jobs subsystem (registry + concrete jobs + init hook).
var JobProviderSet = wire.NewSet(
	JobDomainDepsSet,

	// runtime registry
	kernelJobs.NewRegistry,

	// concrete jobs
	jobs.NewSystemPingJob,

	// bundle
	wire.Struct(new(Jobs), "*"),

	// init hook (doesn't provide *Registry)
	ProvideJobsInit,

	// interface binding
	wire.Bind(new(engineJobs.Registry), new(*kernelJobs.Registry)),
)
