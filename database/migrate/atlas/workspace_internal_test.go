package atlas

import (
	"context"
	"errors"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/skyrix-lib/framework/database/migrate/atlas/atlastest"
)

// TestPublishRollsBackOnlyItsOwnChanges fails each publish step and checks
// that the files created by the operation are removed while atlas.sum and
// the user's files keep their contents.
func TestPublishRollsBackOnlyItsOwnChanges(t *testing.T) {
	binary, _ := atlastest.Install(t)
	for _, test := range []struct {
		name     string
		existing bool // project already has a migration and atlas.sum
		failAt   string
	}{
		{"new directory, sum fails", false, SumFile},
		{"new directory, second file fails", false, "20261103000001_b.sql"},
		{"existing directory, sum fails", true, SumFile},
		{"existing directory, first file fails", true, "20261103000000_a.sql"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "user.txt"), []byte("user"), 0o644); err != nil {
				t.Fatal(err)
			}
			if test.existing {
				seed, err := OpenWorkspace(root)
				if err != nil {
					t.Fatal(err)
				}
				if err := seed.Add("20261101000000_seed.sql", []byte("SELECT 0;")); err != nil {
					t.Fatal(err)
				}
				sum, err := seed.Hash(context.Background(), binary, io.Discard, io.Discard)
				if err != nil {
					t.Fatal(err)
				}
				if err := seed.Publish([]string{"20261101000000_seed.sql"}, sum); err != nil {
					t.Fatal(err)
				}
				seed.Close()
				if err := os.WriteFile(filepath.Join(root, MigrationsDir, "README.md"), []byte("placeholder"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			before := atlastest.Snapshot(t, root)

			workspace, err := OpenWorkspace(root)
			if err != nil {
				t.Fatal(err)
			}
			defer workspace.Close()
			ctx := context.Background()
			if err := workspace.Verify(ctx, binary, io.Discard, io.Discard); err != nil {
				t.Fatal(err)
			}
			created := []string{"20261103000000_a.sql", "20261103000001_b.sql"}
			for _, name := range created {
				if err := workspace.Add(name, []byte("SELECT 1;")); err != nil {
					t.Fatal(err)
				}
			}
			sum, err := workspace.Hash(ctx, binary, io.Discard, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			injected := errors.New("injected publish failure")
			workspace.publish = func(temp, path string, exists bool) error {
				if filepath.Base(path) == test.failAt {
					return injected
				}
				return publishFile(temp, path, exists)
			}
			if err := workspace.Publish(created, sum); !errors.Is(err, injected) {
				t.Fatalf("Publish() error = %v", err)
			}
			if after := atlastest.Snapshot(t, root); !maps.Equal(before, after) {
				t.Fatalf("project changed:\nbefore %v\nafter  %v", before, after)
			}
		})
	}
}

func TestDiffUsesWorkspaceDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "migrations")
	config := diffConfig("/project", DiffOptions{Env: "local", DevURL: DefaultDevURL, Dir: dir})
	if want := hclString(fileURL(dir)); !strings.Contains(config, "dir = "+want) {
		t.Fatalf("config does not use %s:\n%s", dir, config)
	}
}
