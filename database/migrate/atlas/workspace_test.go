package atlas_test

import (
	"context"
	"errors"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/skyrix-lib/framework/database/migrate/atlas"
	"gitlab.com/skyrix-lib/framework/database/migrate/atlas/atlastest"
)

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// generate runs the generator flow used by make migration: verify, add one
// file, hash and publish.
func generate(t *testing.T, binary, root, name, sql string) error {
	t.Helper()
	workspace, err := atlas.OpenWorkspace(root)
	if err != nil {
		return err
	}
	defer workspace.Close()
	ctx := context.Background()
	if err := workspace.Verify(ctx, binary, io.Discard, io.Discard); err != nil {
		return err
	}
	if err := workspace.Add(name, []byte(sql)); err != nil {
		return err
	}
	sum, err := workspace.Hash(ctx, binary, io.Discard, io.Discard)
	if err != nil {
		return err
	}
	return workspace.Publish([]string{name}, sum)
}

func TestWorkspaceFirstAndNextGeneration(t *testing.T) {
	binary, log := atlastest.Install(t)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "migrations", "README.md"), "placeholder")

	if err := generate(t, binary, root, "20261103000000_first.sql", "SELECT 1;"); err != nil {
		t.Fatal(err)
	}
	first := atlastest.Snapshot(t, root)
	if first["migrations/atlas.sum"] == "" || first["migrations/20261103000000_first.sql"] != "SELECT 1;" || first["migrations/README.md"] != "placeholder" {
		t.Fatalf("after first generation: %v", first)
	}
	// An empty directory has no checksum to verify: only the hash of the new file runs.
	if calls := atlastest.Invocations(t, log); len(calls) != 1 || !strings.HasPrefix(calls[0], "migrate hash") {
		t.Fatalf("first generation calls = %q", calls)
	}

	if err := generate(t, binary, root, "20261103000001_second.sql", "SELECT 2;"); err != nil {
		t.Fatal(err)
	}
	second := atlastest.Snapshot(t, root)
	if second["migrations/atlas.sum"] == first["migrations/atlas.sum"] || second["migrations/20261103000000_first.sql"] != "SELECT 1;" {
		t.Fatalf("after second generation: %v", second)
	}
	// Verification (hash of the existing files) precedes the update.
	if calls := atlastest.Invocations(t, log); len(calls) != 3 {
		t.Fatalf("second generation calls = %q", calls)
	}
	if strings.Contains(strings.Join(fileNames(second), " "), ".skyrix-migration-") {
		t.Fatal("temporary files remain")
	}
}

func fileNames(snapshot map[string]string) []string {
	var names []string
	for name := range snapshot {
		names = append(names, name)
	}
	return names
}

func TestWorkspaceRejectsMissingOrStaleChecksum(t *testing.T) {
	binary, _ := atlastest.Install(t)
	for _, test := range []struct {
		name string
		sum  string
		want error
	}{
		{"missing", "", atlas.ErrSumMissing},
		{"mismatch", "h1:stale\n", atlas.ErrSumMismatch},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "migrations", "20261101000000_existing.sql"), "SELECT 0;")
			if test.sum != "" {
				writeFile(t, filepath.Join(root, "migrations", "atlas.sum"), test.sum)
			}
			before := atlastest.Snapshot(t, root)
			err := generate(t, binary, root, "20261103000000_next.sql", "SELECT 1;")
			if !errors.Is(err, test.want) || !strings.Contains(err.Error(), atlas.HashCommand) {
				t.Fatalf("error = %v, want %v with the explicit hash command", err, test.want)
			}
			if after := atlastest.Snapshot(t, root); !maps.Equal(before, after) {
				t.Fatalf("project changed:\nbefore %v\nafter  %v", before, after)
			}
		})
	}
}

func TestWorkspaceHashFailureWritesNothing(t *testing.T) {
	binary, _ := atlastest.Install(t)
	root := t.TempDir()
	if err := generate(t, binary, root, "20261103000000_first.sql", "SELECT 1;"); err != nil {
		t.Fatal(err)
	}
	before := atlastest.Snapshot(t, root)
	t.Setenv("FAKE_ATLAS_FAIL", "hash")
	if err := generate(t, binary, root, "20261103000001_second.sql", "SELECT 2;"); err == nil {
		t.Fatal("generation succeeded with a failing atlas migrate hash")
	}
	if after := atlastest.Snapshot(t, root); !maps.Equal(before, after) {
		t.Fatalf("project changed:\nbefore %v\nafter  %v", before, after)
	}
}

func TestWorkspacePublishDetectsConcurrentChanges(t *testing.T) {
	binary, _ := atlastest.Install(t)
	root := t.TempDir()
	workspace, err := atlas.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	defer workspace.Close()
	if err := workspace.Add("20261103000000_first.sql", []byte("SELECT 1;")); err != nil {
		t.Fatal(err)
	}
	sum, err := workspace.Hash(context.Background(), binary, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "migrations", "20261103000005_user.sql"), "SELECT 5;")
	before := atlastest.Snapshot(t, root)
	if err := workspace.Publish([]string{"20261103000000_first.sql"}, sum); err == nil || !strings.Contains(err.Error(), "changed during generation") {
		t.Fatalf("Publish() error = %v", err)
	}
	if after := atlastest.Snapshot(t, root); !maps.Equal(before, after) {
		t.Fatalf("project changed:\nbefore %v\nafter  %v", before, after)
	}
}

func TestWorkspaceRejectsSymlinkedDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "migrations")); err != nil {
		t.Fatal(err)
	}
	if _, err := atlas.OpenWorkspace(root); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("OpenWorkspace() error = %v", err)
	}
}

// TestRealAtlasChecksum runs the flow against the installed Atlas CLI. Without
// atlas in PATH it is skipped: the fake does not prove the CLI integration.
func TestRealAtlasChecksum(t *testing.T) {
	binary, err := exec.LookPath("atlas")
	if err != nil {
		t.Skip("atlas CLI is not installed; the real checksum integration is not verified")
	}
	root := t.TempDir()
	if err := generate(t, binary, root, "20261103000000_first.sql", "CREATE TABLE a (id int);"); err != nil {
		t.Fatal(err)
	}
	if err := generate(t, binary, root, "20261103000001_second.sql", "CREATE TABLE b (id int);"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "migrations", "20261103000000_first.sql"), "CREATE TABLE a (id bigint);")
	if err := generate(t, binary, root, "20261103000002_third.sql", "SELECT 1;"); !errors.Is(err, atlas.ErrSumMismatch) {
		t.Fatalf("edited migration error = %v, want ErrSumMismatch", err)
	}
}
