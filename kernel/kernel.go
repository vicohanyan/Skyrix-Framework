package kernel

import (
	"gitlab.com/skyrix-lib/framework/engine"
	engineJobs "gitlab.com/skyrix-lib/framework/engine/jobs"
	"gitlab.com/skyrix-lib/framework/logging"
)

type Kernel struct {
	Config any
	Logger logger.Interface

	DB    *engine.Database
	Cache engine.Cache
	Jobs  engineJobs.Registry
}

func NewKernel(
	cfg any,
	log logger.Interface,
	db *engine.Database,
	cache engine.Cache,
	jobs engineJobs.Registry,
) *Kernel {
	return &Kernel{
		Config: cfg,
		Logger: log,
		DB:     db,
		Cache:  cache,
		Jobs:   jobs,
	}
}
