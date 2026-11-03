package migratecmd

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"gitlab.com/skyrix-lib/framework/cobra/console"
	"gitlab.com/skyrix-lib/framework/database/migrate"
	"gitlab.com/skyrix-lib/framework/database/migrate/atlas/atlastest"
)

func TestCommandsValidateFlagsBeforeLoadingSettings(t *testing.T) {
	loads := 0
	commands := New(func() (Settings, func(), error) {
		loads++
		return Settings{}, nil, errors.New("settings must not be loaded")
	}, fstest.MapFS{})

	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"migrate", "--steps", "1", "--to", "20261102135250"}, "cannot be used together"},
		{[]string{"migrate", "--steps", "0"}, "at least 1"},
		{[]string{"migrate", "--to", "2026"}, "YYYYMMDDHHMMSS"},
		{[]string{"migrate:diff", "Bad-Name"}, "lowercase"},
		{[]string{"migrate", "--help"}, ""},
		{[]string{"migrate:status", "--help"}, ""},
		{[]string{"migrate:diff", "--help"}, ""},
	} {
		root := commands.Command().Cobra()
		var output strings.Builder
		root.SetOut(&output)
		root.SetErr(&output)
		root.SetArgs(test.args)
		root.SilenceUsage = true
		err := root.Execute()
		if test.want == "" {
			if err != nil {
				t.Fatalf("%v: %v", test.args, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("%v error = %v, want %q", test.args, err, test.want)
		}
	}
	if loads != 0 {
		t.Fatalf("settings loaded %d times, want 0", loads)
	}
	for _, sub := range commands.Subcommands() {
		if sub.Needs() != console.NeedsNothing {
			t.Fatalf("db %s needs %v, want standalone", sub.Name(), sub.Needs())
		}
	}
}

func TestWriteStatusReportsDisagreement(t *testing.T) {
	var output strings.Builder
	err := writeStatus(&output, migrate.Status{
		Migrations: []migrate.MigrationStatus{{
			Migration: migrate.Migration{Version: "20260101000000", Name: "create", FileName: "20260101000000_create.sql"},
			Applied:   true,
			Record:    migrate.Record{Run: 1, AppVersion: "v1", AppliedAt: time.Now()},
		}},
		Orphans: []migrate.Record{{Version: "20250101000000", Name: "old", Run: 1}},
	})
	if err == nil {
		t.Fatal("writeStatus() error = nil with orphan migrations")
	}
	for _, want := range []string{"20260101000000", "applied", "v1", "20250101000000_old"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("status output does not contain %q:\n%s", want, output.String())
		}
	}
}

func TestSettingsCleanupOnOpenFailure(t *testing.T) {
	calls := 0
	commands := New(func() (Settings, func(), error) {
		// Empty PostgreSQL config fails validation before any network access.
		return Settings{}, func() { calls++ }, nil
	}, fstest.MapFS{})
	runner, cleanup, err := commands.Open(context.Background())
	if err == nil || runner != nil || cleanup != nil {
		t.Fatalf("Open() = %v, %v, %v", runner, cleanup != nil, err)
	}
	if calls != 1 {
		t.Fatalf("cleanup calls = %d, want 1", calls)
	}
}

func TestSettingsCleanupAfterAtlasOptionsLookup(t *testing.T) {
	for _, configured := range []string{"", "production"} {
		t.Run(configured, func(t *testing.T) {
			calls := 0
			commands := New(func() (Settings, func(), error) {
				return Settings{AtlasEnv: configured, AtlasDevURL: "docker://postgres/18/scratch"}, func() { calls++ }, nil
			}, fstest.MapFS{})
			options, err := commands.atlasOptions("")
			want := configured

			if err != nil || options.Env != want || options.DevURL != "docker://postgres/18/scratch" {
				t.Fatalf("atlasOptions() = %+v, %v", options, err)
			}
			if calls != 1 {
				t.Fatalf("cleanup calls = %d, want 1", calls)
			}
		})
	}
}

func diffCommands(root string, loads, closes *int) *Commands {
	commands := New(func() (Settings, func(), error) {
		*loads++
		return Settings{AtlasEnv: "local", AtlasDevURL: "docker://postgres/18/configured"}, func() { *closes++ }, nil
	}, fstest.MapFS{})
	commands.ProjectRoot = root
	return commands
}

func runDiff(commands *Commands, args ...string) (string, error) {
	command := commands.Command().Cobra()
	var output strings.Builder
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs(append([]string{"migrate:diff"}, args...))
	command.SilenceUsage = true
	command.SilenceErrors = true
	err := command.Execute()
	return output.String(), err
}

