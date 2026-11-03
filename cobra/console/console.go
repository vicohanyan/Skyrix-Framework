package console

import (
	"context"
	"io"

	"github.com/spf13/cobra"
)

// Console is the console application: the root command and the command
// descriptions. Building and executing it builds no dependency graph by
// itself: help and completion are answered from the descriptions, and a
// command's graph is built only when that command runs.
type Console struct {
	// Use, Short and Long describe the root command.
	Use   string
	Short string
	Long  string

	commands []*Command
	cobra    []*cobra.Command
}

// New returns a console with the default root ("cobra") and commands.
func New(commands ...*Command) *Console {
	console := &Console{
		Use:   "cobra",
		Short: "Skyrix CLI console built with github.com/spf13/cobra",
		Long:  `Skyrix CLI console. This console is powered by the Cobra library (github.com/spf13/cobra).`,
	}
	return console.Add(commands...)
}

// Add registers command descriptions; nil commands are skipped.
func (c *Console) Add(commands ...*Command) *Console {
	for _, command := range commands {
		if command != nil {
			c.commands = append(c.commands, command)
		}
	}
	return c
}

// AddCobra registers application commands that are already Cobra commands.
// They are standalone:
// the console builds no graph for them. A Cobra command can belong to one
// root only, so build them for each console. nil commands are skipped.
func (c *Console) AddCobra(commands ...*cobra.Command) *Console {
	for _, command := range commands {
		if command != nil {
			c.cobra = append(c.cobra, command)
		}
	}
	return c
}

// Commands returns the registered descriptions in registration order.
func (c *Console) Commands() []*Command {
	return append([]*Command(nil), c.commands...)
}

// Find returns the description at path (command names), or nil. Commands
// added with AddCobra are not descriptions and are not found.
func (c *Console) Find(path ...string) *Command {
	if len(path) == 0 {
		return nil
	}
	return (&Command{Commands: c.commands}).Find(path...)
}

// Root builds the Cobra root command with every registered command. Errors
// are returned by Execute, not printed by Cobra.
func (c *Console) Root() *cobra.Command {
	root := &cobra.Command{
		Use:           c.Use,
		Short:         c.Short,
		Long:          c.Long,
		SilenceErrors: true,
	}
	for _, command := range c.commands {
		root.AddCommand(command.Cobra())
	}
	root.AddCommand(c.cobra...)
	return root
}

// Execute runs args (typically os.Args[1:]) against Root. nil writers keep
// the process stdout and stderr.
func (c *Console) Execute(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if args == nil {
		args = []string{} // nil would make Cobra read os.Args
	}
	root := c.Root()
	root.SetArgs(args)
	if stdout != nil {
		root.SetOut(stdout)
	}
	if stderr != nil {
		root.SetErr(stderr)
	}
	return root.ExecuteContext(ctx)
}
