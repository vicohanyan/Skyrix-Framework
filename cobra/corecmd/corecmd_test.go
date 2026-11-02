package corecmd_test

import (
	"testing"

	"gitlab.com/skyrix-lib/framework/cobra/console"
	"gitlab.com/skyrix-lib/framework/cobra/corecmd"
	"gitlab.com/skyrix-lib/framework/database/dbcmd"
	"gitlab.com/skyrix-lib/framework/database/migrate/migratecmd"
	engineJobs "gitlab.com/skyrix-lib/framework/engine/jobs"
)

func TestCommandsDescribeFrameworkCommandsAndTheirNeeds(t *testing.T) {
	commands := corecmd.Commands(corecmd.Options{
		Migrations: migratecmd.New(nil, nil),
		Database:   console.Value(console.NeedsDatabase, &dbcmd.Deps{}),
		Jobs:       console.Value[engineJobs.Registry](console.NeedsApp, nil),
	})
	want := []string{"db", "jobs:run", "make", "config:env"}
	if len(commands) != len(want) {
		t.Fatalf("commands = %d, want %d", len(commands), len(want))
	}
	for i, name := range want {
		if got := commands[i].Name(); got != name {
			t.Fatalf("command %d = %q, want %q", i, got, name)
		}
	}

	c := console.New(commands...)
	for _, test := range []struct {
		path  []string
		needs console.Needs
	}{
		{[]string{"db", "migrate"}, console.NeedsNothing},
		{[]string{"db", "migrate:status"}, console.NeedsNothing},
		{[]string{"db", "migrate:baseline"}, console.NeedsNothing},
		{[]string{"db", "migrate:diff"}, console.NeedsNothing},
		{[]string{"db", "automigrate"}, console.NeedsDatabase},
		{[]string{"db", "seed"}, console.NeedsDatabase},
		{[]string{"jobs:run"}, console.NeedsApp},
		{[]string{"config:env"}, console.NeedsNothing},
		{[]string{"make", "domain"}, console.NeedsNothing},
		{[]string{"make", "migration"}, console.NeedsNothing},
	} {
		command := c.Find(test.path...)
		if command == nil {
			t.Fatalf("%q is missing", test.path)
		}
		if command.Needs() != test.needs {
			t.Fatalf("%q needs %v, want %v", test.path, command.Needs(), test.needs)
		}
	}
}