func writeProjectFile(t *testing.T, root, relative, data string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiffLoadsConfiguredDevURLWithExplicitEnv(t *testing.T) {
	_, log := atlastest.Install(t)
	t.Setenv("FAKE_ATLAS_SQL", "CREATE TABLE t (id int);")
	configCopy := filepath.Join(t.TempDir(), "config.hcl")
	t.Setenv("FAKE_ATLAS_CONFIG", configCopy)
	root := t.TempDir()
	loads, closes := 0, 0
	output, err := runDiff(diffCommands(root, &loads, &closes), "add_field", "--env", "production")
	if err != nil {
		t.Fatal(err, output)
	}
	config, err := os.ReadFile(configCopy)
	if err != nil {
		t.Fatal(err)
	}
	// Atlas writes into a private copy, never into the project directory.
	if !strings.Contains(string(config), `env "production"`) || !strings.Contains(string(config), "docker://postgres/18/configured") ||
		strings.Contains(string(config), filepath.Join(root, "migrations")) {
		t.Fatal(string(config))
	}
	calls := atlastest.Invocations(t, log)
	if len(calls) != 2 || !strings.HasPrefix(calls[0], "migrate diff add_field --config file://") || !strings.HasSuffix(calls[0], "--env production") {
		t.Fatalf("atlas calls = %q", calls)
	}
	if loads != 1 || closes != 1 {
		t.Fatalf("loads=%d closes=%d", loads, closes)
	}
	if !strings.Contains(output, "created migrations/20261103000000_add_field.sql") {
		t.Fatal(output)
	}
}

func TestDiffFirstAndNextGeneration(t *testing.T) {
	_, log := atlastest.Install(t)
	root := t.TempDir()
	writeProjectFile(t, root, "migrations/README.md", "user placeholder")
	loads, closes := 0, 0
	commands := diffCommands(root, &loads, &closes)

	// No schema changes: nothing is written, not even atlas.sum.
	before := atlastest.Snapshot(t, root)
	if output, err := runDiff(commands, "noop"); err != nil || !strings.Contains(output, "no schema changes") {
		t.Fatalf("diff without changes = %q, %v", output, err)
	}
	if after := atlastest.Snapshot(t, root); !maps.Equal(before, after) {
		t.Fatalf("project changed without schema changes: %v", after)
	}

	// The first generation on an empty directory creates the migration and atlas.sum.
	t.Setenv("FAKE_ATLAS_SQL", "CREATE TABLE a (id int);")
	if _, err := runDiff(commands, "create_a"); err != nil {
		t.Fatal(err)
	}
	first := atlastest.Snapshot(t, root)
	if first["migrations/20261103000000_create_a.sql"] == "" || !strings.Contains(first["migrations/atlas.sum"], "20261103000000_create_a.sql") {
		t.Fatalf("after first generation: %v", first)
	}

	// The next generation verifies the checksum first, then updates it.
	t.Setenv("FAKE_ATLAS_SQL", "CREATE TABLE b (id int);")
	t.Setenv("FAKE_ATLAS_VERSION", "20261103000001")
	calls := len(atlastest.Invocations(t, log))
	if _, err := runDiff(commands, "create_b"); err != nil {
		t.Fatal(err)
	}
	next := atlastest.Invocations(t, log)[calls:]
	if len(next) != 3 || !strings.HasPrefix(next[0], "migrate hash") || !strings.HasPrefix(next[1], "migrate diff") || !strings.HasPrefix(next[2], "migrate hash") {
		t.Fatalf("second generation calls = %q", next)
	}
	second := atlastest.Snapshot(t, root)
	if second["migrations/atlas.sum"] == first["migrations/atlas.sum"] || second["migrations/README.md"] != "user placeholder" {
		t.Fatalf("after second generation: %v", second)
	}
}

func TestDiffWritesNothingOnError(t *testing.T) {
	_, log := atlastest.Install(t)
	for _, test := range []struct {
		name     string
		setup    func(t *testing.T, root string)
		want     string
		noAtlas  bool
		diffRuns bool
	}{
		{
			name: "checksum missing",
			setup: func(t *testing.T, root string) {
				writeProjectFile(t, root, "migrations/20261101000000_existing.sql", "SELECT 1;")
			},
			want: "atlas migrate hash --dir file://migrations",
		},
		{
			name: "checksum mismatch",
			setup: func(t *testing.T, root string) {
				writeProjectFile(t, root, "migrations/20261101000000_existing.sql", "SELECT 1;")
				writeProjectFile(t, root, "migrations/atlas.sum", "h1:stale\n")
			},
			want: "does not match",
		},
		{
			name:     "diff fails after writing a file",
			setup:    func(t *testing.T, root string) { t.Setenv("FAKE_ATLAS_FAIL", "diff") },
			want:     "nothing was written",
			diffRuns: true,
		},
		{
			name: "hash fails",
			setup: func(t *testing.T, root string) {
				t.Setenv("FAKE_ATLAS_SQL", "CREATE TABLE a (id int);")
				t.Setenv("FAKE_ATLAS_FAIL", "hash")
			},
			want:     "nothing was written",
			diffRuns: true,
		},
		{
			name:    "atlas missing",
			noAtlas: true,
			want:    "atlas CLI is not installed",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("FAKE_ATLAS_FAIL", "")
			t.Setenv("FAKE_ATLAS_SQL", "")
			if err := os.WriteFile(log, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			writeProjectFile(t, root, "migrations/README.md", "user placeholder")
			if test.setup != nil {
				test.setup(t, root)
			}
			if test.noAtlas {
				t.Setenv("PATH", t.TempDir())
			}
			before := atlastest.Snapshot(t, root)
			loads, closes := 0, 0
			_, err := runDiff(diffCommands(root, &loads, &closes), "add_field")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("diff error = %v, want %q", err, test.want)
			}
			if after := atlastest.Snapshot(t, root); !maps.Equal(before, after) {
				t.Fatalf("project changed:\nbefore %v\nafter  %v", before, after)
			}
			ranDiff := strings.Contains(strings.Join(atlastest.Invocations(t, log), "\n"), "migrate diff")
			if ranDiff != test.diffRuns {
				t.Fatalf("atlas migrate diff ran = %v, want %v", ranDiff, test.diffRuns)
			}
			if loads != closes {
				t.Fatalf("loads=%d closes=%d", loads, closes)
			}
		})
	}
}
