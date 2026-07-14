package job

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	engineJobs "skyrix/internal/engine/jobs"

	"github.com/spf13/cobra"
)

type JobsRunCommand struct {
	Registry engineJobs.Registry
}

func NewJobsRunCommand(registry engineJobs.Registry) *JobsRunCommand {
	return &JobsRunCommand{Registry: registry}
}

func (c *JobsRunCommand) ToCobraCommand() *cobra.Command {
	var name string
	var argsJSON string
	var limit int
	var maxAttempts int

	cmd := &cobra.Command{
		Use:   "jobs:run",
		Short: "Run a registered job synchronously",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}

			args, err := decodeJobArgs(argsJSON)
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("limit") {
				args["limit"] = limit
			}
			if cmd.Flags().Changed("max-attempts") {
				args["max_attempts"] = maxAttempts
			}

			if err := c.Registry.Run(cmd.Context(), name, args); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "job failed: %s: %v\n", name, err)
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "job succeeded: %s\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Registered job name")
	cmd.Flags().StringVar(&argsJSON, "args-json", "", "JSON object with job arguments")
	cmd.Flags().IntVar(&limit, "limit", 0, "Job limit argument")
	cmd.Flags().IntVar(&maxAttempts, "max-attempts", 0, "Job max_attempts argument")

	return cmd
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
