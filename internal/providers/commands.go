package providers

import (
	"skyrix/internal/commands"

	"github.com/google/wire"
	"github.com/spf13/cobra"
)

// Commands is a bundle of all CLI commands exposed by the application.
type Commands struct {
	Hello *commands.HelloCommand

	// All is the final list of cobra commands registered in the root CLI.
	All []*cobra.Command
}

// ProvideCommands assembles the command list.
// Keep this function as the single place that defines command registration order.
func ProvideCommands(hello *commands.HelloCommand) *Commands {
	out := &Commands{
		Hello: hello,
	}
	out.All = []*cobra.Command{
		hello.ToCobraCommand(),
	}
	return out
}

var CommandProviderSet = wire.NewSet(
	commands.NewHelloCommand,
	ProvideCommands,
)
