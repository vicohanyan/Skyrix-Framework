// Package makecmd provides the "make" scaffolding command: business-domain
// skeletons and empty SQL migrations.
package makecmd

import (
	"fmt"
	"os"
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
	return &console.Command{
		Use:   "make",
		Short: "Create project scaffolding",
		Commands: []*console.Command{
			{
				Use:   "domain <Name>",
				Short: "Create a business-domain directory skeleton",
				Args:  cobra.ExactArgs(1),
				Run: console.Standalone(func(cmd *cobra.Command, args []string) error {
					name, root, err := c.resolve(args[0])
					if err != nil {
						return err
					}
					domainRoot := c.domainDirectory(root, name.Package)
					if _, err := os.Stat(domainRoot); err == nil {
						return fmt.Errorf("domain already exists: %s", name.Package)
					} else if !os.IsNotExist(err) {
						return fmt.Errorf("inspect domain: %w", err)
					}

					directories := []struct {
						path        string
						packageName string
					}{
						{"dto", "dto"},
						{"entity", "entity"},
						{"interfaces", "interfaces"},
						{"repository", "repository"},
						{"services", "services"},
						{"unitOfWork", "unitofwork"},
					}
					if err := os.MkdirAll(domainRoot, 0o755); err != nil {
						return fmt.Errorf("create domain directory: %w", err)
					}
					provider := fmt.Sprintf(`// Package %s contains the %s business domain.
				package %s
				
				import "github.com/goforj/wire"
				
				// ProviderSet contains the domain providers.
				var ProviderSet = wire.NewSet()
				`, name.Package, name.Type, name.Package)
					if err := support.WriteNewFile(filepath.Join(domainRoot, "provider.go"), []byte(provider)); err != nil {
						_ = os.RemoveAll(domainRoot)
						return err
					}
					for _, directory := range directories {
						doc := fmt.Sprintf("// Package %s contains %s domain scaffolding.\npackage %s\n", directory.packageName, name.Type, directory.packageName)
						if err := support.WriteNewFile(filepath.Join(domainRoot, directory.path, "doc.go"), []byte(doc)); err != nil {
							_ = os.RemoveAll(domainRoot)
							return err
						}
					}
					fmt.Fprintf(cmd.OutOrStdout(), "domain created: %s\n", name.Package)
					return nil
				}),
			},
			{
				Use:   "repository <Name>",
				Short: "Create a minimal domain repository skeleton",
				Args:  cobra.ExactArgs(1),
				Run: console.Standalone(func(cmd *cobra.Command, args []string) error {
					name, root, err := c.resolve(args[0])
					if err != nil {
						return err
					}
					if err := c.requireDomain(root, name.Package); err != nil {
						return err
					}
					content := fmt.Sprintf(`package repository

				// %sRepository provides persistence scaffolding for the %s domain.
				type %sRepository struct{}
				
				// New%sRepository constructs a %s repository.
				func New%sRepository() *%sRepository {
					return &%sRepository{}
				}
				`, name.Type, name.Type, name.Type, name.Type, name.Type, name.Type, name.Type, name.Type)
					path := filepath.Join(c.domainDirectory(root, name.Package), "repository", name.Package+"Repository.go")
					if err := support.WriteNewFile(path, []byte(content)); err != nil {
						return err
					}
					fmt.Fprintf(cmd.OutOrStdout(), "repository created: %s\n", name.Package)
					return nil
				}),
			},
			{
				Use:   "service <Name>",
				Short: "Create a minimal domain service skeleton",
				Args:  cobra.ExactArgs(1),
				Run: console.Standalone(func(cmd *cobra.Command, args []string) error {
					name, root, err := c.resolve(args[0])
					if err != nil {
						return err
					}
					if err := c.requireDomain(root, name.Package); err != nil {
						return err
					}
					content := fmt.Sprintf(`package services

				// %sService provides application-service scaffolding for the %s domain.
				type %sService struct{}
				
				// New%sService constructs a %s service.
				func New%sService() *%sService {
					return &%sService{}
				}
				`, name.Type, name.Type, name.Type, name.Type, name.Type, name.Type, name.Type, name.Type)
					path := filepath.Join(c.domainDirectory(root, name.Package), "services", name.Package+"Service.go")
					if err := support.WriteNewFile(path, []byte(content)); err != nil {
						return err
					}
					fmt.Fprintf(cmd.OutOrStdout(), "service created: %s\n", name.Package)
					return nil
				}),
			},
			c.migrationCommand(),
		},
	}
}

func (c *MakeCommand) migrationCommand() *console.Command {
	return &console.Command{
		Use:   "migration <name>",
		Short: "Create an empty SQL migration for hand-written changes",
		Long: `Create migrations/<YYYYMMDDHHMMSS>_<name>.sql for SQL that is not generated from
the models (extensions, triggers, functions, data fixes). With --notx the file
is named *.notx.sql and runs outside a transaction. When atlas is installed,
atlas.sum is updated with "atlas migrate hash".`,
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
			existing, err := atlas.MigrationFiles(root)
			if err != nil {
				return err
			}
			for _, name := range existing {
				if strings.HasPrefix(name, version+"_") {
					return fmt.Errorf("migration version %s already exists (%s); retry in a second", version, name)
				}
			}

			relative := filepath.Join(atlas.MigrationsDir, fileName)
			if err := support.WriteNewFile(filepath.Join(root, relative), nil); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "migration created: %s\n", relative)

			binary, err := lookupAtlas()
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "atlas.sum not updated: %v\nrun \"atlas migrate hash --dir file://%s\" after installing atlas\n", err, atlas.MigrationsDir)
				return nil
			}
			if err := atlas.Hash(cmd.Context(), binary, root, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
				return fmt.Errorf("update atlas.sum: %w", err)
			}
			return nil
		}),
	}
}

func (c *MakeCommand) resolve(raw string) (support.Name, string, error) {
	name, err := support.NormalizeName(raw)
	if err != nil {
		return support.Name{}, "", err
	}
	root, err := support.ProjectRoot(c.ProjectRoot)
	return name, root, err
}

func (c *MakeCommand) domainDirectory(root, name string) string {
	dir := c.DomainDir
	if dir == "" {
		dir = DefaultDomainDir
	}
	return filepath.Join(root, filepath.FromSlash(dir), name)
}

func (c *MakeCommand) requireDomain(root, name string) error {
	info, err := os.Stat(c.domainDirectory(root, name))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("domain %s does not exist; run make domain %s first", name, name)
		}
		return fmt.Errorf("inspect domain %s: %w", name, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("domain path is not a directory: %s", c.domainDirectory(root, name))
	}
	return nil
}
