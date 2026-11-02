package cobra

import (
	"context"
	"io"

	spfcobra "github.com/spf13/cobra"
)

// ExecuteStandalone runs args against commands that do not need the
// application container, such as platform commands, scaffolding and SQL
// migrations; such commands open their own connections only when executed, so
// --help never connects anywhere. It reports false, without running anything,
// when args name a command group or a command that is not among commands; the
// caller then builds the full console application.
func ExecuteStandalone(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	commands ...*spfcobra.Command,
) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}

	root := &spfcobra.Command{Use: "cobra", SilenceUsage: true, SilenceErrors: true}
	for _, command := range commands {
		if command != nil {
			root.AddCommand(command)
		}
	}

	target, _, err := root.Find(args)
	if err != nil || target == root || !target.Runnable() {
		return false, nil
	}

	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	return true, root.ExecuteContext(ctx)
}
