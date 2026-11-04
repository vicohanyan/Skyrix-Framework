package dbcmd_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gitlab.com/skyrix-lib/framework/cobra/console"
	"gitlab.com/skyrix-lib/framework/database/dbcmd"
	"gitlab.com/skyrix-lib/framework/database/migrate/migratecmd"
)

func TestDBGroupContainsMigrationAndSeedCommands(t *testing.T) {
	cmd := dbcmd.Command(migratecmd.New(nil, nil), console.Value(&dbcmd.Deps{}))
	if cmd.Name() != "db" {
		t.Fatalf("group = %q, want db", cmd.Name())
	}
	want := []string{"migrate", "migrate:status", "migrate:baseline", "migrate:hash", "migrate:diff", "seed"}
	for _, name := range want {
		sub, _, err := cmd.Find([]string{name})
		if err != nil || sub == cmd || sub.Name() != name {
			t.Errorf("db %s is missing: %v", name, err)
		}
	}
	if len(cmd.Commands()) != len(want) {
		t.Fatalf("subcommands: %d", len(cmd.Commands()))
	}
}

// db seed builds the database graph when it runs and seeds through it.
func TestSeedCommandGetsDepsWhenItRuns(t *testing.T) {
	builds, seeds := 0, 0
	deps := console.NewGraph(func() (*dbcmd.Deps, func(), error) {
		builds++
		return dbcmd.NewDeps(nil, nil, dbcmd.Seeders{
			dbcmd.SeederFunc(func(context.Context) error { seeds++; return nil }),
		}), func() {}, nil
	})
	for _, args := range [][]string{{"--help"}, {"seed", "--help"}} {
		root := dbcmd.Command(nil, deps)
		root.SetArgs(args)
		root.SetOut(&strings.Builder{})
		if err := root.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	if builds != 0 {
		t.Fatalf("help built the database graph %d times", builds)
	}
	root := dbcmd.Command(nil, deps)
	root.SetArgs([]string{"seed"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if builds != 1 || seeds != 1 {
		t.Fatalf("builds = %d, seeds = %d; want 1, 1", builds, seeds)
	}
}

func TestSeedRunsSeedersInOrderAndStopsOnError(t *testing.T) {
	var calls []int
	failure := errors.New("boom")
	seeders := dbcmd.Seeders{
		dbcmd.SeederFunc(func(context.Context) error { calls = append(calls, 1); return nil }),
		nil,
		dbcmd.SeederFunc(func(context.Context) error { calls = append(calls, 2); return failure }),
		dbcmd.SeederFunc(func(context.Context) error { calls = append(calls, 3); return nil }),
	}
	err := dbcmd.NewDeps(nil, nil, seeders).Seed(context.Background())
	if !errors.Is(err, failure) {
		t.Fatalf("Seed() error = %v, want boom", err)
	}
	if len(calls) != 2 || calls[0] != 1 || calls[1] != 2 {
		t.Fatalf("calls = %v, want [1 2]", calls)
	}
}
