package cobra

import (
	"testing"

	"gitlab.com/skyrix-lib/framework/kernel"

	"gitlab.com/skyrix-lib/framework/cobra/console"
)

type componentsOnlyModule struct{ name string }

func (m componentsOnlyModule) Name() string                 { return m.name }
func (componentsOnlyModule) Components() []kernel.Component { return nil }

type commandModule struct {
	componentsOnlyModule
	factories []CommandFactory
}

func (m commandModule) Commands() []CommandFactory { return m.factories }

func TestModuleCommandsCollectsProvidersInOrder(t *testing.T) {
	commands := ModuleCommands(
		commandModule{componentsOnlyModule{"one"}, []CommandFactory{
			func() *console.Command { return &console.Command{Use: "one"} },
			nil,
			func() *console.Command { return nil },
		}},
		componentsOnlyModule{"no-commands"},
		commandModule{componentsOnlyModule{"two"}, []CommandFactory{
			func() *console.Command { return &console.Command{Use: "two"} },
		}},
	)
	if len(commands) != 2 || commands[0].Use != "one" || commands[1].Use != "two" {
		t.Fatalf("commands = %#v", commands)
	}
}
