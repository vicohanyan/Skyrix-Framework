package job

import (
	"bytes"
	"context"
	"errors"
	"testing"

	engineJobs "skyrix/internal/engine/jobs"
)

func TestJobsRunCommandMissingNameFails(t *testing.T) {
	cmd := NewJobsRunCommand(&fakeJobsRegistry{}).ToCobraCommand()
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err == nil {
		t.Fatalf("Execute() error = nil, want error")
	}
}

func TestJobsRunCommandUnknownJobFails(t *testing.T) {
	reg := &fakeJobsRegistry{runErr: errors.New("job not found")}
	cmd := NewJobsRunCommand(reg).ToCobraCommand()
	cmd.SetArgs([]string{"--name=missing.job"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	if err := cmd.Execute(); err == nil {
		t.Fatalf("Execute() error = nil, want error")
	}
}

func TestJobsRunCommandPassesArgsToRegistry(t *testing.T) {
	reg := &fakeJobsRegistry{}
	cmd := NewJobsRunCommand(reg).ToCobraCommand()
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
