package cobra

import (
	"context"
	"errors"
	"io"
	"testing"

	spfcobra "github.com/spf13/cobra"
	"gitlab.com/skyrix-lib/framework/cobra/console"
)

func TestDriverCommandsAreLazy(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"driver", "--help"}, {"completion", "bash"}, {"__complete", "driver", ""}, {"driver"}} {
		built, cleaned, ran := 0, 0, 0
		graph := console.NewGraph(func() (int, func(), error) { built++; return 7, func() { cleaned++ }, nil })
		module := commandModule{factories: []CommandFactory{func() *spfcobra.Command {
			return &spfcobra.Command{Use: "driver", RunE: console.With(graph, func(_ *spfcobra.Command, _ []string, v int) error {
				if v != 7 {
					t.Fatal(v)
				}
				ran++
				return nil
			})}
		}}}
		app := console.New(ModuleCommands(module)...)
		if built != 0 {
			t.Fatal("registration built graph")
		}
		if err := app.Execute(context.Background(), args, io.Discard, io.Discard); err != nil {
			t.Fatal(err)
		}
		graph.Close()
		graph.Close()
		want := 0
		if len(args) == 1 && args[0] == "driver" {
			want = 1
		}
		if built != want || cleaned != want || ran != want {
			t.Fatalf("%v: build=%d cleanup=%d run=%d", args, built, cleaned, ran)
		}
	}
}
func TestDriverBuildFailureCleansUp(t *testing.T) {
	cleaned, ran := 0, false
	failed := errors.New("constructor failed")
	graph := console.NewGraph(func() (int, func(), error) { return 0, func() { cleaned++ }, failed })
	app := console.New(&spfcobra.Command{Use: "driver", RunE: console.With(graph, func(_ *spfcobra.Command, _ []string, _ int) error { ran = true; return nil })})
	err := app.Execute(context.Background(), []string{"driver"}, io.Discard, io.Discard)
	graph.Close()
	if !errors.Is(err, failed) || cleaned != 1 || ran {
		t.Fatalf("%v cleanup=%d ran=%v", err, cleaned, ran)
	}
}
