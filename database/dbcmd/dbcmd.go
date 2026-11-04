// Package dbcmd provides the framework "db" console command group: the SQL
// migration commands of migratecmd plus "db seed".
//
// The application supplies the seeders to run (Seeders), built with the database in its
// database graph (Deps).
package dbcmd

import (
	"gitlab.com/skyrix-lib/framework/logger"

	"context"
	"fmt"

	"gitlab.com/skyrix-lib/framework/cobra/console"
	"gitlab.com/skyrix-lib/framework/database/migrate/migratecmd"
	"gitlab.com/skyrix-lib/framework/engine/postgres/gormdb"

	"github.com/spf13/cobra"
)

// Seeder seeds data, typically dictionaries. Seeders must be idempotent.
type Seeder interface {
	Seed(ctx context.Context) error
}

// SeederFunc adapts a function to Seeder.
type SeederFunc func(ctx context.Context) error

// Seed calls f.
func (f SeederFunc) Seed(ctx context.Context) error { return f(ctx) }

// Seeders run in order by "db seed".
type Seeders []Seeder

// Deps are what "db seed" runs with. The application builds them in its
// database graph, only when the command runs.
type Deps struct {
	DB      *gormdb.Database
	Logger  logger.Interface
	Seeders Seeders
}

// NewDeps returns the dependencies of db seed.
func NewDeps(db *gormdb.Database, log logger.Interface, seeders Seeders) *Deps {
	return &Deps{DB: db, Logger: log, Seeders: seeders}
}

// Command returns the "db" group: the standalone migration commands of
// migrations (when not nil) plus "db seed", which gets deps when it runs.
func Command(migrations *migratecmd.Commands, deps *console.Graph[*Deps]) *cobra.Command {
	var subcommands []*cobra.Command
	if migrations != nil {
		subcommands = migrations.Subcommands()
	}
	return migratecmd.NewDBGroup(append(subcommands, seedCommand(deps))...)
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

func seedCommand(deps *console.Graph[*Deps]) *cobra.Command {
	return &cobra.Command{
		Use:   "seed",
		Short: "Seed required dictionary data",
		Args:  cobra.NoArgs,
		RunE: console.With(deps, func(cmd *cobra.Command, _ []string, deps *Deps) error {
			if err := deps.Seed(cmd.Context()); err != nil {
				return fmt.Errorf("seed database: %w", err)
			}
			return nil
		}),
	}
}
