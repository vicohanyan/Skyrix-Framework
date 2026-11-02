package jobs

import (
	"context"
	"fmt"
	"sort"
	"sync"

	kerneljobs "gitlab.com/skyrix-lib/framework/kernel/jobs"
	"gitlab.com/skyrix-lib/framework/logging"
)

type RuntimeRegistry struct {
	log logger.Interface

	mu   sync.RWMutex
	jobs map[string]Job
}

func NewRegistry(log logger.Interface) *RuntimeRegistry {
	return &RuntimeRegistry{
		log:  log,
		jobs: make(map[string]Job),
	}
}

func (r *RuntimeRegistry) Register(job Job) {
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

func (r *RuntimeRegistry) Get(name string) (Job, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	j, ok := r.jobs[name]
	return j, ok
}

func (r *RuntimeRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]string, 0, len(r.jobs))
	for name := range r.jobs {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func (r *RuntimeRegistry) Run(ctx context.Context, name string, args map[string]any) error {
	j, ok := r.Get(name)
	if !ok {
		return fmt.Errorf("job not found: %s", name)
	}
	return ExecuteJob(ctx, j, r.log, args)
}

var _ kerneljobs.Registry = (*RuntimeRegistry)(nil)
