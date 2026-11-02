// Package migratecmd provides the console commands for SQL migrations:
// db migrate, db migrate:status, db migrate:baseline and db migrate:diff.
//
// The commands are standalone (console.NeedsNothing): they load their
// settings and connect to the database only when they run, so describing
// them and showing help never opens a connection.
package migratecmd

import (
	"gitlab.com/skyrix-lib/framework/logger"

	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"text/tabwriter"
	"time"

	"gitlab.com/skyrix-lib/framework/cobra/console"
	"gitlab.com/skyrix-lib/framework/database/migrate"
	"gitlab.com/skyrix-lib/framework/database/migrate/atlas"
	"gitlab.com/skyrix-lib/framework/engine/postgres"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Settings are what the migration commands need from the application.
type Settings struct {
	Postgres postgres.Config
	// AppVersion is recorded with every applied migration; optional. The
	// application sets it, for example from its own -ldflags variable.
	AppVersion string
	// Logger receives runner progress; optional.
	Logger logger.Interface
	// AtlasEnv names the atlas.hcl env used by migrate:diff; default "local".
	AtlasEnv string
}

// SettingsLoader loads settings when a command runs. On success it returns a
// non-nil cleanup that releases resources after the command finishes.
type SettingsLoader func() (Settings, func(), error)

// Commands are the SQL migration commands of an application.
type Commands struct {
	LoadSettings SettingsLoader
	Source       fs.FS
	// ProjectRoot is where atlas.hcl and the migrations directory live.
	ProjectRoot string
	// Options are extra runner options, such as migrate.WithTable.
	Options []migrate.Option
}

// New returns the migration commands for the migrations in source.
func New(load SettingsLoader, source fs.FS) *Commands {
	return &Commands{LoadSettings: load, Source: source, ProjectRoot: "."}
}

// NewDBGroup returns the "db" command group with subcommands.
func NewDBGroup(subcommands ...*console.Command) *console.Command {
	return &console.Command{
		Use:      "db",
		Short:    "Manage the application database",
		Commands: subcommands,
	}
}

// Subcommands returns the migration subcommands of the db group.
func (c *Commands) Subcommands() []*console.Command {
	return []*console.Command{
		c.migrateCommand(),
		c.statusCommand(),
		c.baselineCommand(),
		c.diffCommand(),
	}
}

// Command returns a db group that contains only the migration commands.
func (c *Commands) Command() *console.Command {
	return NewDBGroup(c.Subcommands()...)
}

// Open loads the settings, connects to PostgreSQL and returns a runner for the
// migrations together with a function that closes the connection and settings resources.
func (c *Commands) Open(ctx context.Context, options ...migrate.Option) (*migrate.Runner, func(), error) {
	settings, cleanup, err := c.LoadSettings()
	if err != nil {
		return nil, nil, err
	}
	db, err := postgres.OpenSQL(ctx, settings.Postgres)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("connect to postgres: %w", err)
	}

	all := []migrate.Option{migrate.WithAppVersion(settings.AppVersion)}
	if settings.Logger != nil {
		all = append(all, migrate.WithLogger(settings.Logger))
	}
	all = append(all, c.Options...)
	all = append(all, options...)
	runner, err := migrate.NewRunner(db, c.Source, all...)
	if err != nil {
		_ = db.Close()
		cleanup()
		return nil, nil, err
	}
	return runner, func() {
		_ = db.Close()
		cleanup()
	}, nil
}

func (c *Commands) atlasEnv() (string, error) {
	settings, cleanup, err := c.LoadSettings()
	if err != nil {
		return "", err
	}
	defer cleanup()
	if settings.AtlasEnv == "" {
		return "local", nil
	}
	return settings.AtlasEnv, nil
}

func (c *Commands) migrateCommand() *console.Command {
	return &console.Command{
		Use:   "migrate",
		Short: "Apply pending SQL migrations",
		Long: `Apply pending SQL migrations from the migrations directory in version order.

Without flags all pending migrations are applied. --steps N applies only the
next N pending migrations, --to <version> applies the pending migrations up to
and including that version. The migrations applied by one call form a batch
that shares a run number.

Migrations are forward-only. Each migration runs in its own transaction, except
*.notx.sql files.`,
		Example: `  cobra db migrate
  cobra db migrate --steps 1
  cobra db migrate --to 20261102135250`,
		Args: cobra.NoArgs,
		Flags: func(flags *pflag.FlagSet) {
			flags.Int("steps", 0, "apply only the next N pending migrations")
			flags.String("to", "", "apply pending migrations up to and including this version (YYYYMMDDHHMMSS)")
			flags.Bool("allow-modified", false,
				"apply pending migrations even though applied migration files were modified (the changes are NOT re-applied)")
		},
		Run: console.Standalone(func(cmd *cobra.Command, _ []string) error {
			steps, _ := cmd.Flags().GetInt("steps")
			to, _ := cmd.Flags().GetString("to")
			allowModified, _ := cmd.Flags().GetBool("allow-modified")
			if cmd.Flags().Changed("steps") && to != "" {
				return fmt.Errorf("--steps and --to cannot be used together")
			}
			if cmd.Flags().Changed("steps") && steps < 1 {
				return fmt.Errorf("--steps must be at least 1")
			}
			if to != "" {
				if err := migrate.ValidateVersion(to); err != nil {
					return err
				}
			}

			runner, closeDB, err := c.Open(cmd.Context(), migrate.WithAllowModified(allowModified))
			if err != nil {
				return err
			}
			defer closeDB()

			var result migrate.UpResult
			switch {
			case to != "":
				result, err = runner.UpTo(cmd.Context(), to)
			case cmd.Flags().Changed("steps"):
				result, err = runner.UpSteps(cmd.Context(), steps)
			default:
				result, err = runner.Up(cmd.Context())
			}
			for _, migration := range result.Applied {
				fmt.Fprintf(cmd.OutOrStdout(), "applied %s\n", migration.FileName)
			}
			if err != nil {
				return fmt.Errorf("migrate database: %w", err)
			}
			if len(result.Applied) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no migrations to apply")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "applied %d migration(s) in run %d\n", len(result.Applied), result.Run)
			return nil
		}),
	}
}

