package depcheck

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Embed the real TB so go-list/setup failures still fail this test, but record
// boundary diagnostics to verify intentional violations without a failing suite.
type recordingTB struct {
	testing.TB
	errors []string
}

func (t *recordingTB) Errorf(format string, args ...any) {
	t.errors = append(t.errors, fmt.Sprintf(format, args...))
}

func TestProductionGraphAndImportChains(t *testing.T) {
	// A local module makes the graph deterministic and requires no network or
	// platform services. Both kinds of test files depend on forbidden tooling.
	t.Chdir(t.TempDir())
	t.Setenv("GOWORK", "off")
	files := map[string]string{
		"go.mod":                "module example.com/boundary\n\ngo 1.27\n",
		"core/core.go":          "package core\nimport _ \"example.com/boundary/bridge\"\n",
		"core/core_test.go":     "package core\nimport _ \"example.com/boundary/tooling\"\n",
		"core/external_test.go": "package core_test\nimport _ \"example.com/boundary/tooling\"\n",
		"core/child/child.go":   "package child\nimport _ \"example.com/boundary/forbidden\"\n",
		"bridge/bridge.go":      "package bridge\nimport _ \"example.com/boundary/forbidden\"\n",
		"forbidden/driver.go":   "package forbidden\n",
		"tooling/tooling.go":    "package tooling\n",
	}
	for name, source := range files {
		if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	const root = "example.com/boundary/core"
	const driver = "example.com/boundary/forbidden"
	forbidden := HasPrefix(driver, "example.com/boundary/tooling")

	t.Run("single package and transitive chain", func(t *testing.T) {
		recorded := &recordingTB{TB: t}
		ForbidPackage(recorded, root, forbidden)
		want := []string{root + " imports forbidden package " + driver + " via " + root + " -> example.com/boundary/bridge -> " + driver}
		if !reflect.DeepEqual(recorded.errors, want) {
			t.Fatalf("diagnostics = %v, want %v", recorded.errors, want)
		}
	})
	t.Run("tree includes subpackages and direct imports", func(t *testing.T) {
		recorded := &recordingTB{TB: t}
		Forbid(recorded, root, forbidden)
		if len(recorded.errors) != 2 {
			t.Fatalf("diagnostics = %v, want two production violations", recorded.errors)
		}
		want := root + "/child imports forbidden package " + driver + " via " + root + "/child -> " + driver
		if recorded.errors[1] != want {
			t.Fatalf("diagnostic = %s, want %s", recorded.errors[1], want)
		}
	})
	t.Run("test tooling excluded", func(t *testing.T) {
		Forbid(t, root, HasPrefix("example.com/boundary/tooling"))
	})
	t.Run("production changes are reread", func(t *testing.T) {
		if err := os.WriteFile("bridge/bridge.go", []byte("package bridge\n"), 0644); err != nil {
			t.Fatal(err)
		}
		ForbidPackage(t, root, forbidden)
		// The single-package variant must not pick up core/child's violation.
	})
}

func TestTransitiveWalkHandlesCyclesAndKeepsShortestChain(t *testing.T) {
	packages := map[string]Package{
		"core":      {ImportPath: "core", Imports: []string{"bridge", "forbidden"}},
		"bridge":    {ImportPath: "bridge", Imports: []string{"core", "forbidden"}},
		"forbidden": {ImportPath: "forbidden"},
	}
	chains := violations("core", packages, inTree("core"), func(pkg Package) bool {
		return pkg.ImportPath != "forbidden"
	}, true)
	if len(chains) != 1 || strings.Join(chains[0], " -> ") != "core -> forbidden" {
		t.Fatalf("chains = %v, want shortest chain exactly once", chains)
	}
}
