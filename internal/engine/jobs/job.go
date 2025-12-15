package jobs

import "context"

type Job interface {
	Name() string
	RetryCount() int
	Execute(ctx context.Context, args map[string]any) error
}

type Registry interface {
	Register(job Job)
	Get(name string) (Job, bool)
	List() []string
	Run(ctx context.Context, name string, args map[string]any) error
}
