package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDirectivesPreservesOrder(t *testing.T) {
	path := writeDirectiveFile(t, `package providers
//skyrix:platform example.com/first/platform config=First
//skyrix:platform example.com/second/platform
`)
	directives, err := parseDirectives(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(directives) != 2 || directives[0].ImportPath != "example.com/first/platform" || directives[0].Config != "First" || directives[1].ImportPath != "example.com/second/platform" {
		t.Fatalf("directives = %#v", directives)
	}
}

func TestParseDirectivesRejectsDuplicates(t *testing.T) {
	path := writeDirectiveFile(t, `package providers
//skyrix:platform example.com/eventbus/platform
//skyrix:platform example.com/eventbus/platform
`)
	if _, err := parseDirectives(path); err == nil {
		t.Fatal("duplicate directive error = nil")
	}
}

func TestParseDirectivesRejectsInvalidConfigOption(t *testing.T) {
	path := writeDirectiveFile(t, `package providers
//skyrix:platform example.com/eventbus/platform config=bad-field
`)
	if _, err := parseDirectives(path); err == nil {
		t.Fatal("invalid config error = nil")
	}
}

func writeDirectiveFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "platform.go")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDefaultConfigImportUsesNearestModule(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	providers := filepath.Join(root, "providers")
	if err := os.MkdirAll(providers, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := defaultConfigImport(providers)
	if err != nil || got != "example.com/app/internal/config" {
		t.Fatalf("defaultConfigImport() = %q, %v", got, err)
	}
}

func TestRenderUsesConfigImport(t *testing.T) {
	generated, err := render([]*directive{{ImportPath: "example.com/bus/platform", Config: "Bus", Package: "platform", Alias: "busplatform"}}, "example.com/app/internal/config")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"example.com/app/internal/config"`, `wire.FieldsOf(new(*config.Config), "Bus")`, "kernel.NewPlatform(", "frameworkcobra.ModuleCommands("} {
		if !strings.Contains(string(generated), want) {
			t.Fatalf("generated code does not contain %q:\n%s", want, generated)
		}
	}
}

// A commented-out directive is not a directive.
func TestParseDirectivesIgnoresCommentedDirectives(t *testing.T) {
	path := writeDirectiveFile(t, `package providers
// //skyrix:platform example.com/eventbus/platform config=EventBus
`)
	directives, err := parseDirectives(path)
	if err != nil || len(directives) != 0 {
		t.Fatalf("directives = %#v, %v; want none", directives, err)
	}
}

// Without directives the generated code is an empty but valid platform.
func TestRenderWithoutDirectives(t *testing.T) {
	generated, err := render(nil, "example.com/app/internal/config")
	if err != nil {
		t.Fatal(err)
	}
	code := string(generated)
	for _, want := range []string{"var PlatformSet = wire.NewSet(\n\tProvidePlatform,\n)", "func ProvidePlatform() (*kernel.Platform, error)", "frameworkcobra.ModuleCommands()"} {
		if !strings.Contains(code, want) {
			t.Fatalf("generated code does not contain %q:\n%s", want, code)
		}
	}
	if strings.Contains(code, "internal/config") {
		t.Fatalf("generated code imports the config package without config directives:\n%s", code)
	}
}