func (c *Commands) statusCommand() *console.Command {
	return &console.Command{
		Use:   "migrate:status",
		Short: "Show the state of SQL migrations",
		Args:  cobra.NoArgs,
		Run: console.Standalone(func(cmd *cobra.Command, _ []string) error {
			runner, closeDB, err := c.Open(cmd.Context())
			if err != nil {
				return err
			}
			defer closeDB()

			status, err := runner.Status(cmd.Context())
			if err != nil {
				return fmt.Errorf("read migration status: %w", err)
			}
			return writeStatus(cmd.OutOrStdout(), status)
		}),
	}
}

func (c *Commands) baselineCommand() *console.Command {
	return &console.Command{
		Use:   "migrate:baseline <version>",
		Short: "Mark migrations up to a version as applied without running them",
		Long: `Record every pending migration up to and including <version> as applied
without executing it. Use it once to adopt an existing database whose schema
already matches those migrations.`,
		Args: cobra.ExactArgs(1),
		Run: console.Standalone(func(cmd *cobra.Command, args []string) error {
			runner, closeDB, err := c.Open(cmd.Context())
			if err != nil {
				return err
			}
			defer closeDB()

			marked, err := runner.Baseline(cmd.Context(), args[0])
			if err != nil {
				return fmt.Errorf("baseline migrations: %w", err)
			}
			for _, migration := range marked {
				fmt.Fprintf(cmd.OutOrStdout(), "baselined %s\n", migration.FileName)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "baselined %d migration(s) up to %s\n", len(marked), args[0])
			return nil
		}),
	}
}

func (c *Commands) diffCommand() *console.Command {
	return &console.Command{
		Use:   "migrate:diff <name>",
		Short: "Generate a SQL migration from the GORM models with Atlas",
		Long: `Run "atlas migrate diff <name> --env <env>" in the project root. Atlas compares
the schema loaded from the application models with the migrations directory
and writes the difference as a new migration. The env defaults to the
application's Atlas env (APP_ENV in the template).`,
		Args: cobra.ExactArgs(1),
		Flags: func(flags *pflag.FlagSet) {
			flags.String("env", "", "Atlas env from atlas.hcl (default: the application's Atlas env)")
		},
		Run: console.Standalone(func(cmd *cobra.Command, args []string) error {
			env, _ := cmd.Flags().GetString("env")
			name := args[0]
			if err := migrate.ValidateName(name); err != nil {
				return err
			}
			binary, err := atlas.Lookup()
			if err != nil {
				return err
			}
			if env == "" {
				if env, err = c.atlasEnv(); err != nil {
					return err
				}
			}

			before, err := atlas.MigrationFiles(c.ProjectRoot)
			if err != nil {
				return err
			}
			if err := atlas.Run(cmd.Context(), binary, c.ProjectRoot, cmd.OutOrStdout(), cmd.ErrOrStderr(),
				"migrate", "diff", name, "--env", env); err != nil {
				return err
			}
			after, err := atlas.MigrationFiles(c.ProjectRoot)
			if err != nil {
				return err
			}

			created := atlas.NewFiles(before, after)
			if err := atlas.ValidateFiles(created); err != nil {
				return fmt.Errorf("atlas created migrations with unsupported names; rename them and run atlas migrate hash: %w", err)
			}
			if len(created) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no schema changes")
			}
			for _, file := range created {
				fmt.Fprintf(cmd.OutOrStdout(), "created %s/%s\n", atlas.MigrationsDir, file)
			}
			return nil
		}),
	}
}

func writeStatus(out io.Writer, status migrate.Status) error {
	table := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "VERSION\tNAME\tSTATE\tRUN\tAPP VERSION\tAPPLIED AT")
	pending := 0
	for _, item := range status.Migrations {
		state := "pending"
		run, appVersion, appliedAt := "-", "-", "-"
		switch {
		case item.Modified:
			state = "MODIFIED"
		case item.Applied:
			state = "applied"
		default:
			pending++
		}
		if item.Applied {
			run = fmt.Sprint(item.Record.Run)
			appliedAt = item.Record.AppliedAt.Local().Format(time.DateTime)
			if item.Record.AppVersion != "" {
				appVersion = item.Record.AppVersion
			}
		}
		name := item.Name
		if item.NoTx {
			name += " (notx)"
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\n", item.Version, name, state, run, appVersion, appliedAt)
	}
	if err := table.Flush(); err != nil {
		return err
	}

	fmt.Fprintf(out, "\n%d migration(s), %d pending\n", len(status.Migrations), pending)
	if len(status.Modified) > 0 {
		fmt.Fprintf(out, "\nModified applied migrations (file checksum differs from the database):\n")
		for _, item := range status.Modified {
			fmt.Fprintf(out, "  %s\n", item.FileName)
		}
	}
	if len(status.Orphans) > 0 {
		fmt.Fprintf(out, "\nApplied migrations without a file:\n")
		for _, record := range status.Orphans {
			fmt.Fprintf(out, "  %s_%s (run %d)\n", record.Version, record.Name, record.Run)
		}
	}
	if len(status.Modified) > 0 || len(status.Orphans) > 0 {
		return errors.New("migration files and the database disagree")
	}
	return nil
}
