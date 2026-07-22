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

	for _, name := range []string{"Payment", "Billing"} {
		if _, err := executeScaffoldCommand(command.ToCobraCommand(), "module", "create", name); err != nil {
			t.Fatalf("eventbus create %s error = %v", name, err)
		}
	}
	assertGoFileParses(t, filepath.Join(eventBusTransportRoot(root), "payment", "provider.go"))
	assertGoFileParses(t, filepath.Join(eventBusTransportRoot(root), "billing", "provider.go"))

	output, err = executeScaffoldCommand(command.ToCobraCommand(), "module", "list")
	if err != nil {
		t.Fatalf("eventbus list error = %v", err)
	}
	if output != "billing\npayment\n" {
		t.Fatalf("eventbus list output = %q", output)
	}

	registrationPath := filepath.Join(eventBusTransportRoot(root), "provider.go")
	registration := readTestFile(t, registrationPath)
	for _, expected := range []string{"billing.ProviderSet", "payment.ProviderSet", "example.com/scaffold/internal/transport/eventbus/v1/payment"} {
		if !strings.Contains(registration, expected) {
			t.Fatalf("registration does not contain %q:\n%s", expected, registration)
		}
	}

	if _, err := executeScaffoldCommand(command.ToCobraCommand(), "module", "create", "Payment"); err == nil {
		t.Fatal("duplicate eventbus create error = nil, want error")
	}
	if _, err := executeScaffoldCommand(command.ToCobraCommand(), "module", "remove", "Payment"); err != nil {
		t.Fatalf("eventbus remove error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(eventBusTransportRoot(root), "payment")); !os.IsNotExist(err) {
		t.Fatalf("removed payment module stat error = %v, want not exist", err)
	}
	registration = readTestFile(t, registrationPath)
	if strings.Contains(registration, "payment.ProviderSet") {
		t.Fatalf("registration still contains removed payment module:\n%s", registration)
	}
	if !strings.Contains(registration, "billing.ProviderSet") {
		t.Fatalf("registration lost billing module:\n%s", registration)
	}
	assertGoFileParses(t, registrationPath)
}

func TestEventBusCommandValidatesArgumentsAndManagedModules(t *testing.T) {
	root := newScaffoldProject(t)
	command := &EventBusCommand{ProjectRoot: root}

	output, err := executeScaffoldCommand(command.ToCobraCommand(), "module", "create", "Payment")
	if err != nil {
		t.Fatalf("create with automatic init error = %v", err)
	}
	if !strings.Contains(output, "eventbus initialized") || !strings.Contains(output, "eventbus module created: payment") {
		t.Fatalf("automatic init output = %q", output)
	}
	if _, err := executeScaffoldCommand(command.ToCobraCommand(), "init", "extra"); err == nil {
		t.Fatal("init with argument error = nil, want error")
	}
	if _, err := executeScaffoldCommand(command.ToCobraCommand(), "init"); err == nil {
		t.Fatal("init after automatic initialization error = nil, want error")
	}
	for _, invalid := range []string{"", "../Payment", "123Payment", "pay ment", "package"} {
		if _, err := executeScaffoldCommand(command.ToCobraCommand(), "module", "create", invalid); err == nil {
			t.Fatalf("create %q error = nil, want error", invalid)
		}
	}

	unmanaged := filepath.Join(eventBusTransportRoot(root), "manual")
	if err := os.MkdirAll(unmanaged, 0o755); err != nil {
		t.Fatalf("mkdir unmanaged module: %v", err)
	}
	if _, err := executeScaffoldCommand(command.ToCobraCommand(), "module", "remove", "Manual"); err == nil {
		t.Fatal("remove unmanaged module error = nil, want error")
	}
	if _, err := os.Stat(unmanaged); err != nil {
		t.Fatalf("unmanaged module was modified: %v", err)
	}

	if aliases := command.ToCobraCommand().Aliases; len(aliases) != 1 || aliases[0] != "bus" {
		t.Fatalf("eventbus aliases = %#v", aliases)
	}
}

