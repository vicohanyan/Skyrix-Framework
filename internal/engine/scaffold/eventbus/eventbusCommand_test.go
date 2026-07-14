package eventbus

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestEventBusCommandLifecycle(t *testing.T) {
	root := newScaffoldProject(t)
	command := &EventBusCommand{ProjectRoot: root}

	output, err := executeScaffoldCommand(command.ToCobraCommand(), "init")
	if err != nil {
		t.Fatalf("eventbus init error = %v", err)
	}
	if !strings.Contains(output, "eventbus initialized") {
		t.Fatalf("eventbus init output = %q", output)
	}
	assertGoFileParses(t, filepath.Join(eventBusTransportRoot(root), "provider.go"))

	if _, err := executeScaffoldCommand(command.ToCobraCommand(), "init"); err == nil {
		t.Fatal("second eventbus init error = nil, want error")
	}

	for _, name := range []string{"Payment", "Delivery"} {
		if _, err := executeScaffoldCommand(command.ToCobraCommand(), "create", name); err != nil {
			t.Fatalf("eventbus create %s error = %v", name, err)
		}
	}
	assertGoFileParses(t, filepath.Join(eventBusTransportRoot(root), "payment", "provider.go"))
	assertGoFileParses(t, filepath.Join(eventBusTransportRoot(root), "delivery", "provider.go"))

	output, err = executeScaffoldCommand(command.ToCobraCommand(), "list")
	if err != nil {
		t.Fatalf("eventbus list error = %v", err)
	}
	if output != "delivery\npayment\n" {
		t.Fatalf("eventbus list output = %q", output)
	}

	registrationPath := filepath.Join(eventBusTransportRoot(root), "provider.go")
	registration := readTestFile(t, registrationPath)
	for _, expected := range []string{"delivery.ProviderSet", "payment.ProviderSet", "example.com/scaffold/internal/transport/eventbus/v1/payment"} {
		if !strings.Contains(registration, expected) {
			t.Fatalf("registration does not contain %q:\n%s", expected, registration)
		}
	}

	if _, err := executeScaffoldCommand(command.ToCobraCommand(), "create", "Payment"); err == nil {
		t.Fatal("duplicate eventbus create error = nil, want error")
	}
	if _, err := executeScaffoldCommand(command.ToCobraCommand(), "remove", "Payment"); err != nil {
		t.Fatalf("eventbus remove error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(eventBusTransportRoot(root), "payment")); !os.IsNotExist(err) {
		t.Fatalf("removed payment module stat error = %v, want not exist", err)
	}
	registration = readTestFile(t, registrationPath)
	if strings.Contains(registration, "payment.ProviderSet") {
		t.Fatalf("registration still contains removed payment module:\n%s", registration)
	}
	if !strings.Contains(registration, "delivery.ProviderSet") {
		t.Fatalf("registration lost delivery module:\n%s", registration)
	}
	assertGoFileParses(t, registrationPath)
}

func TestEventBusCommandValidatesArgumentsAndManagedModules(t *testing.T) {
	root := newScaffoldProject(t)
	command := &EventBusCommand{ProjectRoot: root}

	if _, err := executeScaffoldCommand(command.ToCobraCommand(), "create", "Payment"); err == nil {
		t.Fatal("create before init error = nil, want error")
	}
	if _, err := executeScaffoldCommand(command.ToCobraCommand(), "init", "extra"); err == nil {
		t.Fatal("init with argument error = nil, want error")
	}
	if _, err := executeScaffoldCommand(command.ToCobraCommand(), "init"); err != nil {
		t.Fatalf("eventbus init error = %v", err)
	}
	for _, invalid := range []string{"", "../Payment", "123Payment", "pay ment", "package"} {
		if _, err := executeScaffoldCommand(command.ToCobraCommand(), "create", invalid); err == nil {
			t.Fatalf("create %q error = nil, want error", invalid)
		}
	}

	unmanaged := filepath.Join(eventBusTransportRoot(root), "manual")
	if err := os.MkdirAll(unmanaged, 0o755); err != nil {
		t.Fatalf("mkdir unmanaged module: %v", err)
	}
	if _, err := executeScaffoldCommand(command.ToCobraCommand(), "remove", "Manual"); err == nil {
		t.Fatal("remove unmanaged module error = nil, want error")
	}
	if _, err := os.Stat(unmanaged); err != nil {
		t.Fatalf("unmanaged module was modified: %v", err)
	}

	if aliases := command.ToCobraCommand().Aliases; len(aliases) != 1 || aliases[0] != "bus" {
		t.Fatalf("eventbus aliases = %#v", aliases)
	}
}

func assertGoFileParses(t *testing.T, path string) {
	t.Helper()
	if _, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.AllErrors); err != nil {
		t.Fatalf("parse generated Go file %s: %v", path, err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func newScaffoldProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/scaffold\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	return root
}

func executeScaffoldCommand(command *cobra.Command, args ...string) (string, error) {
	var output strings.Builder
	command.SetArgs(args)
	command.SetOut(&output)
	command.SetErr(&output)
	command.SilenceErrors = true
	command.SilenceUsage = true
	err := command.Execute()
	return output.String(), err
}
