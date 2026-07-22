package kernel

import (
	"context"

	"skyrix/internal/commands"
	"skyrix/internal/engine/scaffold"

	"github.com/spf13/cobra"
)

// ConsoleApp is the final runnable CLI application.
// It wires the Kernel plus the Commands bundle and exposes a single Execute entrypoint.
type ConsoleApp struct {
	Kernel   *Kernel
	Commands *commands.Commands
	Scaffold *scaffold.Commands
}

func NewConsoleApp(
	kernel *Kernel,
	applicationCommands *commands.Commands,
	scaffoldCommands *scaffold.Commands,
) *ConsoleApp {
	return &ConsoleApp{
		Kernel:   kernel,
		Commands: applicationCommands,
		Scaffold: scaffoldCommands,
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

	addApplicationCommands(root, c.Commands)
	addScaffoldCommands(root, c.Scaffold)

	return root
}

func addApplicationCommands(root *cobra.Command, commandSet *commands.Commands) {
	if commandSet == nil {
		return
	}
	for _, command := range commandSet.All {
		if command != nil {
			root.AddCommand(command)
		}
	}
}

func addScaffoldCommands(root *cobra.Command, commandSet *scaffold.Commands) {
	if commandSet == nil {
		return
	}
	for _, command := range commandSet.All {
		if command != nil {
			root.AddCommand(command)
		}
	}
}
