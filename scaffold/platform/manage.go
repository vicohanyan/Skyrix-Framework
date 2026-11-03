package platform

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/skyrix-lib/framework/scaffold"
)

func configImport(dir string) (string, error) {
	for current := dir; ; current = filepath.Dir(current) {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			m, err := scaffold.ModulePath(current)
			return m + "/internal/config", err
		}
		if filepath.Dir(current) == current {
			return "", fmt.Errorf("no go.mod above %s", dir)
		}
	}
}

// Generate updates only the generated adapter file, after all validation.
func Generate(dir, configPackage string) error {
	source, err := os.ReadFile(filepath.Join(dir, "platform.go"))
	if err != nil {
		return err
	}
	_, generated, err := plan(dir, source, configPackage)
	if err != nil {
		return err
	}
	return replace(filepath.Join(dir, "platform_gen.go"), generated)
}
func plan(dir string, source []byte, configPackage string) ([]Driver, []byte, error) {
	drivers, err := parse(source)
	if err != nil {
		return nil, nil, err
	}
	if err = resolve(dir, drivers); err != nil {
		return nil, nil, err
	}
	if configPackage == "" {
		configPackage, err = configImport(dir)
		if err != nil {
			return nil, nil, err
		}
	}
	generated, err := render(drivers, configPackage)
	return drivers, generated, err
}

func List(dir string) ([]Driver, error) {
	source, err := os.ReadFile(filepath.Join(dir, "platform.go"))
	if err != nil {
		return nil, err
	}
	drivers, err := parse(source)
	if err != nil {
		return nil, err
	}
	err = resolve(dir, drivers)
	return drivers, err
}

// Change adds/removes only declarations and generated adapters. The directory
// is the application's providers directory. Missing removals and repeated adds
// are no-ops. User files, config and Wire graphs are never removed or rewritten.
func Change(dir, importPath string, add bool) (bool, error) {
	if !validImport(importPath) {
		return false, fmt.Errorf("invalid driver import path %q", importPath)
	}
	path := filepath.Join(dir, "platform.go")
	source, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	drivers, err := parse(source)
	if err != nil {
		return false, err
	}
	found := false
	for _, d := range drivers {
		if d.ImportPath == importPath {
			found = true
		}
	}
	if found == add {
		return false, nil
	}
	updated := append([]byte{}, source...)
	if add {
		if len(updated) > 0 && updated[len(updated)-1] != '\n' {
			updated = append(updated, '\n')
		}
		updated = append(updated, []byte(prefix+importPath+"\n")...)
	} else {
		var out strings.Builder
		for _, line := range strings.SplitAfter(string(source), "\n") {
			text := strings.TrimSpace(line)
			fields := strings.Fields(strings.TrimPrefix(text, strings.TrimSpace(prefix)))
			if strings.HasPrefix(text, strings.TrimSpace(prefix)) && len(fields) == 1 && fields[0] == importPath {
				continue
			}
			out.WriteString(line)
		}
		updated = []byte(out.String())
	}
	_, generated, err := plan(dir, updated, "")
	if err != nil {
		return false, err
	}
	// Stage both files before publishing. Roll back the declaration if the second
	// rename fails; invalid inputs never reach this point.
	genPath := filepath.Join(dir, "platform_gen.go")
	if err := checkGenerated(genPath); err != nil {
		return false, err
	}

	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		return false, fmt.Errorf("declaration path must be a regular file")
	}
	first, err := stage(path, updated)
	if err != nil {
		return false, err
	}
	defer os.Remove(first)
	second, err := stage(genPath, generated)
	if err != nil {
		return false, err
	}
	defer os.Remove(second)
	if err = os.Rename(first, path); err != nil {
		return false, err
	}
	if err = os.Rename(second, genPath); err != nil {
		if rollback := replace(path, source); rollback != nil {
			return false, fmt.Errorf("%w; rollback: %v", err, rollback)
		}
		return false, err
	}
	return true, nil
}
func stage(path string, data []byte) (string, error) {
	mode := os.FileMode(0644)
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("%s must be a regular file", path)
		}
		mode = info.Mode().Perm()
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".platform-*")
	if err != nil {
		return "", err
	}
	name := f.Name()
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(data)
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}
func replace(path string, data []byte) error {
	if filepath.Base(path) == "platform_gen.go" {
		if err := checkGenerated(path); err != nil {
			return err
		}
	}

	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
		return nil
	}
	name, err := stage(path, data)
	if err != nil {
		return err
	}
	defer os.Remove(name)
	return os.Rename(name, path)
}

// Never take ownership of an existing user file with the generated filename.
func checkGenerated(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s must be a regular generated file", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.HasPrefix(data, []byte(header)) {
		return fmt.Errorf("%s is not owned by platformgen", path)
	}
	return nil
}
