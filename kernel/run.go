package kernel

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// DefaultStopTimeout bounds how long Run waits for the kernel to stop.
const DefaultStopTimeout = 10 * time.Second

// RunOption configures Run.
type RunOption func(*runOptions)

type runOptions struct {
	stopTimeout time.Duration
}

// WithStopTimeout sets how long Run waits for the kernel to stop.
func WithStopTimeout(timeout time.Duration) RunOption {
	return func(options *runOptions) {
		if timeout > 0 {
			options.stopTimeout = timeout
		}
	}
}

// Run starts the kernel and blocks until ctx is cancelled (for example by a
// signal) or, when the kernel implements FailureNotifier, until a component
// fails. It then stops the kernel within the stop timeout. Run returns nil
// after a clean shutdown requested through ctx.
func Run(ctx context.Context, kernel Kernel, options ...RunOption) error {
	if kernel == nil {
		return fmt.Errorf("kernel is nil")
	}
	settings := runOptions{stopTimeout: DefaultStopTimeout}
	for _, option := range options {
		option(&settings)
	}

	if err := kernel.Start(ctx); err != nil {
		return fmt.Errorf("start kernel: %w", err)
	}

	var failures <-chan error
	if notifier, ok := kernel.(FailureNotifier); ok {
		failures = notifier.Failed()
	}

	var runErr error
	select {
	case <-ctx.Done():
	case err := <-failures:
		runErr = err
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), settings.stopTimeout)
	defer cancel()
	if err := kernel.Stop(stopCtx); err != nil {
		return errors.Join(runErr, fmt.Errorf("stop kernel: %w", err))
	}
	return runErr
}
