package scaffold

import (
	"testing"

	eventbuscmd "gitlab.com/skyrix-lib/framework/cobra/eventbus"
	makecmd "gitlab.com/skyrix-lib/framework/cobra/make"
	"gitlab.com/skyrix-lib/template/internal/scaffold/db"
	jobcmd "gitlab.com/skyrix-lib/template/internal/scaffold/job"
)

func TestNewCommandsRegistersFrameworkCommandGroups(t *testing.T) {
	commandSet := NewCommands(
		db.NewDBCommand(&db.DBAutoMigrateCommand{}, &db.DBSeedCommand{}),
		&jobcmd.JobsRunCommand{},
		eventbuscmd.NewEventBusCommand(),
		makecmd.NewMakeCommand(),
	)

	want := []string{"db", "jobs:run", "eventbus", "make"}
	if len(commandSet.All) != len(want) {
		t.Fatalf("registered commands = %d, want %d", len(commandSet.All), len(want))
	}
	for index, name := range want {
		if got := commandSet.All[index].Name(); got != name {
			t.Fatalf("command %d = %q, want %q", index, got, name)
		}
	}
}
