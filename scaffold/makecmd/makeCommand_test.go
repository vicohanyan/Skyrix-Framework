package makecmd

import (
	"errors"
	"go/parser"
	"go/token"
	"maps"
	"os"
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

func TestMakeMigrationCreatesFileAndChecksum(t *testing.T) {
	binary, log := atlastest.Install(t)
	root := newScaffoldProject(t)
	now := time.Date(2026, 11, 2, 13, 52, 50, 0, time.UTC)
	command := &MakeCommand{
		ProjectRoot: root,
		Now:         func() time.Time { return now },
		LookupAtlas: func() (string, error) { return binary, nil },
	}

	// First migration on an empty project creates the file and atlas.sum.
	output, err := executeScaffoldCommand(command.Command().Cobra(), "migration", "add_trigger")
	if err != nil {
		t.Fatalf("make migration error = %v", err)
	}
	path := filepath.Join(root, "migrations", "20261102135250_add_trigger.sql")
	if content := readTestFile(t, path); content != "" {
		t.Fatalf("migration content = %q, want empty", content)
	}
	firstSum := readTestFile(t, filepath.Join(root, "migrations", "atlas.sum"))
	if !strings.Contains(firstSum, "20261102135250_add_trigger.sql") {
		t.Fatalf("atlas.sum = %q", firstSum)
	}
	if !strings.Contains(output, "migration created: migrations/20261102135250_add_trigger.sql") || !strings.Contains(output, "atlas migrate hash") {
		t.Fatalf("make migration output = %q", output)
	}

	if _, err := executeScaffoldCommand(command.Command().Cobra(), "migration", "other", "--notx"); err == nil {
		t.Fatal("make migration with an existing version error = nil")
	}

	// The next migration verifies the existing checksum, then updates it.
	now = now.Add(time.Second)
	calls := len(atlastest.Invocations(t, log))
	if _, err := executeScaffoldCommand(command.Command().Cobra(), "migration", "build_index", "--notx"); err != nil {
		t.Fatalf("make notx migration error = %v", err)
	}
	readTestFile(t, filepath.Join(root, "migrations", "20261102135251_build_index.notx.sql"))
	secondSum := readTestFile(t, filepath.Join(root, "migrations", "atlas.sum"))
	if secondSum == firstSum || !strings.Contains(secondSum, "20261102135251_build_index.notx.sql") {
		t.Fatalf("atlas.sum not updated: %q", secondSum)
	}
	if got := len(atlastest.Invocations(t, log)) - calls; got != 2 {
		t.Fatalf("atlas calls for the second migration = %d, want verify + update", got)
	}

	if _, err := executeScaffoldCommand(command.Command().Cobra(), "migration", "Add-Trigger"); err == nil {
		t.Fatal("make migration accepted an invalid name")
	}
}

func TestMakeMigrationWritesNothingOnError(t *testing.T) {
	binary, _ := atlastest.Install(t)
	now := time.Date(2026, 11, 2, 13, 52, 50, 0, time.UTC)
	for _, test := range []struct {
		name  string
		setup func(t *testing.T, root string)
		atlas func() (string, error)
		want  string
	}{
		{
			name:  "atlas missing",
			atlas: func() (string, error) { return "", errors.New("atlas CLI is not installed") },
			want:  "needs the Atlas CLI",
		},
		{
			name: "checksum missing",
			setup: func(t *testing.T, root string) {
				writeProjectFile(t, root, "migrations/20261101000000_existing.sql", "SELECT 1;")
			},
			want: "atlas migrate hash --dir file://migrations",
		},
		{
			name: "checksum mismatch",
			setup: func(t *testing.T, root string) {
				writeProjectFile(t, root, "migrations/20261101000000_existing.sql", "SELECT 1;")
				writeProjectFile(t, root, "migrations/atlas.sum", "h1:stale\n")
			},
			want: "does not match",
		},
		{
			name: "hash fails",
			setup: func(t *testing.T, root string) {
				t.Setenv("FAKE_ATLAS_FAIL", "hash")
			},
			want: "nothing was written",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := newScaffoldProject(t)
			writeProjectFile(t, root, "migrations/README.md", "user placeholder")
			writeProjectFile(t, root, "notes.txt", "user file")
			if test.setup != nil {
				test.setup(t, root)
			}
			lookup := test.atlas
			if lookup == nil {
				lookup = func() (string, error) { return binary, nil }
			}
			command := &MakeCommand{ProjectRoot: root, Now: func() time.Time { return now }, LookupAtlas: lookup}
			before := atlastest.Snapshot(t, root)
			_, err := executeScaffoldCommand(command.Command().Cobra(), "migration", "add_trigger")
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
