package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// HelloCommand is a simple "hello" command.
type HelloCommand struct{}

// NewHelloCommand constructs a new HelloCommand.
func NewHelloCommand() *HelloCommand {
	return &HelloCommand{}
}

// ToCobraCommand converts HelloCommand into a *cobra.Command.
// It defines flags, help text, and the execution handler.
func (c *HelloCommand) ToCobraCommand() *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "hello",
		Short: "Print a greeting message",
		Long:  "This command prints a greeting message to stdout.",
		Run: func(cmd *cobra.Command, args []string) {
			if name == "" {
				fmt.Println("Hello, World!")
				return
			}
			fmt.Printf("Hello, %s!\n", name)
		},
	}

	// Optional flag to customize the greeting target.
	cmd.Flags().StringVarP(&name, "name", "n", "", "Name to greet")

	return cmd
}
