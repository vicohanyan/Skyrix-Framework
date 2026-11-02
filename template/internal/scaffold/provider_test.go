package scaffold

import (
	"testing"

	"gitlab.com/skyrix-lib/framework/database/migrate/migratecmd"
	"gitlab.com/skyrix-lib/framework/engine/jobs/jobscmd"
	"gitlab.com/skyrix-lib/framework/scaffold/makecmd"
	"gitlab.com/skyrix-lib/template/internal/scaffold/db"
)

func TestNewCommandsRegistersFrameworkCommandGroups(t *testing.T) {
	commandSet := NewCommands(
		db.NewDBCommand(&db.DBAutoMigrateCommand{}, &db.DBSeedCommand{}, migratecmd.New(nil, nil)),
		&jobscmd.JobsRunCommand{},
		makecmd.NewMakeCommand(),
	)

	want := []string{"db", "jobs:run", "make"}
	if len(commandSet.All) != len(want) {
		t.Fatalf("registered commands = %d, want %d", len(commandSet.All), len(want))
	}
	for index, name := range want {
		if got := commandSet.All[index].Name(); got != name {
			t.Fatalf("command %d = %q, want %q", index, got, name)
		}
	}
}
