// Package integration holds the framework's integration harness for the
// Template: fixtures that copy the Template into temporary projects to check
// framework generators, Wire composition and the opt-in wiring of external
// libraries (Auth, Tenant). It is a separate test-only Go module, so neither
// framework nor Template depends on those libraries or on this directory.
//
// Missing fixtures and missing external libraries are test failures, never
// skips. Library locations default to siblings of the framework directory and
// can be overridden with SKYRIX_AUTH_LIB and SKYRIX_TENANT_LIB.
package integration

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// layout locates the modules the harness works with.
type layout struct {
	framework string
	template  string
	fixtures  string
	authLib   string
	tenantLib string
}

func projectLayout(t *testing.T) layout {
	t.Helper()
	here, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	framework := filepath.Dir(here)
	l := layout{
		framework: framework,
		template:  filepath.Join(framework, "template"),
		fixtures:  filepath.Join(here, "testdata"),
		authLib:   libraryDir("SKYRIX_AUTH_LIB", filepath.Join(filepath.Dir(framework), "skyrix-auth-lib")),
		tenantLib: libraryDir("SKYRIX_TENANT_LIB", filepath.Join(filepath.Dir(framework), "skyrix-tenant-lib")),
	}
	requireFile(t, filepath.Join(l.framework, "go.mod"), "framework module")
	requireFile(t, filepath.Join(l.template, "go.mod"), "Template module")
	return l
}

func libraryDir(env, fallback string) string {
	if dir := strings.TrimSpace(os.Getenv(env)); dir != "" {
		if abs, err := filepath.Abs(dir); err == nil {
			return abs
		}
		return dir
	}
	return fallback
}

// requireFile fails the test (never skips) when a required input is missing.
func requireFile(t *testing.T, path, what string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("required %s is missing: %v", what, err)
	}
}

// requireLibrary fails with an explicit message when an external integration
// dependency is unavailable.
func requireLibrary(t *testing.T, dir, module, env string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		t.Fatalf("external integration dependency %s is unavailable at %s (set %s): %v", module, dir, env, err)
	}
}

// copyTree copies regular files from src to dst, skipping the named
// top-level directories.
func copyTree(t *testing.T, src, dst string, skip ...string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if entry.IsDir() {
			for _, name := range skip {
				if rel == name {
					return filepath.SkipDir
				}
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dst, rel)), 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// patchFile replaces the first occurrence of old; a missing old is a failure.
func patchFile(t *testing.T, path, old, replacement string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), old) {
		t.Fatalf("%s: %q not found", path, old)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), old, replacement, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// goCommand runs a command and fails with its output on error.
func goCommand(t *testing.T, dir string, env []string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, output)
	}
	return string(output)
}
