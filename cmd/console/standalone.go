package main

import (
	"context"
	"io"

	"skyrix/internal/engine/scaffold/eventbus"
)

// executeStandaloneCommand runs filesystem-only commands without bootstrapping
// PostgreSQL, Redis, jobs, or the rest of the application container.
func executeStandaloneCommand(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) (bool, error) {
	if len(args) == 0 || (args[0] != "eventbus" && args[0] != "bus") {
		return false, nil
	}

	command := eventbus.NewEventBusCommand().ToCobraCommand()
	command.SetArgs(args[1:])
	command.SetContext(ctx)
	command.SetOut(stdout)
	command.SetErr(stderr)
	return true, command.Execute()
}
