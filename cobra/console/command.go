// Package console provides a Cobra host and lazy dependency injection.
package console

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Handler runs a Cobra command after its arguments and flags are validated.
type Handler func(cmd *cobra.Command, args []string) error

// Standalone suppresses usage for runtime errors. Cobra still shows usage for
// invalid arguments and flags, which are checked before the handler runs.
func Standalone(run Handler) Handler {
	return func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return run(cmd, args)
	}
}

// With obtains a graph only when the command runs. Help and completion never
// build dependencies. The application owns graph.Close.
func With[T any](graph *Graph[T], run func(*cobra.Command, []string, T) error) Handler {
	return Standalone(func(cmd *cobra.Command, args []string) error {
		deps, err := graph.Get()
		if err != nil {
			return fmt.Errorf("build command dependencies: %w", err)
		}
		return run(cmd, args, deps)
	})
}
