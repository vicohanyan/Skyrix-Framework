package providers

import (
	"skyrix/internal/commands"

	"github.com/google/wire"
	"github.com/spf13/cobra"
)

// Commands contains application-specific console commands.
type Commands struct {
	All []*cobra.Command
}

// NewCommands assembles application-specific console commands.
func NewCommands(hello *commands.HelloCommand) *Commands {
	return &Commands{All: []*cobra.Command{
		hello.ToCobraCommand(),
	}}
}

// ProviderSet contains application-specific console command providers.
var ProviderSet = wire.NewSet(
	commands.NewHelloCommand(),
	NewCommands,
)
