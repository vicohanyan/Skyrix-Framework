// Package atlas runs the Atlas CLI used to generate SQL migrations.
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
const MigrationsDir = "migrations"

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

// Hash recomputes atlas.sum for the migration directory under root.
func Hash(ctx context.Context, binary, root string, stdout, stderr io.Writer) error {
	return Run(ctx, binary, root, stdout, stderr, "migrate", "hash", "--dir", "file://"+MigrationsDir)
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
