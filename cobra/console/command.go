// Package console is the Cobra console host. Every command is described
// once, as a Command: its metadata (use, help texts, arguments, flags) and a
// Runner that receives the command's dependencies only when it runs.
//
// Help and completion are built from the metadata alone, so they never read
// the configuration or open a connection. A command that needs dependencies
// names a lazily built dependency graph (Graph), typically a Wire injector:
// the graph is built just before that command runs, and only the graph it
// needs (the database graph for NeedsDatabase commands, the application
// graph for NeedsApp commands). Standalone commands need no graph at all.
package console

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Handler runs a command with the parsed flags (cmd.Flags()) and the
// positional arguments.
type Handler func(cmd *cobra.Command, args []string) error

// Runner is what a command does when it runs, together with what it needs.
// Create it with Standalone or With; the zero Runner makes the command a
// group that only shows its subcommands.
type Runner struct {
	needs Needs
	run   Handler
}

// Standalone returns a Runner that needs no dependency graph: run reads its
// configuration and opens its connections itself.
func Standalone(run Handler) Runner {
	return Runner{needs: NeedsNothing, run: run}
}

// With returns a Runner that receives the value of graph. The graph is built
// when the command runs, right before run is called, and never for help or
// completion. A build error is returned without calling run.
// The runner needs what the graph provides (Graph.Needs); with a nil graph
// the command fails when it runs.
func With[T any](graph *Graph[T], run func(cmd *cobra.Command, args []string, deps T) error) Runner {
	return Runner{
		needs: graph.Needs(),
		run: func(cmd *cobra.Command, args []string) error {
			deps, err := graph.Get()
			if err != nil {
				return fmt.Errorf("build %s dependencies: %w", graph.Needs(), err)
			}
			return run(cmd, args, deps)
		},
	}
}

// Needs reports what the runner needs.
func (r Runner) Needs() Needs { return r.needs }

// Runnable reports whether the runner runs anything.
func (r Runner) Runnable() bool { return r.run != nil }

// Command describes a console command once: the metadata used for help and
// completion, and the Runner used to execute it.
type Command struct {
	// Use is the one-line usage message; its first word is the command name.
	Use     string
	Aliases []string
	Short   string
	Long    string
	Example string
	// Args validates the positional arguments; nil accepts any.
	Args cobra.PositionalArgs
	// Flags declares the command flags. It is called each time the Cobra
	// command is built; the handler reads the values from cmd.Flags().
	Flags  func(flags *pflag.FlagSet)
	Hidden bool
	// Run runs the command; the zero Runner makes it a group.
	Run Runner
	// Commands are the subcommands.
	Commands []*Command
}

// Name returns the command name: the first word of Use.
func (c *Command) Name() string {
	name, _, _ := strings.Cut(c.Use, " ")
	return name
}

// Needs reports what the command needs when it runs; NeedsNothing for groups.
func (c *Command) Needs() Needs {
	return c.Run.Needs()
}

// Find returns the subcommand at path (names, not aliases), or nil.
func (c *Command) Find(path ...string) *Command {
	current := c
	for _, name := range path {
		var next *Command
		for _, sub := range current.Commands {
			if sub != nil && sub.Name() == name {
				next = sub
				break
			}
		}
		if next == nil {
			return nil
		}
		current = next
	}
	return current
}

// Cobra builds the Cobra command tree from the description. Building it
// reads no configuration and builds no graph; each call returns a new tree.
// A runtime error of the handler does not print the usage; argument and
// flag errors do.
func (c *Command) Cobra() *cobra.Command {
	cmd := &cobra.Command{
		Use:     c.Use,
		Aliases: c.Aliases,
		Short:   c.Short,
		Long:    c.Long,
		Example: c.Example,
		Args:    c.Args,
		Hidden:  c.Hidden,
	}
	if c.Flags != nil {
		c.Flags(cmd.Flags())
	}
	if run := c.Run.run; run != nil {
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return run(cmd, args)
		}
	}
	for _, sub := range c.Commands {
		if sub != nil {
			cmd.AddCommand(sub.Cobra())
		}
	}
	return cmd
}
