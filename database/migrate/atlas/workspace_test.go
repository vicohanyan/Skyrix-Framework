package atlas_test

import (
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	atlasmigrate "ariga.io/atlas/sql/migrate"

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

// withoutAtlasCLI makes sure no atlas binary can be found.
func withoutAtlasCLI(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	if _, err := exec.LookPath("atlas"); err == nil {
		t.Fatal("atlas still in PATH")
	}
}

// generate runs the generator flow used by make migration: verify, add one
// file, compute the checksum and publish.
func generate(t *testing.T, root, name, sql string) error {
	t.Helper()
	workspace, err := atlas.OpenWorkspace(root)
	if err != nil {
		return err
	}
	defer workspace.Close()
	if err := workspace.Verify(); err != nil {
		return err
	}
	if err := workspace.Add(name, []byte(sql)); err != nil {
		return err
	}
	sum, err := workspace.Sum()
	if err != nil {
		return err
	}
	return workspace.Publish([]string{name}, sum)
}

func rehash(t *testing.T, root string) (bool, error) {
	t.Helper()
	workspace, err := atlas.OpenWorkspace(root)
	if err != nil {
		return false, err
	}
	defer workspace.Close()
	return workspace.Rehash()
}

// validate checks the project's atlas.sum with the Atlas library validator,
// the same check "atlas migrate" commands run on a directory.
func validate(t *testing.T, root string) error {
	t.Helper()
	dir, err := atlasmigrate.NewLocalDir(filepath.Join(root, atlas.MigrationsDir))
	if err != nil {
		t.Fatal(err)
	}
	return atlasmigrate.Validate(dir)
}

// TestSumMatchesAtlasFormat pins the atlas.sum format for fixed input. A
// library upgrade that changes it fails here instead of invalidating
// committed checksums silently.
func TestSumMatchesAtlasFormat(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "20261103000000_create_a.sql"), "CREATE TABLE a (id int);\n")
	writeFile(t, filepath.Join(dir, "20261103000001_index.notx.sql"), "CREATE INDEX CONCURRENTLY a_id ON a (id);\n")
	writeFile(t, filepath.Join(dir, "README.md"), "not a migration")
	sum, err := atlas.Sum(dir)
	if err != nil {
		t.Fatal(err)
	}
	const want = "h1:LA5Qy61AJZwVjTtKzdzvgsYEzAYq8p7182vJhMOvjlA=\n" +
		"20261103000000_create_a.sql h1:u97rxnU7PQNMH188B8OihUe2Ve1xmQ2eXk++xvMfKy8=\n" +
		"20261103000001_index.notx.sql h1:C5QvDRsx6BmjaYhLgM4nM0q5fLvhX1/NUvPz2q6uwb0=\n"
	if string(sum) != want {
		t.Fatalf("atlas.sum =\n%s\nwant\n%s", sum, want)
	}
}

func TestWorkspaceFirstAndNextGenerationWithoutAtlasCLI(t *testing.T) {
	withoutAtlasCLI(t)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "migrations", "README.md"), "placeholder")

	if err := generate(t, root, "20261103000000_first.sql", "SELECT 1;"); err != nil {
		t.Fatal(err)
	}
	first := atlastest.Snapshot(t, root)
	if !strings.Contains(first["migrations/atlas.sum"], "20261103000000_first.sql h1:") || first["migrations/20261103000000_first.sql"] != "SELECT 1;" || first["migrations/README.md"] != "placeholder" {
		t.Fatalf("after first generation: %v", first)
	}
	if err := validate(t, root); err != nil {
		t.Fatalf("Atlas rejects the first atlas.sum: %v", err)
	}

	if err := generate(t, root, "20261103000001_second.sql", "SELECT 2;"); err != nil {
		t.Fatal(err)
	}
	second := atlastest.Snapshot(t, root)
	if second["migrations/atlas.sum"] == first["migrations/atlas.sum"] || second["migrations/20261103000000_first.sql"] != "SELECT 1;" {
		t.Fatalf("after second generation: %v", second)
	}
	if err := validate(t, root); err != nil {
		t.Fatalf("Atlas rejects the updated atlas.sum: %v", err)
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
			err := generate(t, root, "20261103000000_next.sql", "SELECT 1;")
			if !errors.Is(err, test.want) || !strings.Contains(err.Error(), atlas.HashCommand) {
				t.Fatalf("error = %v, want %v with the explicit hash command", err, test.want)
			}
			if after := atlastest.Snapshot(t, root); !maps.Equal(before, after) {
				t.Fatalf("project changed:\nbefore %v\nafter  %v", before, after)
			}
		})
	}
}

