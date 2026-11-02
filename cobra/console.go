package cobra

import (
	"context"

	"gitlab.com/skyrix-lib/framework/logger"

	spfcobra "github.com/spf13/cobra"
)

type CommandGroups struct {
	Application []*spfcobra.Command
	Framework   []*spfcobra.Command
}

// ConsoleApp is the final runnable CLI application.
// It wires the Kernel plus the Commands bundle and exposes a single Execute entrypoint.
type ConsoleApp struct {
	Logger   logger.Interface
	Commands []*spfcobra.Command
	Scaffold []*spfcobra.Command
}

func NewConsoleApp(
	log logger.Interface,
	groups *CommandGroups,
) *ConsoleApp {
	if groups == nil {
		groups = &CommandGroups{}
	}
	return &ConsoleApp{
		Logger:   log,
		Commands: groups.Application,
		Scaffold: groups.Framework,
	}
}

// Execute builds the root command, attaches context, and runs Cobra.
func (c *ConsoleApp) Execute(ctx context.Context) error {
	root := c.newRootCommand()
	root.SetContext(ctx)
	return root.Execute()
}

// newRootCommand constructs the CLI root command and registers all sub-commands.
func (c *ConsoleApp) newRootCommand() *spfcobra.Command {
	root := &spfcobra.Command{
		Use:   "cobra",
		Short: "Skyrix CLI console built with github.com/spf13/cobra",
		Long:  `Skyrix CLI console. This console is powered by the Cobra library (github.com/spf13/cobra).`,
	}

	addApplicationCommands(root, c.Commands)
	addScaffoldCommands(root, c.Scaffold)

	return root
}

func addApplicationCommands(root *spfcobra.Command, commandSet []*spfcobra.Command) {
	for _, command := range commandSet {
		if command != nil {
			root.AddCommand(command)
		}
	}
}

func addScaffoldCommands(root *spfcobra.Command, commandSet []*spfcobra.Command) {
	for _, command := range commandSet {
		if command != nil {
			root.AddCommand(command)
		}
	}
}