func TestEventBusCommandHelpExplainsInstallationAndPackageCreation(t *testing.T) {
	command := (&EventBusCommand{ProjectRoot: newScaffoldProject(t)}).ToCobraCommand()

	output, err := executeScaffoldCommand(command, "--help")
	if err != nil {
		t.Fatalf("eventbus help error = %v", err)
	}
	for _, expected := range []string{
		"go get gitlab.com/skyrix-lib/eventbus@v1.0.0",
		"eventbus module create Payment",
		"eventbus event create Payment PaymentRequested",
		"No Saga or application-layer code is",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("eventbus help does not contain %q:\n%s", expected, output)
		}
	}

	output, err = executeScaffoldCommand((&EventBusCommand{}).ToCobraCommand(), "module", "create", "--help")
	if err != nil {
		t.Fatalf("eventbus create help error = %v", err)
	}
	for _, expected := range []string{
		"internal/transport/eventbus/v1/payment/provider.go",
		"it is initialized",
		"does not create consumers, subscribers, publishers, or events",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("eventbus create help does not contain %q:\n%s", expected, output)
		}
	}
}

func TestEventBusComponentLifecycle(t *testing.T) {
	root := newScaffoldProject(t)
	command := &EventBusCommand{ProjectRoot: root}

	if _, err := executeScaffoldCommand(command.ToCobraCommand(), "module", "create", "Payment"); err != nil {
		t.Fatalf("create module: %v", err)
	}
	if _, err := executeScaffoldCommand(command.ToCobraCommand(), "consumer", "create", "Payment", "PaymentRequested"); err == nil {
		t.Fatal("consumer without event error = nil, want error")
	}
	if _, err := executeScaffoldCommand(
		command.ToCobraCommand(),
		"event", "create", "Payment", "PaymentRequested",
		"--subject", "payment.requested.v1",
	); err != nil {
		t.Fatalf("create event: %v", err)
	}
	if _, err := executeScaffoldCommand(command.ToCobraCommand(), "subscriber", "create", "Payment", "PaymentRequested"); err == nil {
		t.Fatal("subscriber without consumer error = nil, want error")
	}
	for _, component := range []string{"consumer", "publisher", "subscriber"} {
		if _, err := executeScaffoldCommand(command.ToCobraCommand(), component, "create", "Payment", "PaymentRequested"); err != nil {
			t.Fatalf("create %s: %v", component, err)
		}
	}

	moduleDir := filepath.Join(eventBusTransportRoot(root), "payment")
	for _, filename := range []string{
		"paymentRequestedEvent.go",
		"paymentRequestedConsumer.go",
		"paymentRequestedPublisher.go",
		"paymentRequestedSubscriber.go",
		"provider.go",
	} {
		assertGoFileParses(t, filepath.Join(moduleDir, filename))
	}

	eventContent := readTestFile(t, filepath.Join(moduleDir, "paymentRequestedEvent.go"))
	for _, expected := range []string{"SubjectPaymentRequested", `"payment.requested.v1"`, "type PaymentRequested struct"} {
		if !strings.Contains(eventContent, expected) {
			t.Fatalf("generated event does not contain %q:\n%s", expected, eventContent)
		}
	}

	provider := readTestFile(t, filepath.Join(moduleDir, "provider.go"))
	for _, expected := range []string{
		"NewPaymentRequestedConsumer",
		"NewPaymentRequestedPublisher",
		"NewPaymentRequestedSubscriber",
	} {
		if !strings.Contains(provider, expected) {
			t.Fatalf("module provider does not contain %q:\n%s", expected, provider)
		}
	}

	if _, err := executeScaffoldCommand(
		command.ToCobraCommand(),
		"event", "create", "Payment", "PaymentRequested",
		"--subject", "payment.requested.v1",
	); err == nil {
		t.Fatal("duplicate event error = nil, want error")
	}
}

func TestEventBusComponentHelpStatesGeneratedArtifact(t *testing.T) {
	command := (&EventBusCommand{}).ToCobraCommand()
	tests := []struct {
		args []string
		want string
	}{
		{args: []string{"event", "create", "--help"}, want: "only the event contract"},
		{args: []string{"consumer", "create", "--help"}, want: "does not subscribe the consumer"},
		{args: []string{"publisher", "create", "--help"}, want: "Bus.PublishJSON"},
		{args: []string{"subscriber", "create", "--help"}, want: "must drain it during"},
	}
	for _, test := range tests {
		output, err := executeScaffoldCommand(command, test.args...)
		if err != nil {
			t.Fatalf("help %v: %v", test.args, err)
		}
		if !strings.Contains(output, test.want) {
			t.Fatalf("help %v does not contain %q:\n%s", test.args, test.want, output)
		}
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
