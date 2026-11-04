// Package jobscmd provides the "jobs:run" console command.
package jobscmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"gitlab.com/skyrix-lib/framework/cobra/console"
	engineJobs "gitlab.com/skyrix-lib/framework/engine/jobs"

	"github.com/spf13/cobra"
)

// Command describes "jobs:run". The job registry comes from registry, built
// only when the command runs; it usually derives from the application
// graph, since jobs need the application's services.
func Command(registry *console.Graph[engineJobs.Registry]) *cobra.Command {
	command := &cobra.Command{
		Use:   "jobs:run",
		Short: "Run a registered job synchronously",
		RunE:  console.With(registry, run),
	}

	command.Flags().String("name", "", "Registered job name")
	command.Flags().String("args-json", "", "JSON object with job arguments")
	command.Flags().Int("limit", 0, "Job limit argument")
	command.Flags().Int("max-attempts", 0, "Job max_attempts argument")

	return command
}

func run(cmd *cobra.Command, _ []string, registry engineJobs.Registry) error {
	flags := cmd.Flags()
	name, _ := flags.GetString("name")
	argsJSON, _ := flags.GetString("args-json")
	if name == "" {
		return fmt.Errorf("--name is required")
	}

	args, err := decodeJobArgs(argsJSON)
	if err != nil {
		return err
	}
	if flags.Changed("limit") {
		args["limit"], _ = flags.GetInt("limit")
	}
	if flags.Changed("max-attempts") {
		args["max_attempts"], _ = flags.GetInt("max-attempts")
	}

	if err := registry.Run(cmd.Context(), name, args); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "job failed: %s: %v\n", name, err)
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "job succeeded: %s\n", name)
	return nil
}

func decodeJobArgs(raw string) (map[string]any, error) {
	if raw == "" {
		return map[string]any{}, nil
	}

	dec := json.NewDecoder(bytes.NewBufferString(raw))
	dec.UseNumber()

	var args map[string]any
	if err := dec.Decode(&args); err != nil {
		return nil, fmt.Errorf("decode --args-json: %w", err)
	}
	if args == nil {
		args = map[string]any{}
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("decode --args-json: unexpected trailing data")
	}
	return args, nil
}
