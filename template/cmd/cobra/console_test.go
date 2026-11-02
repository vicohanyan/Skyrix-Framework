package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"gitlab.com/skyrix-lib/framework/cobra/console"
	"gitlab.com/skyrix-lib/framework/database/dbcmd"
	"gitlab.com/skyrix-lib/template/internal/commands"

	"github.com/spf13/cobra"
)

// countingGraphs wraps the real Wire injectors and counts their builds.
type countingGraphs struct {
	graphs
	app, database int
}

func newCountingGraphs() *countingGraphs {
	counter := &countingGraphs{}
	counter.graphs = graphs{
		App: console.NewGraph(console.NeedsApp, func() (*appGraph, func(), error) {
			counter.app++
			return buildApp()
		}),
		Database: console.NewGraph(console.NeedsDatabase, func() (*dbcmd.Deps, func(), error) {
			counter.database++
			return buildDatabase()
		}),
	}
	return counter
}

func execute(t *testing.T, c *console.Console, args ...string) (string, error) {
	t.Helper()
	var stdout, stderr strings.Builder
	err := c.Execute(context.Background(), args, &stdout, &stderr)
	return stdout.String() + stderr.String(), err
}

// TestCommandNeeds checks what every console command needs when it runs.
func TestCommandNeeds(t *testing.T) {
	g := newCountingGraphs()
	c := newConsole(g.graphs)
	for _, test := range []struct {
		path  string
		needs console.Needs
	}{
		{"db migrate", console.NeedsNothing},
		{"db migrate:status", console.NeedsNothing},
		{"db migrate:baseline", console.NeedsNothing},
		{"db migrate:diff", console.NeedsNothing},
		{"make domain", console.NeedsNothing},
		{"config:env", console.NeedsNothing},
		{"make migration", console.NeedsNothing},
		{"db automigrate", console.NeedsDatabase},
		{"db seed", console.NeedsDatabase},
		{"jobs:run", console.NeedsApp},
		{"hello", console.NeedsApp},
	} {
		command := c.Find(strings.Fields(test.path)...)
		if command == nil {
			t.Fatalf("%s is missing", test.path)
		}
		if command.Needs() != test.needs {
			t.Errorf("%s needs %v, want %v", test.path, command.Needs(), test.needs)
		}
	}
	if g.app != 0 || g.database != 0 {
		t.Fatalf("describing the commands built graphs: app %d, database %d", g.app, g.database)
	}
}

// TestNewCommandDefinedInOnePlace adds a command in one place only (its
// description, registered once): it is listed by --help without building any
// graph, even with a configuration that cannot be loaded, and it runs with
// its dependencies from the application graph.
func TestNewCommandDefinedInOnePlace(t *testing.T) {
	setConsoleEnv(t, "127.0.0.1:1", "127.0.0.1:1")
	t.Setenv("SERVICE_NAME", "orders")
	chdirToProject(t)

	newApp := func(g *countingGraphs) *console.Console {
		deps := console.Derive(g.App, func(a *appGraph) *commands.Deps { return a.Commands })
		return newConsole(g.graphs).Add(&console.Command{
			Use:   "whoami",
			Short: "Print the service name",
			Args:  cobra.NoArgs,
			Run: console.With(deps, func(cmd *cobra.Command, _ []string, deps *commands.Deps) error {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), deps.Config.ServiceName)
				return err
			}),
		})
	}

	t.Run("help", func(t *testing.T) {
		t.Setenv("DB_PORT", "not-a-port")
		g := newCountingGraphs()
		for _, args := range [][]string{nil, {"--help"}, {"help", "whoami"}, {"whoami", "--help"}} {
			output, err := execute(t, newApp(g), args...)
			if err != nil {
				t.Fatalf("%q: %v", args, err)
			}
			if !strings.Contains(output, "whoami") {
				t.Fatalf("%q does not list whoami:\n%s", args, output)
			}
		}
		if g.app != 0 || g.database != 0 {
			t.Fatalf("help built graphs: app %d, database %d", g.app, g.database)
		}
	})

	t.Run("run", func(t *testing.T) {
		g := newCountingGraphs()
		defer g.Close()
		output, err := execute(t, newApp(g), "whoami")
		if err != nil {
			t.Fatal(err)
		}
		if output != "orders\n" {
			t.Fatalf("output = %q, want orders", output)
		}
		if g.app != 1 || g.database != 0 {
			t.Fatalf("whoami built graphs: app %d, database %d; want 1, 0", g.app, g.database)
		}
	})
}

// TestApplicationCommandRunsWithDependencies runs hello: it gets the service
// name from the application graph and builds no database graph.
func TestApplicationCommandRunsWithDependencies(t *testing.T) {
	database, redis := countingListener(t), countingListener(t)
	setConsoleEnv(t, database.addr, redis.addr)
	t.Setenv("SERVICE_NAME", "orders")

	g := newCountingGraphs()
	defer g.Close()
	output, err := execute(t, newConsole(g.graphs), "hello", "--name", "Ann")
	if err != nil {
		t.Fatal(err)
	}
	if output != "Hello, Ann! from orders\n" {
		t.Fatalf("output = %q", output)
	}
	if g.app != 1 || g.database != 0 {
		t.Fatalf("hello built graphs: app %d, database %d; want 1, 0", g.app, g.database)
	}
	if n := database.count.Load() + redis.count.Load(); n != 0 {
		t.Fatalf("hello opened %d connections, want 0", n)
	}
}

// TestDatabaseCommandBuildsOnlyDatabaseGraph runs db seed against a fake
// PostgreSQL server: it builds the database graph only, and connecting fails.
func TestDatabaseCommandBuildsOnlyDatabaseGraph(t *testing.T) {
	database, redis := countingListener(t), countingListener(t)
	setConsoleEnv(t, database.addr, redis.addr)

	g := newCountingGraphs()
	defer g.Close()
	if _, err := execute(t, newConsole(g.graphs), "db", "automigrate"); err == nil {
		t.Fatal("db automigrate against a fake server error = nil")
	}
	if g.app != 0 || g.database != 1 {
		t.Fatalf("db automigrate built graphs: app %d, database %d; want 0, 1", g.app, g.database)
	}
	if database.count.Load() == 0 {
		t.Fatal("db automigrate did not connect to the database")
	}
	if n := redis.count.Load(); n != 0 {
		t.Fatalf("redis connections = %d, want 0", n)
	}
}

// TestMigrationHelpNeedsNoDatabase shows migration help with an unreachable
// database and no configuration file.
func TestMigrationHelpNeedsNoDatabase(t *testing.T) {
	t.Setenv("APP_ENV", "missing-environment")
	t.Setenv("DB_HOST", "203.0.113.1")

	g := newCountingGraphs()
	for _, args := range [][]string{
		{"db", "migrate", "--help"},
		{"db", "migrate:status", "--help"},
		{"db", "migrate:baseline", "--help"},
		{"db", "migrate:diff", "--help"},
		{"make", "migration", "--help"},
		{"db"},
		{"make"},
	} {
		output, err := execute(t, newConsole(g.graphs), args...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !strings.Contains(output, "Usage:") {
			t.Fatalf("%v help output:\n%s", args, output)
		}
	}
	if g.app != 0 || g.database != 0 {
		t.Fatalf("help built graphs: app %d, database %d", g.app, g.database)
	}
}
