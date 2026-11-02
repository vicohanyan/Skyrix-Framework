package kernel

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Runtime starts components in order and stops them in reverse order. It is
// the Kernel of a binary: every host (HTTP server, subscribers, workers) and
// every platform module contributes components to it.
type Runtime struct {
	components []Component

	mu       sync.Mutex
	failures chan error
	done     chan struct{}
}

// NewRuntime returns a runtime for the components. Nil components are skipped.
func NewRuntime(components ...Component) *Runtime {
	return &Runtime{components: components}
}

// Start starts every component in order. When a component fails to start, the
// components started before it are stopped in reverse order.
func (r *Runtime) Start(ctx context.Context) error {
	if r == nil {
		return fmt.Errorf("runtime is not configured")
	}

	r.mu.Lock()
	r.failures = make(chan error, 1)
	r.done = make(chan struct{})
	failures, done := r.failures, r.done
	r.mu.Unlock()

	started := make([]Component, 0, len(r.components))
	for index, component := range r.components {
		if component == nil {
			continue
		}
		if err := component.Start(ctx); err != nil {
			startErr := fmt.Errorf("start component %d: %w", index, err)
			if rollbackErr := stopComponents(context.Background(), started); rollbackErr != nil {
				return errors.Join(startErr, fmt.Errorf("rollback started components: %w", rollbackErr))
			}
			return startErr
		}
		started = append(started, component)
		if notifier, ok := component.(FailureNotifier); ok {
			go forwardFailure(index, notifier.Failed(), failures, done)
		}
	}
	return nil
}

// Stop stops every component in reverse order and reports all stop errors.
func (r *Runtime) Stop(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	if r.done != nil {
		close(r.done)
		r.done = nil
	}
	r.mu.Unlock()
	return stopComponents(ctx, r.components)
}

// Failed reports the first failure of a started component that implements
// FailureNotifier. It returns nil before Start.
func (r *Runtime) Failed() <-chan error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.failures
}

func forwardFailure(index int, source <-chan error, failures chan<- error, done <-chan struct{}) {
	select {
	case err, ok := <-source:
		if !ok || err == nil {
			return
		}
		select {
		case failures <- fmt.Errorf("component %d failed: %w", index, err):
		default:
		}
	case <-done:
	}
}

func stopComponents(ctx context.Context, components []Component) error {
	var result error
	for i := len(components) - 1; i >= 0; i-- {
		component := components[i]
		if component == nil {
			continue
		}
		if err := component.Stop(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("stop component %d: %w", i, err))
		}
	}
	return result
}

var (
	_ Kernel          = (*Runtime)(nil)
	_ FailureNotifier = (*Runtime)(nil)
)
