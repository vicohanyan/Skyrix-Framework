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

func TestEventBusInitCreatesRoleBasedStructure(t *testing.T) {
	root := newScaffoldProject(t)
	command := &EventBusCommand{ProjectRoot: root}

	output, err := executeScaffoldCommand(command.ToCobraCommand(), "init")
	if err != nil {
		t.Fatalf("eventbus init: %v", err)
	}
	for _, expected := range []string{"eventbus initialized", "next incoming", "next outgoing"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("init output does not contain %q:\n%s", expected, output)
		}
	}

	rootDir := eventBusTransportRoot(root)
	for _, path := range []string{
		"provider.go",
		"events/envelope.go",
		"consumers/provider.go",
		"publishers/provider.go",
		"subscribers/provider.go",
	} {
		assertGoFileParses(t, filepath.Join(rootDir, path))
	}
	provider := readTestFile(t, filepath.Join(rootDir, "provider.go"))
	for _, expected := range []string{
		`example.com/scaffold/internal/transport/eventbus/consumers`,
		"consumers.ProviderSet", "publishers.ProviderSet", "subscribers.ProviderSet",
	} {
		if !strings.Contains(provider, expected) {
			t.Fatalf("root provider does not contain %q:\n%s", expected, provider)
		}
	}
	if _, err := executeScaffoldCommand(command.ToCobraCommand(), "init"); err == nil {
		t.Fatal("second init error = nil")
	}
}

func TestEventBusComponentLifecycle(t *testing.T) {
	root := newScaffoldProject(t)
	command := &EventBusCommand{ProjectRoot: root}
	name := "CatalogPaymentHoldRequested"

	if _, err := executeScaffoldCommand(command.ToCobraCommand(), "consumer", "create", name); err == nil {
		t.Fatal("consumer without event error = nil")
	}
	output, err := executeScaffoldCommand(
		command.ToCobraCommand(), "event", "create", name,
		"--subject", "catalog.payment_hold.requested.v1",
	)
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	if !strings.Contains(output, "event created") || !strings.Contains(output, "incoming next") {
		t.Fatalf("event create output = %q", output)
	}
	if _, err := executeScaffoldCommand(command.ToCobraCommand(), "subscriber", "create", name); err == nil {
		t.Fatal("subscriber without consumer error = nil")
	}
	for _, role := range []string{"consumer", "publisher", "subscriber"} {
		if _, err := executeScaffoldCommand(command.ToCobraCommand(), role, "create", name); err != nil {
			t.Fatalf("create %s: %v", role, err)
		}
	}

	rootDir := eventBusTransportRoot(root)
	filename := "catalogPaymentHoldRequested.go"
	for _, role := range []string{"events", "consumers", "publishers", "subscribers"} {
		assertGoFileParses(t, filepath.Join(rootDir, role, filename))
	}
	event := readTestFile(t, filepath.Join(rootDir, "events", filename))
	for _, expected := range []string{
		"package events", "SubjectCatalogPaymentHoldRequested",
		`"catalog.payment_hold.requested.v1"`, "type CatalogPaymentHoldRequested struct",
	} {
		if !strings.Contains(event, expected) {
			t.Fatalf("event does not contain %q:\n%s", expected, event)
		}
	}
	consumer := readTestFile(t, filepath.Join(rootDir, "consumers", filename))
	if !strings.Contains(consumer, "events.Envelope[events.CatalogPaymentHoldRequested]") ||
		!strings.Contains(consumer, "handler CatalogPaymentHoldRequestedHandler") {
		t.Fatalf("consumer does not use typed envelope and handler:\n%s", consumer)
	}
	subscriber := readTestFile(t, filepath.Join(rootDir, "subscribers", filename))
	if !strings.Contains(subscriber, "*consumers.CatalogPaymentHoldRequestedConsumer") ||
		!strings.Contains(subscriber, "events.SubjectCatalogPaymentHoldRequested") {
		t.Fatalf("subscriber does not bind consumer and subject:\n%s", subscriber)
	}

	for role, constructor := range map[string]string{
		"consumers":   "NewCatalogPaymentHoldRequestedConsumer",
		"publishers":  "NewCatalogPaymentHoldRequestedPublisher",
		"subscribers": "NewCatalogPaymentHoldRequestedSubscriber",
	} {
		provider := readTestFile(t, filepath.Join(rootDir, role, "provider.go"))
		if !strings.Contains(provider, constructor) {
			t.Fatalf("%s provider does not contain %s:\n%s", role, constructor, provider)
		}
	}

	list, err := executeScaffoldCommand(command.ToCobraCommand(), "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if list != "catalogPaymentHoldRequested: event, consumer, subscriber, publisher\n" {
		t.Fatalf("list output = %q", list)
	}

	if _, err := executeScaffoldCommand(command.ToCobraCommand(), "remove", name); err != nil {
		t.Fatalf("remove: %v", err)
	}
	for _, role := range []string{"events", "consumers", "publishers", "subscribers"} {
		if _, err := os.Stat(filepath.Join(rootDir, role, filename)); !os.IsNotExist(err) {
			t.Fatalf("%s component still exists: %v", role, err)
		}
	}
}

