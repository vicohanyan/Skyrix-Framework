// Package support contains shared implementation helpers for scaffold commands.
package support

import (
	"fmt"
	"go/format"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// Name contains normalized Go package and exported type names.
type Name struct {
	Package string
	Type    string
}

// NormalizeName validates and normalizes a scaffold name.
func NormalizeName(raw string) (Name, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Name{}, fmt.Errorf("name is required")
	}

	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '-' || r == '_'
	})
	if len(parts) == 0 {
		return Name{}, fmt.Errorf("name %q is invalid", raw)
	}

	for _, part := range parts {
		for index, r := range part {
			if r > unicode.MaxASCII || (!unicode.IsLetter(r) && !(index > 0 && unicode.IsDigit(r))) {
				return Name{}, fmt.Errorf("name %q must contain only ASCII letters, digits, '-' or '_'", raw)
			}
		}
	}

	packageName := strings.ToLower(strings.Join(parts, ""))
	if token.Lookup(packageName).IsKeyword() {
		return Name{}, fmt.Errorf("name %q resolves to Go keyword %q", raw, packageName)
	}

	typeParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == strings.ToUpper(part) {
			part = strings.ToLower(part)
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		typeParts = append(typeParts, string(runes))
	}

	return Name{Package: packageName, Type: strings.Join(typeParts, "")}, nil
}

// ProjectRoot resolves a project root and verifies its go.mod.
func ProjectRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve project root: %w", err)
	}
	if _, err := os.Stat(filepath.Join(abs, "go.mod")); err != nil {
		return "", fmt.Errorf("project root %q must contain go.mod: %w", abs, err)
	}
	return abs, nil
}

// ModulePath reads the module path declared by the project's go.mod.
func ModulePath(root string) (string, error) {
	content, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}
	return "", fmt.Errorf("go.mod does not declare a module path")
}

// WriteNewFile formats Go source and creates a file without overwriting it.
func WriteNewFile(path string, content []byte) error {
	if strings.HasSuffix(path, ".go") {
		formatted, err := format.Source(content)
		if err != nil {
			return fmt.Errorf("format generated %s: %w", path, err)
		}
		content = formatted
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", path, err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("file already exists: %s", path)
		}
		return fmt.Errorf("create %s: %w", path, err)
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}
	return nil
}
