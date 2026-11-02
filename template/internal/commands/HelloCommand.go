package commands

import (
	"fmt"

	"gitlab.com/skyrix-lib/framework/cobra/console"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Hello describes the example "hello" command: it prints a greeting signed
// with the service name from the configuration.
func Hello(deps *console.Graph[*Deps]) *console.Command {
	return &console.Command{
		Use:   "hello",
		Short: "Print a greeting message",
		Long:  "This command prints a greeting message to stdout.",
		Args:  cobra.NoArgs,
		Flags: func(flags *pflag.FlagSet) {
			// Optional flag to customize the greeting target.
			flags.StringP("name", "n", "", "Name to greet")
		},
		Run: console.With(deps, func(cmd *cobra.Command, _ []string, deps *Deps) error {
			name, _ := cmd.Flags().GetString("name")
			if name == "" {
				name = "World"
			}
			greeting := fmt.Sprintf("Hello, %s!", name)
			if service := deps.Config.ServiceName; service != "" {
				greeting += " from " + service
			}
			deps.Logger.Debug("hello command", "name", name)
			fmt.Fprintln(cmd.OutOrStdout(), greeting)
			return nil
		}),
	}
}
