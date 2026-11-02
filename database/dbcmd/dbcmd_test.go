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

func TestDBGroupContainsMigrationAndDevelopmentCommands(t *testing.T) {
	cmd := dbcmd.Command(migratecmd.New(nil, nil), console.Value(console.NeedsDatabase, &dbcmd.Deps{}))
	if cmd.Name() != "db" {
		t.Fatalf("group = %q, want db", cmd.Name())
	}
	want := map[string]console.Needs{
		"migrate":          console.NeedsNothing,
		"migrate:status":   console.NeedsNothing,
		"migrate:baseline": console.NeedsNothing,
		"migrate:diff":     console.NeedsNothing,
		"automigrate":      console.NeedsDatabase,
		"seed":             console.NeedsDatabase,
	}
	for name, needs := range want {
		sub := cmd.Find(name)
		if sub == nil {
			t.Errorf("db %s is missing", name)
			continue
		}
		if sub.Needs() != needs {
			t.Errorf("db %s needs %v, want %v", name, sub.Needs(), needs)
		}
	}
	if len(cmd.Commands) != len(want) {
		t.Errorf("db has %d subcommands, want %d", len(cmd.Commands), len(want))
	}
}

// db seed builds the database graph when it runs and seeds through it.
func TestSeedCommandGetsDepsWhenItRuns(t *testing.T) {
	builds, seeds := 0, 0
	deps := console.NewGraph(console.NeedsDatabase, func() (*dbcmd.Deps, func(), error) {
		builds++
		return dbcmd.NewDeps(nil, nil, nil, dbcmd.Seeders{
			dbcmd.SeederFunc(func(context.Context) error { seeds++; return nil }),
		}), func() {}, nil
	})
	group := dbcmd.Command(nil, deps)
	for _, args := range [][]string{{"--help"}, {"seed", "--help"}} {
		root := group.Cobra()
		root.SetArgs(args)
		root.SetOut(&strings.Builder{})
		if err := root.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	if builds != 0 {
		t.Fatalf("help built the database graph %d times", builds)
	}
	root := group.Cobra()
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
	err := dbcmd.NewDeps(nil, nil, nil, seeders).Seed(context.Background())
	if !errors.Is(err, failure) {
		t.Fatalf("Seed() error = %v, want boom", err)
	}
	if len(calls) != 2 || calls[0] != 1 || calls[1] != 2 {
		t.Fatalf("calls = %v, want [1 2]", calls)
	}
}

func TestAutoMigrateWithoutDatabase(t *testing.T) {
	if err := dbcmd.NewDeps(nil, nil, dbcmd.Models{struct{}{}}, nil).AutoMigrate(context.Background()); err == nil {
		t.Fatal("AutoMigrate() without database error = nil")
	}
}
