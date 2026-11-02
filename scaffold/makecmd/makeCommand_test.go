package makecmd

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	support "gitlab.com/skyrix-lib/framework/scaffold"

	"github.com/spf13/cobra"
)

func TestMakeCommandCreatesDomainRepositoryAndService(t *testing.T) {
	root := newScaffoldProject(t)
	command := &MakeCommand{ProjectRoot: root}

	if _, err := executeScaffoldCommand(command.Command().Cobra(), "repository", "Payment"); err == nil {
		t.Fatal("repository before domain error = nil, want error")
	}

	output, err := executeScaffoldCommand(command.Command().Cobra(), "domain", "Payment")
	if err != nil {
		t.Fatalf("make domain error = %v", err)
	}
	if output != "domain created: payment\n" {
		t.Fatalf("make domain output = %q", output)
	}
	domain := command.domainDirectory(root, "payment")
	for _, path := range []string{
		"provider.go",
		"dto/doc.go",
		"entity/doc.go",
		"interfaces/doc.go",
		"repository/doc.go",
		"services/doc.go",
		"unitOfWork/doc.go",
	} {
		assertGoFileParses(t, filepath.Join(domain, path))
	}

	if _, err := executeScaffoldCommand(command.Command().Cobra(), "repository", "Payment"); err != nil {
		t.Fatalf("make repository error = %v", err)
	}
	if _, err := executeScaffoldCommand(command.Command().Cobra(), "service", "Payment"); err != nil {
		t.Fatalf("make service error = %v", err)
	}
	repositoryPath := filepath.Join(domain, "repository", "paymentRepository.go")
	servicePath := filepath.Join(domain, "services", "paymentService.go")
	assertGoFileParses(t, repositoryPath)
	assertGoFileParses(t, servicePath)
	if !strings.Contains(readTestFile(t, repositoryPath), "type PaymentRepository struct") {
		t.Fatalf("repository does not use normalized type name")
	}
	if !strings.Contains(readTestFile(t, servicePath), "type PaymentService struct") {
		t.Fatalf("service does not use normalized type name")
	}

	for _, args := range [][]string{{"domain", "Payment"}, {"repository", "Payment"}, {"service", "Payment"}} {
		if _, err := executeScaffoldCommand(command.Command().Cobra(), args...); err == nil {
			t.Fatalf("duplicate make %v error = nil, want error", args)
		}
	}
	if _, err := os.Stat(repositoryPath); err != nil {
		t.Fatalf("duplicate command removed repository: %v", err)
	}
}

func TestNormalizeScaffoldName(t *testing.T) {
	tests := []struct {
		raw         string
		wantPackage string
		wantType    string
	}{
		{"Payment", "payment", "Payment"},
		{"PAYMENT", "payment", "Payment"},
		{"payment_gateway", "paymentgateway", "PaymentGateway"},
		{"Invoice2", "invoice2", "Invoice2"},
	}
	for _, test := range tests {
		name, err := support.NormalizeName(test.raw)
		if err != nil {
			t.Fatalf("normalize %q error = %v", test.raw, err)
		}
		if name.Package != test.wantPackage || name.Type != test.wantType {
			t.Fatalf("normalize %q = %#v, want package=%q type=%q", test.raw, name, test.wantPackage, test.wantType)
		}
	}
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

func TestMakeMigrationCreatesEmptyVersionedFile(t *testing.T) {
	root := newScaffoldProject(t)
	now := time.Date(2026, 11, 2, 13, 52, 50, 0, time.UTC)
	command := &MakeCommand{
		ProjectRoot: root,
		Now:         func() time.Time { return now },
		LookupAtlas: func() (string, error) { return "", errors.New("atlas missing") },
	}

	output, err := executeScaffoldCommand(command.Command().Cobra(), "migration", "add_trigger")
	if err != nil {
		t.Fatalf("make migration error = %v", err)
	}
	path := filepath.Join(root, "migrations", "20261102135250_add_trigger.sql")
	if content := readTestFile(t, path); content != "" {
		t.Fatalf("migration content = %q, want empty", content)
	}
	if !strings.Contains(output, "migration created: migrations/20261102135250_add_trigger.sql") || !strings.Contains(output, "atlas migrate hash") {
		t.Fatalf("make migration output = %q", output)
	}

	if _, err := executeScaffoldCommand(command.Command().Cobra(), "migration", "other", "--notx"); err == nil {
		t.Fatal("make migration with an existing version error = nil")
	}
	now = now.Add(time.Second)
	if _, err := executeScaffoldCommand(command.Command().Cobra(), "migration", "build_index", "--notx"); err != nil {
		t.Fatalf("make notx migration error = %v", err)
	}
	readTestFile(t, filepath.Join(root, "migrations", "20261102135251_build_index.notx.sql"))

	if _, err := executeScaffoldCommand(command.Command().Cobra(), "migration", "Add-Trigger"); err == nil {
		t.Fatal("make migration accepted an invalid name")
	}
}
