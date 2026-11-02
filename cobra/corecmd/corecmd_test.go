package corecmd_test

import (
	"testing"

	"gitlab.com/skyrix-lib/framework/cobra/corecmd"
	"gitlab.com/skyrix-lib/framework/database/dbcmd"
	"gitlab.com/skyrix-lib/framework/database/migrate/migratecmd"
	"gitlab.com/skyrix-lib/framework/engine/jobs/jobscmd"
	"gitlab.com/skyrix-lib/framework/scaffold/makecmd"

	"github.com/spf13/cobra"
)

func TestNewRegistersFrameworkCommands(t *testing.T) {
	commands := corecmd.New(
		dbcmd.New(nil, nil, migratecmd.New(nil, nil), nil, nil),
		&jobscmd.JobsRunCommand{},
		makecmd.NewMakeCommand(),
	)
	assertNames(t, commands.All, "db", "jobs:run", "make")

	standalone := corecmd.NewStandalone(migratecmd.New(nil, nil), makecmd.NewMakeCommand())
	assertNames(t, standalone.All, "db", "make")
}

func assertNames(t *testing.T, commands []*cobra.Command, want ...string) {
	t.Helper()
	if len(commands) != len(want) {
		t.Fatalf("commands = %d, want %d", len(commands), len(want))
	}
	for i, name := range want {
		if got := commands[i].Name(); got != name {
			t.Fatalf("command %d = %q, want %q", i, got, name)
		}
	}
}
