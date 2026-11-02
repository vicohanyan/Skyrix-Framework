package cobra

import (
	"gitlab.com/skyrix-lib/framework/kernel"

	spfcobra "github.com/spf13/cobra"
)

// CommandFactory builds a fresh console command.
type CommandFactory func() *spfcobra.Command

// CommandProvider is implemented by platform modules that contribute console
// commands. It is optional: modules without commands implement only
// kernel.Module.
type CommandProvider interface {
	Commands() []CommandFactory
}

// ModuleCommands builds the commands of every module that implements
// CommandProvider, in module order. Factories may be called on zero-value
// modules, so commands must not depend on module state.
func ModuleCommands(modules ...kernel.Module) []*spfcobra.Command {
	var commands []*spfcobra.Command
	for _, module := range modules {
		provider, ok := module.(CommandProvider)
		if !ok {
			continue
		}
		for _, factory := range provider.Commands() {
			if factory == nil {
				continue
			}
			if command := factory(); command != nil {
				commands = append(commands, command)
			}
		}
	}
	return commands
}
