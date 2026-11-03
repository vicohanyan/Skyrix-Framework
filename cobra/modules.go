// Package cobra holds the optional console capability of platform modules.
package cobra

import (
	"gitlab.com/skyrix-lib/framework/cobra/console"
	"gitlab.com/skyrix-lib/framework/kernel"
)

// CommandFactory describes a fresh command without reading working config,
// opening connections or building graphs. Use console.With for lazy DI.
type CommandFactory func() *console.Command

type CommandProvider interface{ Commands() []CommandFactory }

// ModuleCommands collects descriptions in module order. Factories must work
// with zero-value modules. Application graphs and command selection stay explicit.
func ModuleCommands(modules ...kernel.Module) []*console.Command {
	var commands []*console.Command
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
