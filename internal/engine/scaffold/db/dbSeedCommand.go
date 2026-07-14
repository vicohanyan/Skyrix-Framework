package db

import (
	"fmt"

	"skyrix/internal/database"

	"github.com/spf13/cobra"
)

type DBSeedCommand struct {
	Seeder *database.DBSeeder
}

func NewDBSeedCommand(seeder *database.DBSeeder) *DBSeedCommand {
	return &DBSeedCommand{
		Seeder: seeder,
	}
}

func (c *DBSeedCommand) ToCobraCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "seed",
		Short: "Seed required dictionary data",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := c.Seeder.Seed(cmd.Context()); err != nil {
				return fmt.Errorf("seed database: %w", err)
			}

			return nil
		},
	}
}
