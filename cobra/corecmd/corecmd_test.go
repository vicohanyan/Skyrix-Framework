package corecmd_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"gitlab.com/skyrix-lib/framework/cobra/console"
	"gitlab.com/skyrix-lib/framework/cobra/corecmd"
	"gitlab.com/skyrix-lib/framework/database/dbcmd"
	"gitlab.com/skyrix-lib/framework/database/migrate/migratecmd"
	engineJobs "gitlab.com/skyrix-lib/framework/engine/jobs"
	"gitlab.com/skyrix-lib/framework/scaffold/makecmd"
)

func TestCommandsIncludeFrameworkCommands(t *testing.T) {
	commands := corecmd.Commands(corecmd.Options{
		Migrations: migratecmd.New(nil, nil),
		Database:   console.Value(&dbcmd.Deps{}),
		Jobs:       console.Value[engineJobs.Registry](nil),
	})
	want := []string{"db", "jobs:run", "make", "config:env", "platform"}
	if len(commands) != len(want) {
		t.Fatalf("commands = %d, want %d", len(commands), len(want))
	}
	for i, name := range want {
		if got := commands[i].Name(); got != name {
			t.Fatalf("command %d = %q, want %q", i, got, name)
		}
	}

	root := console.New(commands...).Root()
	for _, path := range [][]string{{"db", "migrate"}, {"db", "migrate:status"}, {"db", "migrate:baseline"}, {"db", "migrate:hash"}, {"db", "migrate:diff"}, {"db", "seed"}, {"jobs:run"}, {"config:env"}, {"make", "domain"}, {"make", "migration"}} {
		command, _, err := root.Find(path)
		if err != nil || command.Name() != path[len(path)-1] {
			t.Fatalf("%q is missing: %v", path, err)
		}
	}
}

// TestManualMigrationProcessWithoutAtlas runs the hand-written SQL process
// through the assembled console without the Atlas CLI in PATH, database
// settings or a database graph: make migration, a manual edit that breaks
// the checksum, the explicit db migrate:hash and the next make migration.
func TestManualMigrationProcessWithoutAtlas(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := exec.LookPath("atlas"); err == nil {
		t.Fatal("atlas still in PATH")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	migrations := migratecmd.New(func() (migratecmd.Settings, func(), error) {
		t.Fatal("settings loaded")
		return migratecmd.Settings{}, nil, nil
	}, fstest.MapFS{})
	migrations.ProjectRoot = root
	database := console.NewGraph(func() (*dbcmd.Deps, func(), error) {
		t.Fatal("database graph built")
		return nil, nil, nil
	})
	newApp := func() *console.Console {
		return console.New(corecmd.Commands(corecmd.Options{
			Migrations: migrations,
			Database:   database,
			Jobs:       console.Value[engineJobs.Registry](nil),
			Make:       &makecmd.MakeCommand{ProjectRoot: root, Now: func() time.Time { return now }},
		})...)
	}
	run := func(args ...string) (string, error) {
		var out strings.Builder
		err := newApp().Execute(context.Background(), args, &out, &out)
		return out.String(), err
	}
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join(root, "database/migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	for _, args := range [][]string{{"--help"}, {"db", "--help"}, {"db", "migrate:hash", "--help"}, {"db", "migrate:diff", "--help"}, {"make", "migration", "--help"}, {"__complete", "db", ""}, {"completion", "bash"}} {
		if output, err := run(args...); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, output)
		}
	}

	if output, err := run("make", "migration", "create_a"); err != nil {
		t.Fatalf("first make migration: %v\n%s", err, output)
	}
	first := "20261104100000_create_a.sql"
	sum := read("atlas.sum")
	if read(first) != "" || !strings.Contains(sum, first+" h1:") {
		t.Fatalf("first migration: %q, atlas.sum %q", read(first), sum)
	}

	// Writing SQL into the file breaks the checksum until db migrate:hash.
	sql := "CREATE TABLE a (id bigint PRIMARY KEY);\n"
	if err := os.WriteFile(filepath.Join(root, "database/migrations", first), []byte(sql), 0o644); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	if _, err := run("make", "migration", "create_b"); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("make migration after a manual edit error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "database/migrations", "20261104100001_create_b.sql")); !os.IsNotExist(err) {
		t.Fatalf("migration written despite the checksum mismatch: %v", err)
	}
	if read("atlas.sum") != sum {
		t.Fatal("atlas.sum changed without db migrate:hash")
	}
	if output, err := run("db", "migrate:hash"); err != nil || !strings.Contains(output, "updated database/migrations/atlas.sum") {
		t.Fatalf("db migrate:hash = %q, %v", output, err)
	}
	if read(first) != sql || read("atlas.sum") == sum {
		t.Fatal("db migrate:hash changed the SQL file or kept the old checksum")
	}
	if output, err := run("make", "migration", "create_b"); err != nil {
		t.Fatalf("make migration after db migrate:hash: %v\n%s", err, output)
	}
	if !strings.Contains(read("atlas.sum"), "20261104100001_create_b.sql h1:") {
		t.Fatal(read("atlas.sum"))
	}

	// Generation from the models is the only step that needs the CLI.
	if _, err := run("db", "migrate:diff", "create_c"); err == nil || !strings.Contains(err.Error(), "atlas CLI is not installed") {
		t.Fatalf("db migrate:diff without the CLI error = %v", err)
	}
}