// TestManualEditNeedsExplicitRehash covers the hand-written SQL process:
// editing a migration breaks the checksum, generation refuses to continue,
// and an explicit Rehash updates only atlas.sum.
func TestManualEditNeedsExplicitRehash(t *testing.T) {
	withoutAtlasCLI(t)
	root := t.TempDir()
	if err := generate(t, root, "20261103000000_first.sql", ""); err != nil {
		t.Fatal(err)
	}
	if changed, err := rehash(t, root); err != nil || changed {
		t.Fatalf("Rehash() of a valid directory = %v, %v", changed, err)
	}
	writeFile(t, filepath.Join(root, "migrations", "20261103000000_first.sql"), "CREATE TABLE a (id int);\n")
	if err := generate(t, root, "20261103000001_second.sql", ""); !errors.Is(err, atlas.ErrSumMismatch) {
		t.Fatalf("generation after a manual edit error = %v, want ErrSumMismatch", err)
	}

	before := atlastest.Snapshot(t, root)
	if changed, err := rehash(t, root); err != nil || !changed {
		t.Fatalf("Rehash() = %v, %v", changed, err)
	}
	after := atlastest.Snapshot(t, root)
	for name, data := range before {
		if name != "migrations/atlas.sum" && after[name] != data {
			t.Fatalf("Rehash changed %s", name)
		}
	}
	if len(after) != len(before) || after["migrations/atlas.sum"] == before["migrations/atlas.sum"] {
		t.Fatalf("Rehash result: %v", after)
	}
	if err := validate(t, root); err != nil {
		t.Fatalf("Atlas rejects the rehashed atlas.sum: %v", err)
	}
	if err := generate(t, root, "20261103000001_second.sql", ""); err != nil {
		t.Fatalf("generation after Rehash: %v", err)
	}
}

func TestRehashEdgeCases(t *testing.T) {
	t.Run("empty directory stays without atlas.sum", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "migrations", "README.md"), "placeholder")
		before := atlastest.Snapshot(t, root)
		if changed, err := rehash(t, root); err != nil || changed {
			t.Fatalf("Rehash() = %v, %v", changed, err)
		}
		if after := atlastest.Snapshot(t, root); !maps.Equal(before, after) {
			t.Fatalf("project changed: %v", after)
		}
	})
	t.Run("missing atlas.sum is created", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "migrations", "20261101000000_existing.sql"), "SELECT 1;")
		if changed, err := rehash(t, root); err != nil || !changed {
			t.Fatalf("Rehash() = %v, %v", changed, err)
		}
		if err := validate(t, root); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("invalid file name writes nothing", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "migrations", "create_a.sql"), "SELECT 1;")
		before := atlastest.Snapshot(t, root)
		if _, err := rehash(t, root); err == nil || !strings.Contains(err.Error(), "nothing was written") {
			t.Fatalf("Rehash() error = %v", err)
		}
		if after := atlastest.Snapshot(t, root); !maps.Equal(before, after) {
			t.Fatalf("project changed: %v", after)
		}
	})
}

func TestWorkspacePublishDetectsConcurrentChanges(t *testing.T) {
	root := t.TempDir()
	workspace, err := atlas.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	defer workspace.Close()
	if err := workspace.Add("20261103000000_first.sql", []byte("SELECT 1;")); err != nil {
		t.Fatal(err)
	}
	sum, err := workspace.Sum()
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

func TestRehashDetectsConcurrentChanges(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "migrations", "20261101000000_existing.sql"), "SELECT 1;")
	workspace, err := atlas.OpenWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	defer workspace.Close()
	writeFile(t, filepath.Join(root, "migrations", "20261103000005_user.sql"), "SELECT 5;")
	before := atlastest.Snapshot(t, root)
	if _, err := workspace.Rehash(); err == nil || !strings.Contains(err.Error(), "changed during generation") {
		t.Fatalf("Rehash() error = %v", err)
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

// TestRealAtlasCLIChecksum compares atlas.sum written by the framework with
// "atlas migrate hash" of the installed Atlas CLI. Without atlas in PATH it is
// skipped: the CLI compatibility is then not verified.
func TestRealAtlasCLIChecksum(t *testing.T) {
	binary, err := exec.LookPath("atlas")
	if err != nil {
		t.Skip("atlas CLI is not installed; compatibility with atlas migrate hash is not verified")
	}
	root := t.TempDir()
	if err := generate(t, root, "20261103000000_first.sql", "CREATE TABLE a (id int);"); err != nil {
		t.Fatal(err)
	}
	if err := generate(t, root, "20261103000001_second.notx.sql", "CREATE TABLE b (id int);"); err != nil {
		t.Fatal(err)
	}
	ours, err := os.ReadFile(filepath.Join(root, "migrations", "atlas.sum"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "migrate", "hash", "--dir", "file://migrations")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("atlas migrate hash: %v\n%s", err, out)
	}
	theirs, err := os.ReadFile(filepath.Join(root, "migrations", "atlas.sum"))
	if err != nil {
		t.Fatal(err)
	}
	if string(ours) != string(theirs) {
		t.Fatalf("atlas.sum differs from atlas migrate hash:\nframework:\n%s\natlas:\n%s", ours, theirs)
	}
}
