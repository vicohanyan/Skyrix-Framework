package configcmd_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"gitlab.com/skyrix-lib/framework/cobra/configcmd"
	"gitlab.com/skyrix-lib/framework/cobra/console"
	"gitlab.com/skyrix-lib/framework/config"
)

type defaults struct {
	Z        string
	Timeout  time.Duration
	Enabled  bool
	Ports    []int
	Password string                 `secret:"true"`
	Hidden   struct{ Token string } `secret:"true"`
}

func (*defaults) Validate() error { panic("metadata must not validate") }

func run(t *testing.T, command *console.Command, args ...string) (string, error) {
	t.Helper()
	var out, stderr strings.Builder
	err := console.New(command).Execute(context.Background(), append([]string{"config:env"}, args...), &out, &stderr)
	return out.String(), err
}

func TestOutputAndSnapshot(t *testing.T) {
	d := defaults{Z: "hello\nworld", Timeout: 2 * time.Second, Enabled: true, Ports: []int{2, 3}, Password: "password-default"}
	d.Hidden.Token = "nested-default"
	legacy := struct {
		Port   int    `env:"LEGACY_PORT" env-default:"42"`
		Secret string `env:"LEGACY_SECRET" env-default:"tag-secret" secret:"true"`
	}{Port: 99}
	t.Setenv("Z_Z", "live-env-value")
	command := configcmd.Env(config.Section{Key: "z", Value: &d}, config.Section{Key: "a", Value: &legacy, LegacyEnv: true})
	d.Z = "mutated-live-value"
	d.Ports[0] = 99
	if command.Needs() != console.NeedsNothing {
		t.Fatal("command needs a graph")
	}
	output, err := run(t, command)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	want := []string{
		"SECTION ENV TYPE DEFAULT",
		"a LEGACY_PORT int 42",
		"a LEGACY_SECRET string [REDACTED]",
		"z Z_ENABLED bool true",
		"z Z_HIDDEN_TOKEN string [REDACTED]",
		"z Z_PASSWORD string [REDACTED]",
		"z Z_PORTS []int [2,3]",
		"z Z_TIMEOUT time.Duration 2s",
		`z Z_Z string "hello\nworld"`,
	}
	if len(lines) != len(want) {
		t.Fatalf("unexpected output:\n%s", output)
	}
	for i := range want {
		if got := strings.Join(strings.Fields(lines[i]), " "); got != want[i] {
			t.Errorf("row %d: %q, want %q", i, got, want[i])
		}
	}
	for _, secret := range []string{"password-default", "nested-default", "tag-secret", "live-env-value", "mutated-live-value"} {
		if strings.Contains(output, secret) {
			t.Fatalf("value leaked: %s", secret)
		}
	}
	again, err := run(t, command)
	if err != nil || again != output {
		t.Fatalf("unstable repeated execution: %v\n%s", err, again)
	}
}

func TestSectionFilter(t *testing.T) {
	var a, b struct{ Name string }
	command := configcmd.Env(config.Section{Key: "a", Value: &a}, config.Section{Key: "b", Value: &b})
	output, err := run(t, command, "--section", "b")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 || strings.Join(strings.Fields(lines[1]), " ") != `b B_NAME string ""` {
		t.Fatal(output)
	}
	for _, key := range []string{"missing", "B", ""} {
		output, err := run(t, command, "--section", key)
		if err == nil || !strings.Contains(err.Error(), "unknown section") || output != "" {
			t.Fatalf("%q: %q, %v", key, output, err)
		}
	}
}

func TestInvalidDescription(t *testing.T) {
	command := configcmd.Env(config.Section{Key: "invalid", Value: nil})
	if _, err := run(t, command); err == nil {
		t.Fatal("invalid description accepted")
	}
}
