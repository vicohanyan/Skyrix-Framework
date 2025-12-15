package kernel

import (
	"skyrix/internal/config"
	"skyrix/internal/engine"
	engineJobs "skyrix/internal/engine/jobs"
	"skyrix/internal/logger"
)

type Kernel struct {
	Config *config.Config
	Logger logger.Interface

	DB    *engine.Database
	Cache engine.Cache
	Jobs  engineJobs.Registry
}

func NewKernel(
	cfg *config.Config,
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
