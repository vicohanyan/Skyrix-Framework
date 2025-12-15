package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// CreateUserCommand represents a CLI command that creates a new user.
// For now, it has no dependencies, but it is designed to accept them later
// (e.g., UserService, Jobs dispatcher, logger).
type CreateUserCommand struct {
	// Dependencies can be added here in the future, e.g.:
	// UserService *users.Service
	// Jobs        *jobs.Registry
}

// NewCreateUserCommand is the constructor for CreateUserCommand.
// At the moment it does not accept any dependencies.
func NewCreateUserCommand() *CreateUserCommand {
	return &CreateUserCommand{}
}

// ToCobraCommand converts CreateUserCommand into a *cobra.Command.
// This is where the command metadata (Use/Short/Long) and execution logic (RunE) are defined.
func (c *CreateUserCommand) ToCobraCommand() *cobra.Command {
	var runAsJob bool // If true, simulate dispatching the work as a background job
	var name string
	var email string

	cmd := &cobra.Command{
		Use:   "create:user",
		Short: "Create a new user in the system",
		Long:  "Creates a new user. The command can either run synchronously or simulate dispatching a background job.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context() // Use Cobra's context (cancellable via ExecuteContext)

			if runAsJob {
				fmt.Println("Dispatching user creation as a job...")

				// NOTE: This is only a simulation. In a real app you would enqueue the job
				// (e.g., NATS/Redis/DB queue) instead of starting a goroutine in the CLI process.
				go func(ctx context.Context, name, email string) {
					fmt.Printf("Job: user creation started for %s (%s)...\n", name, email)
					time.Sleep(2 * time.Second) // Simulate work
					fmt.Printf("Job: user creation finished for %s.\n", name)
				}(ctx, name, email)

				fmt.Println("Job dispatched successfully.")
				return nil
			}

			fmt.Println("Executing user creation directly...")
			fmt.Printf("User %s (%s) created.\n", name, email)
			return nil
		},
	}

	// Flags
	cmd.Flags().BoolVar(&runAsJob, "job", false, "Run user creation as a background job (simulated)")
	cmd.Flags().StringVarP(&name, "name", "n", "", "User name")
	cmd.Flags().StringVarP(&email, "email", "e", "", "User email")

	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("email")

	return cmd
}
