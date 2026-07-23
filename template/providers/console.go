package providers

import (
	frameworkcobra "gitlab.com/skyrix-lib/framework/cobra"
	"gitlab.com/skyrix-lib/template/internal/commands"
	"gitlab.com/skyrix-lib/template/internal/scaffold"

	"github.com/google/wire"
)

func ProvideCobraCommandGroups(
	application *commands.Commands,
	framework *scaffold.Commands,
) *frameworkcobra.CommandGroups {
	return &frameworkcobra.CommandGroups{
		Application: application.All,
		Framework:   framework.All,
	}
}

var ConsoleProviderSet = wire.NewSet(ProvideCobraCommandGroups)