func TestEventBusWorksWithoutGoMod(t *testing.T) {
	root := t.TempDir()
	command := &EventBusCommand{ProjectRoot: root, ModulePath: "skyrix"}
	name := "PaymentRequested"

	if _, err := executeScaffoldCommand(
		command.ToCobraCommand(), "event", "create", name, "--subject", "payment.requested.v1",
	); err != nil {
		t.Fatalf("create event without go.mod: %v", err)
	}
	if _, err := executeScaffoldCommand(command.ToCobraCommand(), "consumer", "create", name); err != nil {
		t.Fatalf("create consumer without go.mod: %v", err)
	}
	consumer := readTestFile(t, filepath.Join(eventBusTransportRoot(root), "consumers", "paymentRequested.go"))
	if !strings.Contains(consumer, `"skyrix/internal/transport/eventbus/events"`) {
		t.Fatalf("consumer does not use embedded module path:\n%s", consumer)
	}
}

func TestEventBusCommandValidatesAndProtectsUnmanagedFiles(t *testing.T) {
	root := newScaffoldProject(t)
	command := &EventBusCommand{ProjectRoot: root}
	for _, invalid := range []string{"", "../Payment", "123Payment", "pay ment", "package"} {
		if _, err := executeScaffoldCommand(
			command.ToCobraCommand(), "event", "create", invalid, "--subject", "payment.requested.v1",
		); err == nil {
			t.Fatalf("create invalid event %q error = nil", invalid)
		}
	}
	if _, err := executeScaffoldCommand(command.ToCobraCommand(), "init", "extra"); err == nil {
		t.Fatal("init with argument error = nil")
	}
	if aliases := command.ToCobraCommand().Aliases; len(aliases) != 1 || aliases[0] != "bus" {
		t.Fatalf("aliases = %#v", aliases)
	}

	if _, err := executeScaffoldCommand(
		command.ToCobraCommand(), "event", "create", "PaymentRequested", "--subject", "payment.requested.v1",
	); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(eventBusTransportRoot(root), "events", "paymentRequested.go")
	if err := os.WriteFile(path, []byte("package events\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := executeScaffoldCommand(command.ToCobraCommand(), "remove", "PaymentRequested"); err == nil {
		t.Fatal("remove unmanaged event error = nil")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("unmanaged event was removed: %v", err)
	}
}

func TestEventBusHelpExplainsCorrectWorkflow(t *testing.T) {
	command := (&EventBusCommand{}).ToCobraCommand()
	output, err := executeScaffoldCommand(command, "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"go get gitlab.com/skyrix-lib/eventbus@v1.0.0",
		"internal/transport/eventbus/consumers",
		"eventbus consumer create CatalogPaymentHoldRequested",
		"Catalog and\nDelivery belong in subjects",
		"No Saga or application-layer code is generated",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("help does not contain %q:\n%s", expected, output)
		}
	}

	tests := []struct {
		args []string
		want string
	}{
		{[]string{"event", "create", "--help"}, "Create only an event subject"},
		{[]string{"consumer", "create", "--help"}, "does not subscribe the consumer"},
		{[]string{"publisher", "create", "--help"}, "Bus.PublishJSON"},
		{[]string{"subscriber", "create", "--help"}, "drained during application shutdown"},
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
