// Package dbcmd provides the framework "db" console command group: the SQL
// migration commands of migratecmd plus the development commands
// "db automigrate" and "db seed".
//
// The application supplies only its data: the GORM models to synchronise
// (Models) and the seeders to run (Seeders), built with the database in its
// database graph (Deps).
package dbcmd

import (
	"gitlab.com/skyrix-lib/framework/logger"

	"context"
	"errors"
	"fmt"

	"gitlab.com/skyrix-lib/framework/cobra/console"
	"gitlab.com/skyrix-lib/framework/database/migrate/migratecmd"
	"gitlab.com/skyrix-lib/framework/engine/postgres/gormdb"

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

// Deps are what "db automigrate" and "db seed" run with. The application
// builds them in its database graph (console.NeedsDatabase), only when one of
// these commands runs.
type Deps struct {
	DB      *gormdb.Database
	Logger  logger.Interface
	Models  Models
	Seeders Seeders
}

// NewDeps returns the dependencies of db automigrate and db seed.
func NewDeps(db *gormdb.Database, log logger.Interface, models Models, seeders Seeders) *Deps {
	return &Deps{DB: db, Logger: log, Models: models, Seeders: seeders}
}

// Command returns the "db" group: the standalone migration commands of
// migrations (when not nil) plus "db automigrate" and "db seed", which get
// deps when they run.
func Command(migrations *migratecmd.Commands, deps *console.Graph[*Deps]) *console.Command {
	var subcommands []*console.Command
	if migrations != nil {
		subcommands = migrations.Subcommands()
	}
	return migratecmd.NewDBGroup(append(subcommands, autoMigrateCommand(deps), seedCommand(deps))...)
}

// AutoMigrate synchronises the models with GORM AutoMigrate and runs the
// seeders.
func (d *Deps) AutoMigrate(ctx context.Context) error {
	if d.DB == nil {
		return errors.New("db automigrate: database is not configured")
	}
	if len(d.Models) > 0 {
		if err := d.DB.WithContext(ctx).AutoMigrate(d.Models...); err != nil {
			return fmt.Errorf("auto migrate entities: %w", err)
		}
	}
	d.info("db automigrate completed", "models", len(d.Models))
	if err := d.Seed(ctx); err != nil {
		return fmt.Errorf("seed dictionaries: %w", err)
	}
	return nil
}

// Seed runs the seeders in order.
func (d *Deps) Seed(ctx context.Context) error {
	for i, seeder := range d.Seeders {
		if seeder == nil {
			continue
		}
		if err := seeder.Seed(ctx); err != nil {
			return fmt.Errorf("seeder %d: %w", i, err)
		}
	}
	d.info("db dictionary seed completed", "seeders", len(d.Seeders))
	return nil
}

func (d *Deps) info(msg string, fields ...any) {
	if d.Logger != nil {
		d.Logger.Info(msg, fields...)
	}
}

func autoMigrateCommand(deps *console.Graph[*Deps]) *console.Command {
	return &console.Command{
		Use:   "automigrate",
		Short: "DEV ONLY: sync GORM entities with AutoMigrate (use db migrate in production)",
		Long: `Synchronise the application GORM entities with gorm AutoMigrate and run the seeders.

WARNING: development only. AutoMigrate never drops or rewrites columns, is not
recorded in skyrix_migrations and can diverge from the SQL migrations. Use
"db migrate" for every shared, staging or production database.`,
		Args: cobra.NoArgs,
		Run: console.With(deps, func(cmd *cobra.Command, _ []string, deps *Deps) error {
			if err := deps.AutoMigrate(cmd.Context()); err != nil {
				return fmt.Errorf("migrate database: %w", err)
			}
			return nil
		}),
	}
}

func seedCommand(deps *console.Graph[*Deps]) *console.Command {
	return &console.Command{
		Use:   "seed",
		Short: "Seed required dictionary data",
		Args:  cobra.NoArgs,
		Run: console.With(deps, func(cmd *cobra.Command, _ []string, deps *Deps) error {
			if err := deps.Seed(cmd.Context()); err != nil {
				return fmt.Errorf("seed database: %w", err)
			}
			return nil
		}),
	}
}
