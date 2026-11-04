// Package migratecmd provides the console commands for SQL migrations:
// db migrate, db migrate:status, db migrate:baseline, db migrate:hash and
// db migrate:diff.
//
// The commands are standalone: they load their
// settings and connect to the database only when they run, so describing
// them and showing help never opens a connection. Only db migrate:diff needs
// the Atlas CLI; db migrate:hash needs neither the CLI nor the settings.
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
)

// Settings are what the migration commands need from the application.
type Settings struct {
	Postgres postgres.Config
	// AppVersion is recorded with every applied migration; optional. The
	// application sets it, for example from its own -ldflags variable.
	AppVersion string
	// Logger receives runner progress; optional.
	Logger logger.Interface
	// AtlasEnv labels the generated Atlas environment; default "local".
	AtlasEnv string
	// AtlasDevURL is optional disposable scratch storage for migrate:diff.
	// Empty uses atlas.DefaultDevURL, never Postgres (the application database).
	AtlasDevURL string
}

// SettingsLoader loads settings when a command runs. On success it returns a
// non-nil cleanup that releases resources after the command finishes.
type SettingsLoader func() (Settings, func(), error)

// Commands are the SQL migration commands of an application.
type Commands struct {
	LoadSettings SettingsLoader
	Source       fs.FS
	// ProjectRoot contains database/migrations/ and tools/schemaloader/.
	ProjectRoot string
	// Options are extra runner options, such as migrate.WithTable.
	Options []migrate.Option
}

// New returns the migration commands for the migrations in source.
func New(load SettingsLoader, source fs.FS) *Commands {
	return &Commands{LoadSettings: load, Source: source, ProjectRoot: "."}
}

// NewDBGroup returns the "db" command group with subcommands.
func NewDBGroup(subcommands ...*cobra.Command) *cobra.Command {
	group := &cobra.Command{Use: "db", Short: "Manage the application database"}
	group.AddCommand(subcommands...)
	return group
}

// Subcommands returns the migration subcommands of the db group.
func (c *Commands) Subcommands() []*cobra.Command {
	return []*cobra.Command{
		c.migrateCommand(),
		c.statusCommand(),
		c.baselineCommand(),
		c.hashCommand(),
		c.diffCommand(),
	}
}

