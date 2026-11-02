package dbcmd_test

import (
	"context"
	"errors"
	"testing"

	"gitlab.com/skyrix-lib/framework/database/dbcmd"
	"gitlab.com/skyrix-lib/framework/database/migrate/migratecmd"
)

func TestDBGroupContainsMigrationAndDevelopmentCommands(t *testing.T) {
	cmd := dbcmd.New(nil, nil, migratecmd.New(nil, nil), nil, nil).ToCobraCommand()
	if cmd.Name() != "db" {
		t.Fatalf("group = %q, want db", cmd.Name())
	}
	got := map[string]bool{}
	for _, sub := range cmd.Commands() {
		got[sub.Name()] = true
	}
	for _, name := range []string{"migrate", "migrate:status", "migrate:baseline", "migrate:diff", "automigrate", "seed"} {
		if !got[name] {
			t.Errorf("db %s is missing; have %v", name, got)
		}
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
	err := dbcmd.New(nil, nil, nil, nil, seeders).Seed(context.Background())
	if !errors.Is(err, failure) {
		t.Fatalf("Seed() error = %v, want boom", err)
	}
	if len(calls) != 2 || calls[0] != 1 || calls[1] != 2 {
		t.Fatalf("calls = %v, want [1 2]", calls)
	}
}

func TestAutoMigrateWithoutDatabase(t *testing.T) {
	if err := dbcmd.New(nil, nil, nil, dbcmd.Models{struct{}{}}, nil).AutoMigrate(context.Background()); err == nil {
		t.Fatal("AutoMigrate() without database error = nil")
	}
}
