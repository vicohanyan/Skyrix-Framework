package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
)

// ApplicationProvidersDir finds the application providers directory relative to
// root. Both the canonical and legacy layouts are supported.
func ApplicationProvidersDir(root string) (string, error) {
	canonical := filepath.Join("internal", "providers")
	const legacy = "providers"

	var found string
	for _, dir := range []string{canonical, legacy} {
		path := filepath.Join(root, dir)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("inspect application providers directory %q: %w", path, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("application providers path %q is not a directory", path)
		}
		if found != "" {
			return "", fmt.Errorf("ambiguous application providers layout: both %q and %q exist", canonical, legacy)
		}
		found = dir
	}
	if found == "" {
		return "", fmt.Errorf("application providers directory not found: expected %q or %q", canonical, legacy)
	}
	return found, nil
}
