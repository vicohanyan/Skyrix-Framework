package console_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"gitlab.com/skyrix-lib/framework/cobra/console"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type greeter struct{ greeting string }

// countingGraph is an application graph whose builds and cleanups are counted.
type countingGraph struct {
	builds, cleanups int
	err              error
	graph            *console.Graph[*greeter]
}

func newCountingGraph(needs console.Needs, err error) *countingGraph {
	counter := &countingGraph{err: err}
	counter.graph = console.NewGraph(needs, func() (*greeter, func(), error) {
		counter.builds++
		if counter.err != nil {
			return nil, func() { counter.cleanups++ }, counter.err
		}
		return &greeter{greeting: "Hello"}, func() { counter.cleanups++ }, nil
	})
	return counter
}

// greetCommand is a command described in one place only: metadata, flags and
// a handler that gets its dependencies when it runs.
func greetCommand(deps *console.Graph[*greeter]) *console.Command {
	return &console.Command{
		Use:   "greet [name]",
		Short: "Greet somebody",
		Args:  cobra.MaximumNArgs(1),
		Flags: func(flags *pflag.FlagSet) {
			flags.BoolP("shout", "s", false, "Shout the greeting")
		},
		Run: console.With(deps, func(cmd *cobra.Command, args []string, g *greeter) error {
			name := "World"
			if len(args) == 1 {
				name = args[0]
			}
			message := fmt.Sprintf("%s, %s!", g.greeting, name)
			if shout, _ := cmd.Flags().GetBool("shout"); shout {
				message = strings.ToUpper(message)
			}
			fmt.Fprintln(cmd.OutOrStdout(), message)
			return nil
		}),
	}
}

func execute(t *testing.T, c *console.Console, args ...string) (string, error) {
	t.Helper()
	var stdout, stderr strings.Builder
	err := c.Execute(context.Background(), args, &stdout, &stderr)
	return stdout.String() + stderr.String(), err
}

// A command added in one place appears in help and completion without
// building its graph, and runs with its dependencies.
func TestCommandDefinedOnceHelpWithoutGraphRunWithDependencies(t *testing.T) {
	app := newCountingGraph(console.NeedsApp, nil)
	c := console.New(greetCommand(app.graph))

	for _, args := range [][]string{
		nil,
		{"--help"},
		{"help"},
		{"help", "greet"},
		{"greet", "--help"},
		{"greet", "-h"},
		{cobra.ShellCompRequestCmd, "gr"},
		{cobra.ShellCompRequestCmd, "greet", "--"},
	} {
		output, err := execute(t, c, args...)
		if err != nil {
			t.Fatalf("%q: %v", args, err)
		}
		if !strings.Contains(output, "greet") && !strings.Contains(output, "--shout") {
			t.Fatalf("%q does not mention greet:\n%s", args, output)
		}
	}
	if _, err := execute(t, c, "completion", "bash"); err != nil {
		t.Fatalf("completion bash: %v", err)
	}
	if output, _ := execute(t, c, "greet", "--help"); !strings.Contains(output, "--shout") {
		t.Fatalf("greet --help does not show the flag:\n%s", output)
	}
	if app.builds != 0 {
		t.Fatalf("help built the graph %d times", app.builds)
	}

	output, err := execute(t, c, "greet", "--shout", "Ann")
	if err != nil {
		t.Fatal(err)
	}
	if output != "HELLO, ANN!\n" {
		t.Fatalf("output = %q", output)
	}
	if app.builds != 1 {
		t.Fatalf("builds = %d, want 1", app.builds)
	}
	app.graph.Close()
	app.graph.Close()
	if app.cleanups != 1 {
		t.Fatalf("cleanups = %d, want 1", app.cleanups)
	}
}

