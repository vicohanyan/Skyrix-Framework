// Package makecmd provides the "make" scaffolding command: business-domain
// skeletons and empty SQL migrations.
package makecmd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/skyrix-lib/framework/cobra/console"
	"gitlab.com/skyrix-lib/framework/database/migrate"
	"gitlab.com/skyrix-lib/framework/database/migrate/atlas"
	support "gitlab.com/skyrix-lib/framework/scaffold"

	"github.com/spf13/cobra"
)

// DefaultDomainDir is where business domains live, relative to the project root.
const DefaultDomainDir = "internal/domain"

// MakeCommand creates business-domain scaffolding and migration files.
type MakeCommand struct {
	ProjectRoot string
	// DomainDir is the domain directory relative to the project root;
	// empty means DefaultDomainDir.
	DomainDir string
	Now       func() time.Time
}

// NewMakeCommand constructs the scaffolding command.
func NewMakeCommand() *MakeCommand {
	return &MakeCommand{ProjectRoot: ".", DomainDir: DefaultDomainDir, Now: time.Now}
}

// Command describes the "make" group. Its commands are standalone
// : they only write files in the project.
func (c *MakeCommand) Command() *cobra.Command {
	group := &cobra.Command{Use: "make", Short: "Create project scaffolding"}
	for _, kind := range []string{"domain", "entity", "repository", "service", "uow", "handler", "command", "job", "seeder", "stubs"} {
		kind := kind
		args := cobra.ExactArgs(1)
		use := kind + " <name>"
		if kind == "stubs" {
			args = cobra.NoArgs
			use = kind
		}
		command := &cobra.Command{Use: use, Short: "Generate " + kind, Args: args}
		if kind == "command" {
			command.Flags().Bool("db", false, "use the existing lazy database graph")
		}
		command.RunE = console.Standalone(func(cmd *cobra.Command, args []string) error {
			raw := ""
			if len(args) > 0 {
				raw = args[0]
			}
			db := false
			if kind == "command" {
				db, _ = cmd.Flags().GetBool("db")
			}
			result, err := c.generate(kind, raw, db)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), result)
			return nil
		})
		group.AddCommand(command)
	}
	group.AddCommand(c.migrationCommand())
	return group
}

func (c *MakeCommand) migrationCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "migration <name>",
		Short: "Create an empty SQL migration for hand-written changes",
		Long: `Create database/migrations/<YYYYMMDDHHMMSS>_<name>.sql for SQL that is not generated from
the models (extensions, triggers, functions, data fixes). With --notx the file
is named *.notx.sql and runs outside a transaction.

The command needs no Atlas CLI, account or database: it creates
database/migrations/atlas.sum on the first migration and updates it afterwards with the
official Atlas Go library. When SQL migrations already exist, atlas.sum must be
present and match them; otherwise nothing is written and the command asks for
an explicit "db migrate:hash". After writing SQL into the new file, run
"db migrate:hash" and commit the SQL file and atlas.sum together.`,
		Args: cobra.ExactArgs(1),
		RunE: console.Standalone(func(cmd *cobra.Command, args []string) error {
			noTx, _ := cmd.Flags().GetBool("notx")
			root, err := support.ProjectRoot(c.ProjectRoot)
			if err != nil {
				return err
			}
			now := c.Now
			if now == nil {
				now = time.Now
			}
			version := migrate.NewVersion(now())
			fileName, err := migrate.FileName(version, args[0], noTx)
			if err != nil {
				return err
			}
			existing, err := atlas.MigrationFiles(root)
			if err != nil {
				return err
			}
			for _, name := range existing {
				if strings.HasPrefix(name, version+"_") {
					return fmt.Errorf("migration version %s already exists (%s); retry in a second", version, name)
				}
			}

			op, err := newOperation(root)
			if err != nil {
				return err
			}
			module, err := support.ModulePath(root)
			if err != nil {
				return err
			}
			content, err := renderStub(op, "migration", stubData{Module: module, Use: args[0]}, false)
			if err != nil {
				return err
			}

			workspace, err := atlas.OpenWorkspace(op.root)
			if err != nil {
				return err
			}
			defer workspace.Close()
			out := cmd.OutOrStdout()
			if err := workspace.Verify(); err != nil {
				return err
			}
			if err := workspace.Add(fileName, content); err != nil {
				return err
			}
			sum, err := workspace.Sum()
			if err != nil {
				return fmt.Errorf("update %s/%s; nothing was written: %w", atlas.MigrationsDir, atlas.SumFile, err)
			}
			if err := workspace.Publish([]string{fileName}, sum); err != nil {
				return err
			}
			fmt.Fprintf(out, "migration created: %s\n", filepath.Join(atlas.MigrationsDir, fileName))
			fmt.Fprintf(out, "after editing it, run %q and commit the SQL file with %s/%s\n", atlas.HashCommand, atlas.MigrationsDir, atlas.SumFile)
			return nil
		}),
	}

	command.Flags().Bool("notx", false, "run the migration outside a transaction (e.g. CREATE INDEX CONCURRENTLY)")

	return command
}
