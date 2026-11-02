package cobra

import (
	"context"
	"errors"
	"strings"
	"testing"

	spfcobra "github.com/spf13/cobra"
)

func TestExecuteStandaloneRunsOnlyKnownRunnableCommands(t *testing.T) {
	ran := ""
	db := &spfcobra.Command{Use: "db"}
	db.AddCommand(&spfcobra.Command{Use: "migrate", RunE: func(*spfcobra.Command, []string) error {
		ran = "migrate"
		return nil
	}})
	broken := &spfcobra.Command{Use: "broken", RunE: func(*spfcobra.Command, []string) error {
		return errors.New("boom")
	}}
	commands := func() []*spfcobra.Command { return []*spfcobra.Command{db, broken} }

	handled, err := ExecuteStandalone(context.Background(), []string{"db", "migrate"}, &strings.Builder{}, &strings.Builder{}, commands()...)
	if err != nil || !handled || ran != "migrate" {
		t.Fatalf("db migrate: handled = %v, err = %v, ran = %q", handled, err, ran)
	}

	var help strings.Builder
	handled, err = ExecuteStandalone(context.Background(), []string{"db", "migrate", "--help"}, &help, &strings.Builder{}, commands()...)
	if err != nil || !handled || !strings.Contains(help.String(), "Usage:") {
		t.Fatalf("db migrate --help: handled = %v, err = %v, output = %q", handled, err, help.String())
	}

	handled, err = ExecuteStandalone(context.Background(), []string{"broken"}, &strings.Builder{}, &strings.Builder{}, commands()...)
	if !handled || err == nil {
		t.Fatalf("broken: handled = %v, err = %v; want handled error", handled, err)
	}

	for _, args := range [][]string{{}, {"db"}, {"db", "seed"}, {"jobs:run"}, {"--help"}} {
		handled, err := ExecuteStandalone(context.Background(), args, &strings.Builder{}, &strings.Builder{}, commands()...)
		if handled || err != nil {
			t.Fatalf("%v: handled = %v, err = %v; want not handled", args, handled, err)
		}
	}
}
