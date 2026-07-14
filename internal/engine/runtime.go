package engine

import (
	"context"
	"errors"
	"fmt"
)

type Platform interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type Runtime struct {
	components []Platform
}

func NewRuntime(components ...Platform) *Runtime {
	return &Runtime{
		components: components,
	}
}

func (r *Runtime) Start(ctx context.Context) error {
	if r == nil {
		return fmt.Errorf("runtime is not configured")
	}

	started := make([]Platform, 0, len(r.components))

	for index, component := range r.components {
		if component == nil {
			continue
		}
		if err := component.Start(ctx); err != nil {
			rollbackErr := stopComponents(context.Background(), started)
			if rollbackErr != nil {
				return errors.Join(
					fmt.Errorf("start component %d: %w", index, err),
					fmt.Errorf("rollback started components: %w", rollbackErr),
				)
			}

			return fmt.Errorf("start component %d: %w", index, err)
		}

		started = append(started, component)
	}

	return nil
}

func (r *Runtime) Stop(ctx context.Context) error {
	if r == nil {
		return nil
	}

	return stopComponents(ctx, r.components)
}

func stopComponents(ctx context.Context, components []Platform) error {
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
