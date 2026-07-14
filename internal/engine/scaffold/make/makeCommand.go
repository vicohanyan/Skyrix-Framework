package make

import (
	"fmt"
	"os"
	"path/filepath"

	"skyrix/internal/engine/scaffold/internal/support"

	"github.com/spf13/cobra"
)

// MakeCommand creates business-domain scaffolding.
type MakeCommand struct {
	ProjectRoot string
}

// NewMakeCommand constructs the business-domain scaffolding command.
func NewMakeCommand() *MakeCommand {
	return &MakeCommand{ProjectRoot: "."}
}

// ToCobraCommand converts MakeCommand into a Cobra command tree.
func (c *MakeCommand) ToCobraCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "make",
		Short: "Create project scaffolding",
	}
	command.AddCommand(
		&cobra.Command{
			Use:   "domain <Name>",
			Short: "Create a business-domain directory skeleton",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				name, root, err := c.resolve(args[0])
				if err != nil {
					return err
				}
				domainRoot := domainRoot(root, name.Package)
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
				
				import "github.com/google/wire"
				
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
			},
		},
		&cobra.Command{
			Use:   "repository <Name>",
			Short: "Create a minimal domain repository skeleton",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				name, root, err := c.resolve(args[0])
				if err != nil {
					return err
				}
				if err := requireDomain(root, name.Package); err != nil {
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
				path := filepath.Join(domainRoot(root, name.Package), "repository", name.Package+"Repository.go")
				if err := support.WriteNewFile(path, []byte(content)); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "repository created: %s\n", name.Package)
				return nil
			},
		},
		&cobra.Command{
			Use:   "service <Name>",
			Short: "Create a minimal domain service skeleton",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				name, root, err := c.resolve(args[0])
				if err != nil {
					return err
				}
				if err := requireDomain(root, name.Package); err != nil {
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
				path := filepath.Join(domainRoot(root, name.Package), "services", name.Package+"Service.go")
				if err := support.WriteNewFile(path, []byte(content)); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "service created: %s\n", name.Package)
				return nil
			},
		},
	)
	return command
}

func (c *MakeCommand) resolve(raw string) (support.Name, string, error) {
	name, err := support.NormalizeName(raw)
	if err != nil {
		return support.Name{}, "", err
	}
	root, err := support.ProjectRoot(c.ProjectRoot)
	return name, root, err
}

func domainRoot(root, name string) string {
	return filepath.Join(root, "internal", "domain", name)
}

func requireDomain(root, name string) error {
	info, err := os.Stat(domainRoot(root, name))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("domain %s does not exist; run make domain %s first", name, name)
		}
		return fmt.Errorf("inspect domain %s: %w", name, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("domain path is not a directory: %s", domainRoot(root, name))
	}
	return nil
}
