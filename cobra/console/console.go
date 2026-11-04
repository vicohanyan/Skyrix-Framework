package console

import (
	"context"
	"io"

	"github.com/spf13/cobra"
)

// Console hosts native Cobra commands. Construct a fresh console and fresh
// commands for each execution; Cobra retains parsed flag and execution state.
type Console struct{ root *cobra.Command }

func New(commands ...*cobra.Command) *Console {
	c := &Console{root: &cobra.Command{Use: "cobra", Short: "Skyrix CLI console", SilenceErrors: true}}
	return c.Add(commands...)
}

// Add registers commands on the root, skipping nil entries.
func (c *Console) Add(commands ...*cobra.Command) *Console {
	for _, command := range commands {
		if command != nil {
			c.root.AddCommand(command)
		}
	}
	return c
}

// Root exposes Cobra's command tree and root settings.
func (c *Console) Root() *cobra.Command { return c.root }

// Execute runs args against the root. nil args means no arguments.
func (c *Console) Execute(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if args == nil {
		args = []string{}
	}
	c.root.SetArgs(args)
	if stdout != nil {
		c.root.SetOut(stdout)
	}
	if stderr != nil {
		c.root.SetErr(stderr)
	}
	return c.root.ExecuteContext(ctx)
}
