package jobscmd

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"gitlab.com/skyrix-lib/framework/cobra/console"
	engineJobs "gitlab.com/skyrix-lib/framework/engine/jobs"

	"github.com/spf13/cobra"
)

func TestJobsRunCommandMissingNameFails(t *testing.T) {
	cmd := newCommand(&fakeJobsRegistry{})
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err == nil {
		t.Fatalf("Execute() error = nil, want error")
	}
}

func TestJobsRunCommandUnknownJobFails(t *testing.T) {
	reg := &fakeJobsRegistry{runErr: errors.New("job not found")}
	cmd := newCommand(reg)
	cmd.SetArgs([]string{"--name=missing.job"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err == nil {
		t.Fatalf("Execute() error = nil, want error")
	}
}

func TestJobsRunCommandPassesArgsToRegistry(t *testing.T) {
	reg := &fakeJobsRegistry{}
	cmd := newCommand(reg)
	cmd.SetArgs([]string{
		"--name=payment.reconcile",
		`--args-json={"limit":100,"max_attempts":2,"source":"test"}`,
		"--limit=50",
		"--max-attempts=3",
	})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if reg.name != "payment.reconcile" {
		t.Fatalf("registry name = %q", reg.name)
	}
	if reg.args["limit"] != 50 {
		t.Fatalf("limit arg = %#v, want 50", reg.args["limit"])
	}
	if reg.args["max_attempts"] != 3 {
		t.Fatalf("max_attempts arg = %#v, want 3", reg.args["max_attempts"])
	}
	if reg.args["source"] != "test" {
		t.Fatalf("source arg = %#v, want test", reg.args["source"])
	}
}

func newCommand(registry engineJobs.Registry) *cobra.Command {
	return Command(console.Value(console.NeedsApp, registry)).Cobra()
}

// The registry is built only when jobs:run runs, not for its help.
func TestJobsRunCommandBuildsRegistryOnlyWhenItRuns(t *testing.T) {
	builds := 0
	registry := console.NewGraph(console.NeedsApp, func() (engineJobs.Registry, func(), error) {
		builds++
		return &fakeJobsRegistry{}, func() {}, nil
	})
	command := Command(registry)
	if command.Needs() != console.NeedsApp {
		t.Fatalf("jobs:run needs %v, want application", command.Needs())
	}
	cmd := command.Cobra()
	cmd.SetArgs([]string{"--help"})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil || builds != 0 {
		t.Fatalf("--help: err = %v, builds = %d", err, builds)
	}
	cmd = command.Cobra()
	cmd.SetArgs([]string{"--name=x"})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil || builds != 1 {
		t.Fatalf("run: err = %v, builds = %d", err, builds)
	}
}

type fakeJobsRegistry struct {
	name   string
	args   map[string]any
	runErr error
}

func (r *fakeJobsRegistry) Register(job engineJobs.Job)            {}
func (r *fakeJobsRegistry) Get(name string) (engineJobs.Job, bool) { return nil, false }
func (r *fakeJobsRegistry) List() []string                         { return nil }
func (r *fakeJobsRegistry) Run(ctx context.Context, name string, args map[string]any) error {
	r.name = name
	r.args = args
	return r.runErr
}
