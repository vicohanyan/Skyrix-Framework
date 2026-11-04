// Package configcmd describes configuration without loading application settings.
package configcmd

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gitlab.com/skyrix-lib/framework/cobra/console"
	"gitlab.com/skyrix-lib/framework/config"
)

// Env lists defaults from fresh, explicitly supplied section descriptions.
// Pass default targets, never configuration populated from YAML or environment.
// It calls neither Load nor Validate and needs no application dependency graph.
func Env(sections ...config.Section) *cobra.Command {
	loader, metadataErr := config.New(sections...)
	var metadata []config.Metadata
	if metadataErr == nil {
		metadata = loader.Sections()
	}
	command := &cobra.Command{
		Use:   "config:env",
		Short: "List environment variables, types and safe defaults",
		Args:  cobra.NoArgs,
		RunE: console.Standalone(func(cmd *cobra.Command, _ []string) error {
			if metadataErr != nil {
				return metadataErr
			}
			section, err := cmd.Flags().GetString("section")
			if err != nil {
				return err
			}
			selected := make([]config.Metadata, 0, len(metadata))
			for _, m := range metadata {
				if !cmd.Flags().Changed("section") || m.Key == section {
					selected = append(selected, m)
				}
			}
			if cmd.Flags().Changed("section") && len(selected) == 0 {
				return fmt.Errorf("config: unknown section %q", section)
			}
			sort.Slice(selected, func(i, j int) bool { return selected[i].Key < selected[j].Key })
			out := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			if _, err := fmt.Fprintln(out, "SECTION\tENV\tTYPE\tDEFAULT"); err != nil {
				return err
			}
			for _, m := range selected {
				fields := append([]config.Field(nil), m.Fields...)
				sort.Slice(fields, func(i, j int) bool { return fields[i].Env < fields[j].Env })
				for _, f := range fields {
					if _, err := fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", m.Key, f.Env, f.Type, f.Default); err != nil {
						return err
					}
				}
			}
			return out.Flush()
		}),
	}

	command.Flags().String("section", "", "Show only the section with this key")

	return command
}
