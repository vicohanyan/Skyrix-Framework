package platformcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProvidersLayoutSelection(t *testing.T) {
	const declaration = "package providers\n"
	writeProviders := func(t *testing.T, dir string) {
		t.Helper()
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "platform.go"), []byte(declaration), 0644); err != nil {
			t.Fatal(err)
		}
	}
	execute := func(args ...string) (string, error) {
		var output bytes.Buffer
		cmd := Command()
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs(args)
		err := cmd.Execute()
		return output.String(), err
	}
	for _, layout := range []string{"providers", "internal/providers"} {
		t.Run(layout, func(t *testing.T) {
			t.Chdir(t.TempDir())
			writeProviders(t, layout)
			if _, err := execute("list"); err != nil {
				t.Fatal(err)
			}
			if output, err := execute("remove", "example.com/unused"); err != nil || !strings.Contains(output, "No changes.") {
				t.Fatalf("remove: %q, %v", output, err)
			}
		})
	}
	t.Run("explicit directory overrides ambiguity", func(t *testing.T) {
		t.Chdir(t.TempDir())
		dirs := []string{"providers", "internal/providers"}
		for _, dir := range dirs {
			writeProviders(t, dir)
		}
		for _, args := range [][]string{
			{"list"}, {"remove", "example.com/unused"}, {"add", "example.com/unused"},
		} {
			_, err := execute(args...)
			if err == nil || !strings.Contains(err.Error(), "ambiguous application providers layout") || !strings.Contains(err.Error(), "--dir") {
				t.Fatalf("%v: got %v", args, err)
			}
		}
		for _, dir := range dirs {
			data, err := os.ReadFile(filepath.Join(dir, "platform.go"))
			if err != nil || string(data) != declaration {
				t.Fatalf("%s changed: %q, %v", dir, data, err)
			}
			if _, err := os.Stat(filepath.Join(dir, "platform_gen.go")); !os.IsNotExist(err) {
				t.Fatalf("%s: unexpected generated file: %v", dir, err)
			}
		}
		if _, err := execute("list", "--dir", "providers"); err != nil {
			t.Fatal(err)
		}
		if _, err := execute("remove", "example.com/unused", "--dir", "internal/providers"); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("missing", func(t *testing.T) {
		t.Chdir(t.TempDir())
		if _, err := execute("list"); err == nil || !strings.Contains(err.Error(), "application providers directory not found") {
			t.Fatalf("got %v", err)
		}
	})
}
