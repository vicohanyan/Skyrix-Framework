// Package commands contains application-specific console commands.
package commands

import (
	"github.com/google/wire"
	"github.com/spf13/cobra"
)

// Commands contains application-specific console commands.
type Commands struct {
	All []*cobra.Command
}

// NewCommands assembles the framework's reference application commands.
func NewCommands(hello *HelloCommand) *Commands {
	return &Commands{All: []*cobra.Command{hello.ToCobraCommand()}}
}

// ProviderSet contains application-specific console command providers.
var ProviderSet = wire.NewSet(NewHelloCommand, NewCommands)
