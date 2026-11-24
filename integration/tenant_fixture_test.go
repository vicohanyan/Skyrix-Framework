package integration

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestTenantFixture proves the documented opt-in wiring of
// gitlab.com/skyrix-lib/tenant: it copies the Template into a temporary
// project, overlays testdata/tenant, generates Wire twice (which must be
// repeatable), builds everything and runs the fixture test that serves
// requests through the generated graph. The default Template never imports
// the tenant module.
func TestTenantFixture(t *testing.T) {
	l := projectLayout(t)
	overlay := filepath.Join(l.fixtures, "tenant")
	for _, name := range []string{
		"internal/providers/tenant.go", "internal/config/tenant.go",
		"cmd/http/wire.go", "cmd/http/tenant_fixture_test.go", "tools/schemaloader/main.go",
	} {
		requireFile(t, filepath.Join(overlay, name), "tenant fixture "+name)
	}
	requireLibrary(t, l.authLib, "gitlab.com/skyrix-lib/auth", "SKYRIX_AUTH_LIB")
	requireLibrary(t, l.tenantLib, "gitlab.com/skyrix-lib/tenant", "SKYRIX_TENANT_LIB")
	dir := t.TempDir()

	// Copy the Template sources (without generated logs or secrets), then
	// overlay the tenant wiring.
	copyTree(t, l.template, dir, ".git", "logs", "secret", ".idea")
	copyTree(t, overlay, dir)
	file := filepath.Join(dir, "go.mod")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, append(data, []byte("\nrequire gitlab.com/skyrix-lib/tenant v0.0.0\n")...), 0644); err != nil {
		t.Fatal(err)
	}
	// List the tenant section for config:env, as Template README "Tenants" says.
	patchFile(t, filepath.Join(dir, "internal/config/config.go"),
		`{Key: "migrations", Value: &Migrations{}, YAMLPath: []string{"MIGRATIONS"}},`,
		`{Key: "migrations", Value: &Migrations{}, YAMLPath: []string{"MIGRATIONS"}},
		{Key: "tenant", Value: &TenantCache{}, YAMLPath: []string{"TENANT_CACHE"}},`)

	env := workspaceEnv(t, l, dir, true)
	wire := strings.TrimSpace(goCommand(t, dir, env, "go", "tool", "-n", "wire"))
	generated := filepath.Join(dir, "cmd/http/wire_gen.go")
	goCommand(t, dir, env, wire, "gen", "-generate_command", "go tool wire", "./cmd/http")
	first, err := os.ReadFile(generated)
	if err != nil {
		t.Fatal(err)
	}
	goCommand(t, dir, env, wire, "gen", "-generate_command", "go tool wire", "./cmd/http")
	if second, _ := os.ReadFile(generated); string(second) != string(first) {
		t.Fatal("Wire generation is not repeatable")
	}
	code := string(first)
	for _, want := range []string{"providers.ProvideTenantAPIMiddleware(authMiddleware, tenantMiddleware)", "router.ProvideRouter("} {
		if !strings.Contains(code, want) {
			t.Fatalf("generated graph lacks %s", want)
		}
	}
	if strings.Contains(code, "ProvideAPIMiddleware(") {
		t.Fatal("generated graph still uses the default API chain")
	}

	goCommand(t, dir, env, "go", "build", "./...")
	goCommand(t, dir, env, "go", "vet", "./cmd/http", "./internal/...")
	output := goCommand(t, dir, env, "go", "test", "-count=1", "-v", "-run", "TestTenantMiddlewareInGraph", "./cmd/http")
	if !strings.Contains(output, "--- PASS: TestTenantMiddlewareInGraph") {
		t.Fatalf("fixture middleware test did not run:\n%s", output)
	}
	if output := goCommand(t, dir, env, "go", "run", "./cmd/cobra", "config:env", "--section", "tenant"); !strings.Contains(output, "TENANT_CACHE_TTL") {
		t.Fatalf("config:env lacks the tenant section:\n%s", output)
	}
}

// TestDefaultTemplateWithoutTenant keeps Tenant opt-in: the default Template
// neither requires nor imports gitlab.com/skyrix-lib/tenant, so no binary
// links its settings, cache or repository.
func TestDefaultTemplateWithoutTenant(t *testing.T) {
	l := projectLayout(t)
	goMod, err := os.ReadFile(filepath.Join(l.template, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(goMod), "skyrix-lib/tenant") {
		t.Fatal("Template go.mod requires gitlab.com/skyrix-lib/tenant")
	}
	cmd := exec.Command("go", "list", "-deps", "-test", "./...")
	cmd.Dir = l.template
	cmd.Env = workspaceEnv(t, l, l.template, false)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, output)
	}
	for pkg := range strings.FieldsSeq(string(output)) {
		if strings.HasPrefix(pkg, "gitlab.com/skyrix-lib/tenant") {
			t.Fatalf("default Template imports %s", pkg)
		}
	}
}

// TestTemplateHasNoFrameworkFixtures keeps framework development fixtures out
// of the copied Template. Template tests may keep their own data in testdata
// directories, but no testdata directory may carry a framework fixture (a
// directory named like one in integration/testdata, or files wiring the
// external libraries), and no test or test data may reach sibling
// directories outside the Template.
func TestTemplateHasNoFrameworkFixtures(t *testing.T) {
	l := projectLayout(t)
	entries, err := os.ReadDir(l.fixtures)
	if err != nil {
		t.Fatal(err)
	}
	fixtures := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() {
			fixtures[entry.Name()] = true
		}
	}
	if len(fixtures) == 0 {
		t.Fatalf("no framework fixtures found in %s", l.fixtures)
	}
	markers := []string{
		"skyrix-tenant-lib", "skyrix-auth-lib", "gitlab.com/skyrix-lib/tenant",
		"cmd/platformgen", "integration/testdata", `filepath.Abs("..")`,
	}
	err = filepath.WalkDir(l.template, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(l.template, path)
		if entry.IsDir() {
			if rel == "logs" || rel == ".git" {
				return filepath.SkipDir
			}
			if filepath.Base(filepath.Dir(path)) == "testdata" && fixtures[entry.Name()] {
				t.Errorf("Template contains framework fixture %s; it belongs in integration/testdata", rel)
			}
			return nil
		}
		inTestdata := strings.Contains("/"+filepath.ToSlash(rel), "/testdata/")
		if !inTestdata && !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, marker := range markers {
			if strings.Contains(string(data), marker) {
				t.Errorf("%s references %s; framework fixtures belong in integration/", rel, marker)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
