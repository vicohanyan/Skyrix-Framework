// Package atlas maintains the Atlas checksum file (atlas.sum) of the SQL
// migration directory with the official Atlas Go library, and runs the
// optional Atlas CLI that generates migrations from the models
// ("db migrate:diff"). Only that generation needs the CLI; hand-written
// migrations, "make migration", "db migrate:hash" and the migration runner
// do not.
package atlas

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"gitlab.com/skyrix-lib/framework/database/migrate"
)

// MigrationsDir is the migration directory relative to the project root.
const MigrationsDir = "database/migrations"

// ErrNotInstalled reports that the atlas binary is not in PATH.
var ErrNotInstalled = errors.New("atlas CLI is not installed or not in PATH; install it with `curl -sSf https://atlasgo.sh | sh` (see https://atlasgo.io/getting-started)")

// Lookup returns the path of the atlas binary.
func Lookup() (string, error) {
	path, err := exec.LookPath("atlas")
	if err != nil {
		return "", ErrNotInstalled
	}
	return path, nil
}

// Run executes atlas with args in dir.
func Run(ctx context.Context, binary, dir string, stdout, stderr io.Writer, args ...string) error {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("atlas %v: %w", args, err)
	}
	return nil
}

// MigrationFiles returns the names of the migration files in the migration
// directory under root, excluding atlas.sum and non-SQL files.
func MigrationFiles(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, MigrationsDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read migrations directory: %w", err)
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)
	return files, nil
}

// NewFiles returns the files in after that are not in before.
func NewFiles(before, after []string) []string {
	known := make(map[string]struct{}, len(before))
	for _, name := range before {
		known[name] = struct{}{}
	}
	var created []string
	for _, name := range after {
		if _, ok := known[name]; !ok {
			created = append(created, name)
		}
	}
	return created
}

// ValidateFiles checks that every file name follows the migration format.
func ValidateFiles(files []string) error {
	var invalid []error
	for _, name := range files {
		if _, _, _, err := migrate.ParseFileName(name); err != nil {
			invalid = append(invalid, err)
		}
	}
	return errors.Join(invalid...)
}
