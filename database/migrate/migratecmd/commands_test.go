package migratecmd

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"gitlab.com/skyrix-lib/framework/cobra/console"
	"gitlab.com/skyrix-lib/framework/database/migrate"
)

func TestCommandsValidateFlagsBeforeLoadingSettings(t *testing.T) {
	loads := 0
	commands := New(func() (Settings, error) {
		loads++
		return Settings{}, errors.New("settings must not be loaded")
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
