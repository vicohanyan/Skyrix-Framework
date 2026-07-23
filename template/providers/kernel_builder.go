package providers

import (
	"gitlab.com/skyrix-lib/framework/engine"
	engineJobs "gitlab.com/skyrix-lib/framework/engine/jobs"
	"gitlab.com/skyrix-lib/framework/kernel"
	logging "gitlab.com/skyrix-lib/framework/logging"
	"gitlab.com/skyrix-lib/template/internal/config"

	"github.com/google/wire"
)

func ProvideKernel(
	cfg *config.Config,
	log logging.Interface,
	db *engine.Database,
	cache engine.Cache,
	jobs engineJobs.Registry,
) *kernel.Kernel {
	return kernel.NewKernel(cfg, log, db, cache, jobs)
}

var KernelProviderSet = wire.NewSet(ProvideKernel)