// Command returns a db group that contains only the migration commands.
func (c *Commands) Command() *cobra.Command {
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

func (c *Commands) atlasOptions(env string) (atlas.DiffOptions, error) {
	settings, cleanup, err := c.LoadSettings()
	if err != nil {
		return atlas.DiffOptions{}, err
	}
	defer cleanup()
	if env == "" {
		env = settings.AtlasEnv
	}
	return atlas.DiffOptions{Env: env, DevURL: settings.AtlasDevURL}, nil
}

func (c *Commands) migrateCommand() *cobra.Command {
	command := &cobra.Command{
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
		RunE: console.Standalone(func(cmd *cobra.Command, _ []string) error {
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

	command.Flags().Int("steps", 0, "apply only the next N pending migrations")
	command.Flags().String("to", "", "apply pending migrations up to and including this version (YYYYMMDDHHMMSS)")
	command.Flags().Bool("allow-modified", false,
		"apply pending migrations even though applied migration files were modified (the changes are NOT re-applied)")

	return command
}

func (c *Commands) statusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate:status",
		Short: "Show the state of SQL migrations",
		Args:  cobra.NoArgs,
		RunE: console.Standalone(func(cmd *cobra.Command, _ []string) error {
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

func (c *Commands) baselineCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate:baseline <version>",
		Short: "Mark migrations up to a version as applied without running them",
		Long: `Record every pending migration up to and including <version> as applied
without executing it. Use it once to adopt an existing database whose schema
already matches those migrations.`,
		Args: cobra.ExactArgs(1),
		RunE: console.Standalone(func(cmd *cobra.Command, args []string) error {
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

func (c *Commands) hashCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate:hash",
		Short: "Recompute database/migrations/atlas.sum after editing SQL migrations",
		Long: `Recompute database/migrations/atlas.sum from the SQL files in database/migrations/ as they are
now, for example after writing SQL into a file created by "make migration" or
after editing a migration that was not applied yet. SQL files are not changed.

The checksum is computed by the official Atlas Go library in the format of
"atlas migrate hash"; neither the Atlas CLI nor database settings are needed.
Run it deliberately after reviewing the SQL: generators never rewrite a
missing or mismatching atlas.sum on their own. The checksum protects the
directory, not the database: db migrate keeps its own history of applied
migrations and does not read atlas.sum. Commit the SQL files and atlas.sum
together.`,
		Args: cobra.NoArgs,
		RunE: console.Standalone(func(cmd *cobra.Command, _ []string) error {
			workspace, err := atlas.OpenWorkspace(c.ProjectRoot)
			if err != nil {
				return err
			}
			defer workspace.Close()
			changed, err := workspace.Rehash()
			if err != nil {
				return err
			}
			out, files := cmd.OutOrStdout(), workspace.Files()
			switch {
			case changed:
				fmt.Fprintf(out, "updated %s/%s for %d migration(s); commit it together with the SQL files\n", atlas.MigrationsDir, atlas.SumFile, len(files))
			case len(files) == 0:
				fmt.Fprintf(out, "no SQL migrations in %s; nothing to hash\n", atlas.MigrationsDir)
			default:
				fmt.Fprintf(out, "%s/%s is up to date\n", atlas.MigrationsDir, atlas.SumFile)
			}
			return nil
		}),
	}
}

func (c *Commands) diffCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "migrate:diff <name>",
		Short: "Generate a SQL migration from the GORM models (optional, needs the Atlas CLI)",
		Long: `Generate a SQL migration from tools/schemaloader and the migrations directory.
This is the only migration command that needs the Atlas CLI; hand-written
migrations ("make migration", "db migrate:hash") and db migrate work without it.
Atlas configuration is generated temporarily from application settings; no
atlas.hcl is required. MIGRATIONS_DEV_URL in Template can override the disposable
PostgreSQL development database. The application database is never used as scratch
storage. --env overrides the generated environment label (APP_ENV in Template).

Atlas works on a private copy of database/migrations/. When SQL migrations exist,
database/migrations/atlas.sum must be present and match them, otherwise nothing is
generated and the command asks for an explicit "db migrate:hash". New
migration files and the
updated atlas.sum (created on the first migration) are written to the project
together; when there are no schema changes, or on any error, the project is
left untouched. Commit the SQL files and atlas.sum together.`,
		Args: cobra.ExactArgs(1),
		RunE: console.Standalone(func(cmd *cobra.Command, args []string) error {
			env, _ := cmd.Flags().GetString("env")
			name := args[0]
			if err := migrate.ValidateName(name); err != nil {
				return err
			}
			binary, err := atlas.Lookup()
			if err != nil {
				return fmt.Errorf("db migrate:diff generates migrations with the Atlas CLI; hand-written migrations (make migration, db migrate:hash) and db migrate do not need it: %w", err)
			}
			options, err := c.atlasOptions(env)
			if err != nil {
				return err
			}

			out, errOut := cmd.OutOrStdout(), cmd.ErrOrStderr()
			workspace, err := atlas.OpenWorkspace(c.ProjectRoot)
			if err != nil {
				return err
			}
			defer workspace.Close()
			if err := workspace.Verify(); err != nil {
				return err
			}
			options.Dir = workspace.Dir()
			if err := atlas.Diff(cmd.Context(), binary, c.ProjectRoot, name, options, out, errOut); err != nil {
				return fmt.Errorf("%w; nothing was written", err)
			}
			created, err := workspace.Created()
			if err != nil {
				return err
			}
			if len(created) == 0 {
				fmt.Fprintln(out, "no schema changes")
				return nil
			}
			if err := atlas.ValidateFiles(created); err != nil {
				return fmt.Errorf("atlas created migrations with unsupported names; nothing was written: %w", err)
			}
			sum, err := workspace.Sum()
			if err != nil {
				return fmt.Errorf("update %s/%s; nothing was written: %w", atlas.MigrationsDir, atlas.SumFile, err)
			}
			if err := workspace.Publish(created, sum); err != nil {
				return err
			}
			for _, file := range created {
				fmt.Fprintf(out, "created %s/%s\n", atlas.MigrationsDir, file)
			}
			fmt.Fprintf(out, "updated %s/%s; commit it together with the SQL files\n", atlas.MigrationsDir, atlas.SumFile)
			return nil
		}),
	}

	command.Flags().String("env", "", "Generated Atlas environment label (default: application settings)")

	return command
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
