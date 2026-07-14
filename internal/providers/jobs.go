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

// ProvideRegisteredJobsRegistry registers all known jobs and returns the registry
// through the interface used by the kernel, console command, and admin handlers.
func ProvideRegisteredJobsRegistry(reg *kernelJobs.Registry, all *Jobs) engineJobs.Registry {
	reg.Register(all.SystemPingJob)
	return reg
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

	// registered registry
	ProvideRegisteredJobsRegistry,
)

var ConsoleJobProviderSet = wire.NewSet(
	kernelJobs.NewRegistry,
	jobs.NewSystemPingJob,
	wire.Struct(new(Jobs), "*"),
	ProvideRegisteredJobsRegistry,
)