// Running a command builds only the graph it needs.
func TestCommandBuildsOnlyItsGraph(t *testing.T) {
	app := newCountingGraph(console.NeedsApp, nil)
	database := newCountingGraph(console.NeedsDatabase, nil)
	ran := ""
	group := &console.Command{
		Use:   "db",
		Short: "Database commands",
		Commands: []*console.Command{
			{Use: "migrate", Run: console.Standalone(func(*cobra.Command, []string) error { ran = "migrate"; return nil })},
			{Use: "seed", Run: console.With(database.graph, func(*cobra.Command, []string, *greeter) error { ran = "seed"; return nil })},
		},
	}
	c := console.New(group, greetCommand(app.graph))

	if group.Needs() != console.NeedsNothing || group.Find("migrate").Needs() != console.NeedsNothing ||
		group.Find("seed").Needs() != console.NeedsDatabase || c.Find("greet").Needs() != console.NeedsApp {
		t.Fatal("Needs does not follow the runners")
	}

	if _, err := execute(t, c, "db", "migrate"); err != nil || ran != "migrate" {
		t.Fatalf("db migrate: err = %v, ran = %q", err, ran)
	}
	if app.builds != 0 || database.builds != 0 {
		t.Fatalf("standalone built graphs: app %d, database %d", app.builds, database.builds)
	}
	if _, err := execute(t, c, "db", "seed"); err != nil || ran != "seed" {
		t.Fatalf("db seed: err = %v, ran = %q", err, ran)
	}
	if app.builds != 0 || database.builds != 1 {
		t.Fatalf("db seed built graphs: app %d, database %d", app.builds, database.builds)
	}
	if output, err := execute(t, c, "db"); err != nil || !strings.Contains(output, "seed") {
		t.Fatalf("db group: err = %v, output:\n%s", err, output)
	}
	if _, err := execute(t, c, "unknown"); err == nil {
		t.Fatal("unknown command error = nil")
	}
	if app.builds != 0 || database.builds != 1 {
		t.Fatalf("groups and unknown commands built graphs: app %d, database %d", app.builds, database.builds)
	}
}

// A failed build is reported without running the handler and is not retried.
func TestBuildErrorStopsCommand(t *testing.T) {
	failure := errors.New("no config")
	app := newCountingGraph(console.NeedsApp, failure)
	ran := false
	c := console.New(&console.Command{
		Use: "work",
		Run: console.With(app.graph, func(*cobra.Command, []string, *greeter) error { ran = true; return nil }),
	})
	for range 2 {
		output, err := execute(t, c, "work")
		if !errors.Is(err, failure) || !strings.Contains(err.Error(), "build application dependencies") {
			t.Fatalf("error = %v", err)
		}
		if strings.Contains(output, "Usage:") {
			t.Fatalf("runtime error printed the usage:\n%s", output)
		}
	}
	if ran {
		t.Fatal("handler ran after a failed build")
	}
	if app.builds != 1 || app.cleanups != 1 {
		t.Fatalf("builds = %d, cleanups = %d; want 1, 1", app.builds, app.cleanups)
	}
}

func TestDeriveSharesParentBuild(t *testing.T) {
	app := newCountingGraph(console.NeedsApp, nil)
	greeting := console.Derive(app.graph, func(g *greeter) string { return g.greeting })
	if greeting.Needs() != console.NeedsApp || greeting.Built() {
		t.Fatalf("derived graph: needs %v, built %v", greeting.Needs(), greeting.Built())
	}
	for range 2 {
		if value, err := greeting.Get(); err != nil || value != "Hello" {
			t.Fatalf("Get() = %q, %v", value, err)
		}
	}
	if _, err := app.graph.Get(); err != nil || app.builds != 1 {
		t.Fatalf("builds = %d, err = %v; want 1", app.builds, err)
	}
	greeting.Close()
	if app.cleanups != 0 {
		t.Fatal("closing the derived graph closed the parent")
	}
	app.graph.Close()
	if app.cleanups != 1 {
		t.Fatalf("cleanups = %d, want 1", app.cleanups)
	}
}

func TestUnconfiguredGraph(t *testing.T) {
	var graph *console.Graph[*greeter]
	c := console.New(&console.Command{
		Use: "work",
		Run: console.With(graph, func(*cobra.Command, []string, *greeter) error { return nil }),
	})
	if _, err := execute(t, c, "work"); err == nil {
		t.Fatal("command with a nil graph error = nil")
	}
	if value, err := console.Value(console.NeedsApp, 7).Get(); err != nil || value != 7 {
		t.Fatalf("Value().Get() = %v, %v", value, err)
	}
}

func TestAddCobraRegistersStandaloneCommands(t *testing.T) {
	ran := false
	c := console.New().AddCobra(&cobra.Command{Use: "events", RunE: func(*cobra.Command, []string) error { ran = true; return nil }}, nil)
	if _, err := execute(t, c, "events"); err != nil || !ran {
		t.Fatalf("events: err = %v, ran = %v", err, ran)
	}
	if c.Find("events") != nil {
		t.Fatal("Find returned a Cobra command")
	}
}
