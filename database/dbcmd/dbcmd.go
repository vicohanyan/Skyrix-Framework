// Package dbcmd provides the framework "db" console command group: the SQL
// migration commands of migratecmd plus the development commands
// "db automigrate" and "db seed".
//
// The application supplies only its data: the GORM models to synchronise
// (Models) and the seeders to run (Seeders).
package dbcmd

import (
	"context"
	"errors"
	"fmt"

	"gitlab.com/skyrix-lib/framework/database/migrate/migratecmd"
	"gitlab.com/skyrix-lib/framework/engine"
	"gitlab.com/skyrix-lib/framework/logger"

	"github.com/spf13/cobra"
)

// Models are the GORM models synchronised by "db automigrate".
type Models []any

// Seeder seeds data, typically dictionaries. Seeders must be idempotent:
// "db automigrate" and "db seed" both run them.
type Seeder interface {
	Seed(ctx context.Context) error
}

// SeederFunc adapts a function to Seeder.
type SeederFunc func(ctx context.Context) error

// Seed calls f.
func (f SeederFunc) Seed(ctx context.Context) error { return f(ctx) }

// Seeders run in order by "db seed" and after "db automigrate".
type Seeders []Seeder

// Commands is the db command group.
type Commands struct {
	DB         *engine.Database
	Logger     logger.Interface
	Migrations *migratecmd.Commands
	Models     Models
	Seeders    Seeders
}

// New returns the db command group.
func New(
	db *engine.Database,
	log logger.Interface,
	migrations *migratecmd.Commands,
	models Models,
	seeders Seeders,
) *Commands {
	return &Commands{DB: db, Logger: log, Migrations: migrations, Models: models, Seeders: seeders}
}

// ToCobraCommand returns "db" with the migration, automigrate and seed
// subcommands.
func (c *Commands) ToCobraCommand() *cobra.Command {
	cmd := migratecmd.NewDBGroup()
	if c.Migrations != nil {
		cmd.AddCommand(c.Migrations.CobraCommands()...)
	}
	cmd.AddCommand(c.autoMigrateCommand(), c.seedCommand())
	return cmd
}

// AutoMigrate synchronises the models with GORM AutoMigrate and runs the
// seeders.
func (c *Commands) AutoMigrate(ctx context.Context) error {
	if c.DB == nil {
		return errors.New("db automigrate: database is not configured")
	}
	if len(c.Models) > 0 {
		if err := c.DB.WithContext(ctx).AutoMigrate(c.Models...); err != nil {
			return fmt.Errorf("auto migrate entities: %w", err)
		}
	}
	c.info("db automigrate completed", "models", len(c.Models))
	if err := c.Seed(ctx); err != nil {
		return fmt.Errorf("seed dictionaries: %w", err)
	}
	return nil
}

// Seed runs the seeders in order.
func (c *Commands) Seed(ctx context.Context) error {
	for i, seeder := range c.Seeders {
		if seeder == nil {
			continue
		}
		if err := seeder.Seed(ctx); err != nil {
			return fmt.Errorf("seeder %d: %w", i, err)
		}
	}
	c.info("db dictionary seed completed", "seeders", len(c.Seeders))
	return nil
}

func (c *Commands) info(msg string, fields ...any) {
	if c.Logger != nil {
		c.Logger.Info(msg, fields...)
	}
}

func (c *Commands) autoMigrateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "automigrate",
		Short: "DEV ONLY: sync GORM entities with AutoMigrate (use db migrate in production)",
		Long: `Synchronise the application GORM entities with gorm AutoMigrate and run the seeders.

WARNING: development only. AutoMigrate never drops or rewrites columns, is not
recorded in skyrix_migrations and can diverge from the SQL migrations. Use
"db migrate" for every shared, staging or production database.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := c.AutoMigrate(cmd.Context()); err != nil {
				return fmt.Errorf("migrate database: %w", err)
			}
			return nil
		},
	}
}

func (c *Commands) seedCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "seed",
		Short: "Seed required dictionary data",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := c.Seed(cmd.Context()); err != nil {
				return fmt.Errorf("seed database: %w", err)
			}
			return nil
		},
	}
}
