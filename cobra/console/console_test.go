package console_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gitlab.com/skyrix-lib/framework/cobra/console"
)

func execute(t *testing.T, c *console.Console, args ...string) (string, error) {
	t.Helper()
	var out strings.Builder
	err := c.Execute(context.Background(), args, &out, &out)
	return out.String(), err
}

func TestHelpCompletionAndInvalidInputDoNotBuildGraph(t *testing.T) {
	builds, cleanups := 0, 0
	graph := console.NewGraph(func() (string, func(), error) { builds++; return "Hello", func() { cleanups++ }, nil })
	command := func() *cobra.Command {
		cmd := &cobra.Command{Use: "greet [name]", Args: cobra.MaximumNArgs(1), RunE: console.With(graph, func(cmd *cobra.Command, args []string, greeting string) error {
			shout, _ := cmd.Flags().GetBool("shout")
			if shout {
				greeting = strings.ToUpper(greeting)
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), greeting)
			return err
		})}
		cmd.Flags().Bool("shout", false, "Shout the greeting")
		return cmd
	}
	for _, args := range [][]string{nil, {"--help"}, {"help", "greet"}, {"greet", "--help"}, {"__complete", "gr"}, {"completion", "bash"}} {
		if _, err := execute(t, console.New(command()), args...); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	for _, args := range [][]string{{"greet", "--unknown"}, {"greet", "one", "two"}} {
		if _, err := execute(t, console.New(command()), args...); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	if builds != 0 {
		t.Fatalf("help/invalid input built graph %d times", builds)
	}
	for range 2 {
		out, err := execute(t, console.New(command()), "greet", "--shout")
		if err != nil || out != "HELLO\n" {
			t.Fatalf("run: %q, %v", out, err)
		}
	}
	graph.Close()
	graph.Close()
	if builds != 1 || cleanups != 1 {
		t.Fatalf("builds=%d cleanups=%d", builds, cleanups)
	}
}

func TestCommandsBuildOnlySelectedGraph(t *testing.T) {
	appBuilds, dbBuilds := 0, 0
	app := console.NewGraph(func() (int, func(), error) { appBuilds++; return 1, nil, nil })
	db := console.NewGraph(func() (int, func(), error) { dbBuilds++; return 1, nil, nil })
	for _, name := range []string{"standalone", "seed", "app"} {
		c := console.New(
			&cobra.Command{Use: "standalone", RunE: console.Standalone(func(*cobra.Command, []string) error { return nil })},
			&cobra.Command{Use: "seed", RunE: console.With(db, func(*cobra.Command, []string, int) error { return nil })},
			&cobra.Command{Use: "app", RunE: console.With(app, func(*cobra.Command, []string, int) error { return nil })},
		)
		if _, err := execute(t, c, name); err != nil {
			t.Fatal(err)
		}
		if name == "standalone" && (appBuilds != 0 || dbBuilds != 0) {
			t.Fatal("standalone built dependencies")
		}
		if name == "seed" && (appBuilds != 0 || dbBuilds != 1) {
			t.Fatal("seed built wrong dependencies")
		}
	}
	if appBuilds != 1 || dbBuilds != 1 {
		t.Fatal("wrong build counts")
	}
}

func TestFailedBuildIsCachedAndCleanedUp(t *testing.T) {
	failure := errors.New("build failed")
	builds, cleanups := 0, 0
	graph := console.NewGraph(func() (int, func(), error) { builds++; return 7, func() { cleanups++ }, failure })
	for range 2 {
		cmd := &cobra.Command{Use: "work", RunE: console.With(graph, func(*cobra.Command, []string, int) error { t.Fatal("handler ran"); return nil })}
		out, err := execute(t, console.New(cmd), "work")
		if !errors.Is(err, failure) || strings.Contains(out, "Usage:") {
			t.Fatalf("%q, %v", out, err)
		}
	}
	graph.Close()
	if builds != 1 || cleanups != 1 {
		t.Fatalf("builds=%d cleanups=%d", builds, cleanups)
	}
}

func TestDeriveSharesBuildAndParentOwnsCleanup(t *testing.T) {
	builds, cleanups := 0, 0
	parent := console.NewGraph(func() (int, func(), error) { builds++; return 7, func() { cleanups++ }, nil })
	child := console.Derive(parent, func(n int) string { return fmt.Sprint(n) })
	if child.Built() {
		t.Fatal("eager build")
	}
	for range 2 {
		v, err := child.Get()
		if err != nil || v != "7" {
			t.Fatalf("%q, %v", v, err)
		}
	}
	child.Close()
	if cleanups != 0 {
		t.Fatal("child closed parent")
	}
	parent.Close()
	if builds != 1 || cleanups != 1 {
		t.Fatal("wrong lifecycle")
	}
}

func TestUnconfiguredAndValueGraphs(t *testing.T) {
	var graph *console.Graph[int]
	cmd := &cobra.Command{Use: "work", RunE: console.With(graph, func(*cobra.Command, []string, int) error { t.Fatal("handler ran"); return nil })}
	if _, err := execute(t, console.New(cmd), "work"); err == nil {
		t.Fatal("missing graph accepted")
	}
	if value, err := console.Value(7).Get(); err != nil || value != 7 {
		t.Fatalf("%d, %v", value, err)
	}
}
