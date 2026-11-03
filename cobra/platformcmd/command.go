// Package platformcmd supplies standalone driver declaration commands.
package platformcmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gitlab.com/skyrix-lib/framework/cobra/console"
	"gitlab.com/skyrix-lib/framework/scaffold/platform"
)

// Command reads files and resolves declared packages only when run. --dir points
// to the application's providers directory; graph and command choices are manual.
func Command() *console.Command {
	flags := func(f *pflag.FlagSet) { f.String("dir", "providers", "application providers directory") }
	group := &console.Command{Use: "platform", Short: "Manage explicit driver declarations"}
	for _, action := range []string{"add", "remove"} {
		add := action == "add"
		group.Commands = append(group.Commands, &console.Command{Use: action + " <import-path>", Short: action + " a driver declaration", Args: cobra.ExactArgs(1), Flags: flags,
			Run: console.Standalone(func(cmd *cobra.Command, args []string) error {
				dir, _ := cmd.Flags().GetString("dir")
				changed, err := platform.Change(dir, args[0], add)
				if err != nil {
					return err
				}
				if !changed {
					cmd.Println("No changes.")
					return nil
				}
				cmd.Println("Declarations and independent sets updated. Edit Wire graphs, platform providers and command selection explicitly; regenerate Wire with your installed tool.")
				if add {
					cmd.Println("If required, run go get <module>@<version> and go mod tidy explicitly; no dependencies or tools were installed.")
				}
				return nil
			}),
		})
	}
	group.Commands = append(group.Commands, &console.Command{Use: "list", Short: "List declared drivers and config keys", Args: cobra.NoArgs, Flags: flags, Run: console.Standalone(func(cmd *cobra.Command, _ []string) error {
		dir, _ := cmd.Flags().GetString("dir")
		drivers, err := platform.List(dir)
		if err != nil {
			return err
		}
		for _, d := range drivers {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", d.Manifest.Name, d.Manifest.ConfigKey, d.ImportPath); err != nil {
				return err
			}
		}
		return nil
	})})
	return group
}
