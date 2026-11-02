// Package depcheck verifies package import boundaries in tests.
package depcheck

import (
	"bufio"
	"bytes"
	"os/exec"
	"sort"
	"strings"
	"testing"
)

// Package is a package reported by go list.
type Package struct {
	ImportPath string
	Standard   bool
	Imports    []string
}

// Rule decides whether a package outside the checked tree may be imported.
type Rule func(Package) bool

// StandardLibraryOnly allows only packages from the Go standard library.
func StandardLibraryOnly(pkg Package) bool { return pkg.Standard }

// Check fails the test when a package under root (root and root/...) reaches
// a package outside that tree that rule does not allow. Each violation reports
// the import chain. The test is skipped when the go command is unavailable.
func Check(t testing.TB, root string, rule Rule) {
	t.Helper()
	check(t, root+"/...", inTree(root), rule, false)
}

// CheckPackage is like Check for the single package path; its subpackages are
// checked as ordinary dependencies.
func CheckPackage(t testing.TB, path string, rule Rule) {
	t.Helper()
	check(t, path, func(candidate string) bool { return candidate == path }, rule, false)
}

// Forbid fails the test when a package under root (root and root/...)
// reaches a forbidden package directly or transitively, through any other
// package, as "go list -deps" sees it. Each violation reports the import
// chain.
func Forbid(t testing.TB, root string, forbidden func(path string) bool) {
	t.Helper()
	check(t, root+"/...", inTree(root), func(pkg Package) bool { return !forbidden(pkg.ImportPath) }, true)
}

// ForbidPackage is like Forbid for the single package path; its subpackages
// are checked as ordinary dependencies.
func ForbidPackage(t testing.TB, path string, forbidden func(path string) bool) {
	t.Helper()
	check(t, path, func(candidate string) bool { return candidate == path },
		func(pkg Package) bool { return !forbidden(pkg.ImportPath) }, true)
}

// HasPrefix returns a forbidden-path predicate matching each prefix as a
// package path or a parent of package paths.
func HasPrefix(prefixes ...string) func(path string) bool {
	return func(path string) bool {
		for _, prefix := range prefixes {
			if path == prefix || strings.HasPrefix(path, prefix+"/") {
				return true
			}
		}
		return false
	}
}

func inTree(root string) func(string) bool {
	return func(path string) bool {
		return path == root || strings.HasPrefix(path, root+"/")
	}
}

// check walks the imports of the packages matched by pattern. Packages
// outside the tree that rule allows are not followed unless transitive is
// set.
func check(t testing.TB, pattern string, inTree func(string) bool, rule Rule, transitive bool) {
	t.Helper()

	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go command is not available in PATH")
	}

	cmd := exec.Command(goBin, "list", "-deps",
		"-f", "{{.ImportPath}}\t{{.Standard}}\t{{join .Imports \" \"}}",
		pattern)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list failed: %v\n%s", err, stderr.String())
	}

	packages := map[string]Package{}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) != 3 {
			t.Fatalf("unexpected go list output line: %q", scanner.Text())
		}
		packages[fields[0]] = Package{
			ImportPath: fields[0],
			Standard:   fields[1] == "true",
			Imports:    strings.Fields(fields[2]),
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read go list output: %v", err)
	}

	var checked []string
	for path := range packages {
		if inTree(path) {
			checked = append(checked, path)
		}
	}
	if len(checked) == 0 {
		t.Fatalf("go list returned no packages for %s", pattern)
	}
	sort.Strings(checked)

	for _, pkg := range checked {
		for _, chain := range violations(pkg, packages, inTree, rule, transitive) {
			t.Errorf("%s imports forbidden package %s via %s",
				pkg, chain[len(chain)-1], strings.Join(chain, " -> "))
		}
	}
}

// violations walks the import graph from start and returns the shortest import
// chain to every reachable package outside the tree that rule rejects.
func violations(start string, packages map[string]Package, inTree func(string) bool, rule Rule, transitive bool) [][]string {
	parent := map[string]string{start: ""}
	queue := []string{start}
	var result [][]string
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, imported := range packages[current].Imports {
			if _, seen := parent[imported]; seen {
				continue
			}
			parent[imported] = current
			if imported == "C" {
				continue
			}
			if inTree(imported) {
				queue = append(queue, imported)
				continue
			}
			if rule(packages[imported]) {
				if transitive {
					queue = append(queue, imported)
				}
				continue
			}
			var chain []string
			for node := imported; node != ""; node = parent[node] {
				chain = append([]string{node}, chain...)
			}
			result = append(result, chain)
		}
	}
	return result
}
