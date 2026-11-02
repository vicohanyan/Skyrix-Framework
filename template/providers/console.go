package providers

import (
	frameworkcobra "gitlab.com/skyrix-lib/framework/cobra"
	"gitlab.com/skyrix-lib/framework/kernel"
	"gitlab.com/skyrix-lib/template/internal/commands"
	"gitlab.com/skyrix-lib/template/internal/scaffold"

	"github.com/goforj/wire"
)

func ProvideCobraCommandGroups(
	application *commands.Commands,
	framework *scaffold.Commands,
	platform *kernel.Platform,
) *frameworkcobra.CommandGroups {
	return &frameworkcobra.CommandGroups{
		Application: application.All,
		Framework:   append(frameworkcobra.ModuleCommands(platform.Modules()...), framework.All...),
	}
}

var ConsoleProviderSet = wire.NewSet(ProvideCobraCommandGroups)
