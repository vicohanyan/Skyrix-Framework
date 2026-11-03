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
	"github.com/spf13/pflag"
)

// DefaultDomainDir is where business domains live, relative to the project root.
const DefaultDomainDir = "internal/domain"

// MakeCommand creates business-domain scaffolding and migration files.
type MakeCommand struct {
	ProjectRoot string
	// DomainDir is the domain directory relative to the project root;
	// empty means DefaultDomainDir.
	DomainDir   string
	Now         func() time.Time
	LookupAtlas func() (string, error)
}

// NewMakeCommand constructs the scaffolding command.
func NewMakeCommand() *MakeCommand {
	return &MakeCommand{ProjectRoot: ".", DomainDir: DefaultDomainDir, Now: time.Now, LookupAtlas: atlas.Lookup}
}

// Command describes the "make" group. Its commands are standalone
// (console.NeedsNothing): they only write files in the project.
func (c *MakeCommand) Command() *console.Command {
	group := &console.Command{Use: "make", Short: "Create project scaffolding"}
	for _, kind := range []string{"domain", "entity", "repository", "service", "uow", "handler", "command", "job", "seeder", "stubs"} {
		kind := kind
		args := cobra.ExactArgs(1)
		use := kind + " <name>"
		if kind == "stubs" {
			args = cobra.NoArgs
			use = kind
		}
		command := &console.Command{Use: use, Short: "Generate " + kind, Args: args}
		if kind == "command" {
			command.Flags = func(f *pflag.FlagSet) { f.Bool("db", false, "use the existing lazy database graph") }
		}
		command.Run = console.Standalone(func(cmd *cobra.Command, args []string) error {
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
		group.Commands = append(group.Commands, command)
	}
	group.Commands = append(group.Commands, c.migrationCommand())
	return group
}

func (c *MakeCommand) migrationCommand() *console.Command {
	return &console.Command{
		Use:   "migration <name>",
		Short: "Create an empty SQL migration for hand-written changes",
		Long: `Create migrations/<YYYYMMDDHHMMSS>_<name>.sql for SQL that is not generated from
the models (extensions, triggers, functions, data fixes). With --notx the file
is named *.notx.sql and runs outside a transaction.

The command needs the Atlas CLI: it creates migrations/atlas.sum on the first
migration and updates it with "atlas migrate hash" afterwards. When SQL
migrations already exist, atlas.sum must be present and match them; otherwise
nothing is written and the command asks for an explicit
"atlas migrate hash --dir file://migrations". After writing SQL into the new
file, run that command and commit the SQL file and atlas.sum together.`,
		Args: cobra.ExactArgs(1),
		Flags: func(flags *pflag.FlagSet) {
			flags.Bool("notx", false, "run the migration outside a transaction (e.g. CREATE INDEX CONCURRENTLY)")
		},
		Run: console.Standalone(func(cmd *cobra.Command, args []string) error {
			noTx, _ := cmd.Flags().GetBool("notx")
			root, err := support.ProjectRoot(c.ProjectRoot)
			if err != nil {
				return err
			}
			now, lookupAtlas := c.Now, c.LookupAtlas
			if now == nil {
				now = time.Now
			}
			if lookupAtlas == nil {
				lookupAtlas = atlas.Lookup
			}
			version := migrate.NewVersion(now())
			fileName, err := migrate.FileName(version, args[0], noTx)
			if err != nil {
				return err
			}
			binary, err := lookupAtlas()
			if err != nil {
				return fmt.Errorf("make migration needs the Atlas CLI to maintain %s/%s; nothing was written: %w", atlas.MigrationsDir, atlas.SumFile, err)
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
			out, errOut := cmd.OutOrStdout(), cmd.ErrOrStderr()
			if err := workspace.Verify(cmd.Context(), binary, out, errOut); err != nil {
				return err
			}
			if err := workspace.Add(fileName, content); err != nil {
				return err
			}
			sum, err := workspace.Hash(cmd.Context(), binary, out, errOut)
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
}
