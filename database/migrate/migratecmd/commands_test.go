package migratecmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"gitlab.com/skyrix-lib/framework/cobra/console"
	"gitlab.com/skyrix-lib/framework/database/migrate"
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

func TestDiffLoadsConfiguredDevURLWithExplicitEnv(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(t.TempDir(), "atlas")
	script := `#!/bin/sh
printf '%s\n' "$@" > atlas-args
cp "${5#file://}" atlas-config
mkdir -p migrations
: > migrations/20261103000000_add_field.sql
`
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Dir(binary)+string(os.PathListSeparator)+os.Getenv("PATH"))
	loads, closes := 0, 0
	commands := New(func() (Settings, func(), error) {
		loads++
		return Settings{AtlasEnv: "local", AtlasDevURL: "docker://postgres/18/configured"}, func() { closes++ }, nil
	}, fstest.MapFS{})
	commands.ProjectRoot = root
	command := commands.Command().Cobra()
	var output strings.Builder
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{"migrate:diff", "add_field", "--env", "production"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile(filepath.Join(root, "atlas-config"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), `env "production"`) || !strings.Contains(string(config), "docker://postgres/18/configured") {
		t.Fatal(string(config))
	}
	if loads != 1 || closes != 1 {
		t.Fatalf("loads=%d closes=%d", loads, closes)
	}
	if !strings.Contains(output.String(), "created migrations/20261103000000_add_field.sql") {
		t.Fatal(output.String())
	}
}
