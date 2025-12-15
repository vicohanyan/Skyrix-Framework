package kernel

import (
	"context"

	"skyrix/internal/providers"

	"github.com/spf13/cobra"
)

// ConsoleApp is the final runnable CLI application.
// It wires the Kernel plus the Commands bundle and exposes a single Execute entrypoint.
type ConsoleApp struct {
	Kernel   *Kernel
	Jobs     *providers.Jobs
	Commands *providers.Commands
}

func NewConsoleApp(kernel *Kernel, jobs *providers.Jobs, commands *providers.Commands) *ConsoleApp {
	return &ConsoleApp{
		Kernel:   kernel,
		Jobs:     jobs,
		Commands: commands,
	}
}

// Execute builds the root command, attaches context, and runs Cobra.
func (c *ConsoleApp) Execute(ctx context.Context) error {
	root := c.newRootCommand()
	root.SetContext(ctx)
	return root.Execute()
}

// newRootCommand constructs the CLI root command and registers all sub-commands.
func (c *ConsoleApp) newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "cobra",
		Short: "Skyrix CLI console built with github.com/spf13/cobra",
		Long: `Skyrix CLI console.
This console is powered by the Cobra library (github.com/spf13/cobra).`,
	}

	// Register all commands from the Commands provider.
	if c.Commands != nil && len(c.Commands.All) > 0 {
		for _, cmd := range c.Commands.All {
			if cmd != nil {
				root.AddCommand(cmd)
			}
		}
	}

	return root
}
