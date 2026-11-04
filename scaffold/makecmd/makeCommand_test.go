package makecmd

import (
	"go/parser"
	"go/token"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.com/skyrix-lib/framework/database/migrate/atlas/atlastest"
	support "gitlab.com/skyrix-lib/framework/scaffold"

	"github.com/spf13/cobra"
)

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

// withoutAtlasCLI makes sure no atlas binary can be found.
func withoutAtlasCLI(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	if _, err := exec.LookPath("atlas"); err == nil {
		t.Fatal("atlas still in PATH")
	}
}

func TestMakeMigrationCreatesFileAndChecksumWithoutAtlas(t *testing.T) {
	withoutAtlasCLI(t)
	root := newScaffoldProject(t)
	now := time.Date(2026, 11, 2, 13, 52, 50, 0, time.UTC)
	command := &MakeCommand{ProjectRoot: root, Now: func() time.Time { return now }}

	// First migration on an empty project creates the file and atlas.sum.
	output, err := executeScaffoldCommand(command.Command(), "migration", "add_trigger")
	if err != nil {
		t.Fatalf("make migration error = %v", err)
	}
	path := filepath.Join(root, "database/migrations", "20261102135250_add_trigger.sql")
	if content := readTestFile(t, path); content != "" {
		t.Fatalf("migration content = %q, want empty", content)
	}
	firstSum := readTestFile(t, filepath.Join(root, "database/migrations", "atlas.sum"))
	if !strings.HasPrefix(firstSum, "h1:") || !strings.Contains(firstSum, "\n20261102135250_add_trigger.sql h1:") {
		t.Fatalf("atlas.sum = %q", firstSum)
	}
	if !strings.Contains(output, "migration created: database/migrations/20261102135250_add_trigger.sql") || !strings.Contains(output, `"db migrate:hash"`) {
		t.Fatalf("make migration output = %q", output)
	}

	if _, err := executeScaffoldCommand(command.Command(), "migration", "other", "--notx"); err == nil {
		t.Fatal("make migration with an existing version error = nil")
	}

	// The next migration verifies the existing checksum, then updates it.
	now = now.Add(time.Second)
	if _, err := executeScaffoldCommand(command.Command(), "migration", "build_index", "--notx"); err != nil {
		t.Fatalf("make notx migration error = %v", err)
	}
	readTestFile(t, filepath.Join(root, "database/migrations", "20261102135251_build_index.notx.sql"))
	secondSum := readTestFile(t, filepath.Join(root, "database/migrations", "atlas.sum"))
	if secondSum == firstSum || !strings.Contains(secondSum, "20261102135251_build_index.notx.sql") {
		t.Fatalf("atlas.sum not updated: %q", secondSum)
	}

	if _, err := executeScaffoldCommand(command.Command(), "migration", "Add-Trigger"); err == nil {
		t.Fatal("make migration accepted an invalid name")
	}
}

func TestMakeMigrationWritesNothingOnError(t *testing.T) {
	withoutAtlasCLI(t)
	now := time.Date(2026, 11, 2, 13, 52, 50, 0, time.UTC)
	for _, test := range []struct {
		name  string
		setup func(t *testing.T, root string)
		want  string
	}{
		{
			name: "checksum missing",
			setup: func(t *testing.T, root string) {
				writeProjectFile(t, root, "database/migrations/20261101000000_existing.sql", "SELECT 1;")
			},
			want: `"db migrate:hash"`,
		},
		{
			name: "checksum mismatch",
			setup: func(t *testing.T, root string) {
				writeProjectFile(t, root, "database/migrations/20261101000000_existing.sql", "SELECT 1;")
				writeProjectFile(t, root, "database/migrations/atlas.sum", "h1:stale\n")
			},
			want: "does not match",
		},
		{
			name: "checksum without SQL files does not match",
			setup: func(t *testing.T, root string) {
				writeProjectFile(t, root, "database/migrations/atlas.sum", "user checksum\n")
			},
			want: "does not match",
		},
		{
			name: "version already exists",
			setup: func(t *testing.T, root string) {
				writeProjectFile(t, root, "database/migrations/20261102135250_other.sql", "SELECT 1;")
			},
			want: "already exists",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := newScaffoldProject(t)
			writeProjectFile(t, root, "database/migrations/README.md", "user placeholder")
			writeProjectFile(t, root, "notes.txt", "user file")
			if test.setup != nil {
				test.setup(t, root)
			}
			command := &MakeCommand{ProjectRoot: root, Now: func() time.Time { return now }}
			before := atlastest.Snapshot(t, root)
			_, err := executeScaffoldCommand(command.Command(), "migration", "add_trigger")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("make migration error = %v, want %q", err, test.want)
			}
			if after := atlastest.Snapshot(t, root); !maps.Equal(before, after) {
				t.Fatalf("project changed:\nbefore %v\nafter  %v", before, after)
			}
		})
	}
}

func writeProjectFile(t *testing.T, root, relative, data string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
