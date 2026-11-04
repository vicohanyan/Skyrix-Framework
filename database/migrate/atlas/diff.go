package atlas

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// DefaultDevURL is a disposable development database, never the application DB.
const DefaultDevURL = "docker://postgres/18/dev?search_path=public"

// DiffOptions configure model-based migration generation. Empty values use the
// built-in local environment and disposable PostgreSQL development database.
type DiffOptions struct {
	Env    string
	DevURL string
	// Dir is the absolute migration directory Atlas reads and writes; empty
	// means database/migrations/ under the project root. Generators pass a Workspace
	// copy so that Atlas never writes into the project directly.
	Dir string
}

// Diff generates a migration using a private temporary Atlas configuration.
// The project's atlas.hcl is not read or modified. Paths and schema-loader cwd
// are absolute, so placing the config outside the project does not relocate them.
// The caller must supply a dedicated disposable DevURL if overriding the default:
// Atlas uses that database as scratch space, not as a migration target.
func Diff(ctx context.Context, binary, root, name string, options DiffOptions, stdout, stderr io.Writer) (result error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("atlas project root: %w", err)
	}
	if options.Env == "" {
		options.Env = "local"
	}
	if options.DevURL == "" {
		options.DevURL = DefaultDevURL
	}
	if options.Dir == "" {
		options.Dir = filepath.Join(absolute, MigrationsDir)
	}
	config := diffConfig(absolute, options)
	file, err := os.CreateTemp("", "skyrix-atlas-*.hcl")
	if err != nil {
		return fmt.Errorf("create temporary Atlas config: %w", err)
	}
	defer func() {
		if err := os.Remove(file.Name()); err != nil {
			result = errors.Join(result, fmt.Errorf("remove temporary Atlas config: %w", err))
		}
	}()
	_, writeErr := io.WriteString(file, config)
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("write temporary Atlas config: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close temporary Atlas config: %w", closeErr)
	}
	return Run(ctx, binary, absolute, stdout, stderr, "migrate", "diff", name, "--config", fileURL(file.Name()), "--env", options.Env)
}

func fileURL(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}

func diffConfig(root string, options DiffOptions) string {
	return fmt.Sprintf(`data "external_schema" "gorm" {
  program = ["go", "run", "./tools/schemaloader"]
  working_dir = %s
}
env %s {
  src = data.external_schema.gorm.url
  dev = %s
  migration {
    dir = %s
  }
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}
`, hclString(root), hclString(options.Env), hclString(options.DevURL), hclString(fileURL(options.Dir)))
}

// Quote an HCL literal, including template markers. No config value is evaluated
// as an HCL expression or passed through a shell.
func hclString(value string) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if r < 32 {
				fmt.Fprintf(&out, `\u%04x`, r)
			} else {
				out.WriteRune(r)
			}
		}
	}
	out.WriteByte('"')
	return strings.NewReplacer("${", "$${", "%{", "%%{").Replace(out.String())
}
