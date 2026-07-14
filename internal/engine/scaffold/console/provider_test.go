package console

import (
	"testing"

	"skyrix/internal/commands"
	"skyrix/internal/engine/scaffold/db"
	eventbuscmd "skyrix/internal/engine/scaffold/eventbus"
	jobcmd "skyrix/internal/engine/scaffold/job"
	makecmd "skyrix/internal/engine/scaffold/make"
)

func TestNewCommandsRegistersExpectedCommandGroups(t *testing.T) {
	commandSet := NewCommands(
		commands.NewHelloCommand(),
		db.NewDBCommand(&db.DBAutoMigrateCommand{}, &db.DBSeedCommand{}),
		&jobcmd.JobsRunCommand{},
		eventbuscmd.NewEventBusCommand(),
		makecmd.NewMakeCommand(),
	)

	want := []string{"hello", "db", "jobs:run", "eventbus", "make"}
	if len(commandSet.All) != len(want) {
		t.Fatalf("registered commands = %d, want %d", len(commandSet.All), len(want))
	}
	for index, name := range want {
		if got := commandSet.All[index].Name(); got != name {
			t.Fatalf("command %d = %q, want %q", index, got, name)
		}
	}
}
