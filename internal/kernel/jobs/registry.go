package jobs

import (
	"context"
	"fmt"
	"sort"
	"sync"

	engineJobs "skyrix/internal/engine/jobs"
	"skyrix/internal/logger"
)

type Registry struct {
	log logger.Interface

	mu   sync.RWMutex
	jobs map[string]engineJobs.Job
}

func NewRegistry(log logger.Interface) *Registry {
	return &Registry{
		log:  log,
		jobs: make(map[string]engineJobs.Job),
	}
}

func (r *Registry) Register(job engineJobs.Job) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := job.Name()
	if name == "" {
		panic("job name is empty")
	}
	if _, exists := r.jobs[name]; exists {
		panic("job already registered: " + name)
	}
	r.jobs[name] = job
	r.log.Info("job registered", "name", name)
}

func (r *Registry) Get(name string) (engineJobs.Job, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	j, ok := r.jobs[name]
	return j, ok
}

func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]string, 0, len(r.jobs))
	for name := range r.jobs {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func (r *Registry) Run(ctx context.Context, name string, args map[string]any) error {
	j, ok := r.Get(name)
	if !ok {
		return fmt.Errorf("job not found: %s", name)
	}
	return engineJobs.ExecuteJob(ctx, j, r.log, args)
}

var _ engineJobs.Registry = (*Registry)(nil)
