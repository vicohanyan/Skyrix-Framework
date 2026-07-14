package db

import (
	"fmt"

	"skyrix/internal/database"

	"github.com/spf13/cobra"
)

type DBAutoMigrateCommand struct {
	Migrator *database.DBMigrator
}

func NewDBAutoMigrateCommand(
	migrator *database.DBMigrator,
) *DBAutoMigrateCommand {
	return &DBAutoMigrateCommand{
		Migrator: migrator,
	}
}

func (c *DBAutoMigrateCommand) ToCobraCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "automigrate",
		Aliases: []string{"migrate"},
		Short:   "Auto migrate selected GORM entities",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := c.Migrator.Migrate(cmd.Context()); err != nil {
				return fmt.Errorf("migrate database: %w", err)
			}

			return nil
		},
	}
}
