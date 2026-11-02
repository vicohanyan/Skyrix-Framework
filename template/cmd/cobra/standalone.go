package main

import (
	"context"
	"io"

	frameworkcobra "gitlab.com/skyrix-lib/framework/cobra"
	"gitlab.com/skyrix-lib/template/providers"
)

// executeStandaloneCommand runs platform commands, make and db migrate*
// without bootstrapping the application container. It reports false when the
// arguments name a command of the full application.
func executeStandaloneCommand(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) (bool, error) {
	commands := append(providers.PlatformCommands(), buildStandaloneCommands().All...)
	return frameworkcobra.ExecuteStandalone(ctx, args, stdout, stderr, commands...)
}
