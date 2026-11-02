package db

import (
	"fmt"

	"gitlab.com/skyrix-lib/template/internal/database"

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
		Use:   "automigrate",
		Short: "DEV ONLY: sync GORM entities with AutoMigrate (use db migrate in production)",
		Long: `Synchronise the selected GORM entities with gorm AutoMigrate and seed dictionaries.

WARNING: development only. AutoMigrate never drops or rewrites columns, is not
recorded in skyrix_migrations and can diverge from the SQL migrations. Use
"db migrate" for every shared, staging or production database.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := c.Migrator.Migrate(cmd.Context()); err != nil {
				return fmt.Errorf("migrate database: %w", err)
			}

			return nil
		},
	}
}
