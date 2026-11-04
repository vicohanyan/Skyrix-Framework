// Package cobra holds the optional console capability of platform modules.
package cobra

import (
	spfcobra "github.com/spf13/cobra"
	"gitlab.com/skyrix-lib/framework/kernel"
)

// CommandFactory describes a fresh command without reading working config,
// opening connections or building graphs. Use console.With for lazy DI.
type CommandFactory func() *spfcobra.Command

type CommandProvider interface{ Commands() []CommandFactory }

// ModuleCommands collects descriptions in module order. Factories must work
// with zero-value modules. Application graphs and command selection stay explicit.
func ModuleCommands(modules ...kernel.Module) []*spfcobra.Command {
	var commands []*spfcobra.Command
	for _, module := range modules {
		provider, ok := module.(CommandProvider)
		if !ok {
			continue
		}
		for _, factory := range provider.Commands() {
			if factory != nil {
				if command := factory(); command != nil {
					commands = append(commands, command)
				}
			}
		}
	}
	return commands
}
