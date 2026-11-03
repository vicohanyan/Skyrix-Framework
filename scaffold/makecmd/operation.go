package makecmd

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type change struct {
	path          string
	before, after []byte
	exists        bool
	mode          os.FileMode
}
type operation struct {
	root  string
	files map[string]*change
}

func newOperation(root string) (*operation, error) {
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	return &operation{root: real, files: map[string]*change{}}, nil
}

// All paths are project-relative and no descendant may be a symlink, including
// local stubs and existing registration files. Reads and writes use this check.
func (op *operation) path(relative string) (string, error) {
	clean := filepath.Clean(relative)
	if filepath.IsAbs(relative) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path must stay inside the project: %q", relative)
	}
	current := op.root
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("symlink is unsupported: %s", current)
		}
	}
	return current, nil
}
func (op *operation) optional(relative string) ([]byte, error) {
	path, err := op.path(relative)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return data, err
}
func (op *operation) load(relative string) (*change, error) {
	if ch := op.files[relative]; ch != nil {
		return ch, nil
	}
	path, err := op.path(relative)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read registration %s: %w", relative, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	ch := &change{path: path, before: data, after: append([]byte{}, data...), exists: true, mode: info.Mode().Perm()}
	op.files[relative] = ch
	return ch, nil
}
func (op *operation) create(relative string, data []byte) error {
	if op.files[relative] != nil {
		return fmt.Errorf("duplicate generated path %s", relative)
	}
	path, err := op.path(relative)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("file already exists: %s", relative)
	} else if !os.IsNotExist(err) {
		return err
	}
	// Reject conflicting package-level declarations, including in differently named
	// user files. Methods have their own receiver namespace and are excluded.
	if strings.HasSuffix(relative, ".go") {
		generated, err := parser.ParseFile(token.NewFileSet(), path, data, parser.ParseComments)
		if err != nil {
			return err
		}
		symbols := topNames(generated)
		entries, err := os.ReadDir(filepath.Dir(path))
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			existingPath := filepath.Join(filepath.Dir(relative), entry.Name())
			source, err := op.optional(existingPath)
			if err != nil {
				return err
			}
			file, err := parser.ParseFile(token.NewFileSet(), existingPath, source, parser.ParseComments)
			if err != nil {
				return err
			}
			if file.Name.Name != generated.Name.Name {
				return fmt.Errorf("package conflict in %s", relative)
			}
			for name := range topNames(file) {
				if symbols[name] {
					return fmt.Errorf("symbol %s already exists in %s", name, existingPath)
				}
			}
		}
	}
	op.files[relative] = &change{path: path, after: data, mode: 0644}
	return nil
}
func topNames(file *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil && d.Name.Name != "init" {
				names[d.Name.Name] = true
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					names[s.Name.Name] = true
				case *ast.ValueSpec:
					for _, n := range s.Names {
						if n.Name != "_" {
							names[n.Name] = true
						}
					}
				}
			}
		}
	}
	return names
}

// publishFile uses exclusive creation for new outputs and atomic replacement
// for existing registrations. The callback is injected only by rollback tests.
func publishFile(temp, path string, exists bool) error {
	if !exists {
		return os.Link(temp, path)
	}
	return os.Rename(temp, path)
}
func (op *operation) apply() error { return op.applyWith(publishFile) }
func (op *operation) applyWith(publish func(string, string, bool) error) (result error) {
	keys := make([]string, 0, len(op.files))
	for key := range op.files {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		ch := op.files[key]
		if _, err := op.path(key); err != nil {
			return err
		}
		data, err := os.ReadFile(ch.path)
		if ch.exists {
			if err != nil || !bytes.Equal(data, ch.before) {
				return fmt.Errorf("file changed while preparing: %s", key)
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("generated path now exists: %s", key)
		}
	}
	var dirs, temps []string
	var applied []*change
	defer func() {
		if result != nil {
			for i := len(applied) - 1; i >= 0; i-- {
				ch := applied[i]
				current, err := os.ReadFile(ch.path)
				if err != nil || !bytes.Equal(current, ch.after) {
					result = errors.Join(result, fmt.Errorf("cannot roll back externally changed file %s", ch.path))
					continue
				}
				if ch.exists {
					tmp, err := writeTemp(filepath.Dir(ch.path), ch.before, ch.mode)
					if err == nil {
						err = os.Rename(tmp, ch.path)
						os.Remove(tmp)
					}
					if err != nil {
						result = errors.Join(result, fmt.Errorf("restore %s: %w", ch.path, err))
					}
				} else if err := os.Remove(ch.path); err != nil {
					result = errors.Join(result, err)
				}
			}
		}
		for _, tmp := range temps {
			os.Remove(tmp)
		}
		if result != nil {
			for i := len(dirs) - 1; i >= 0; i-- {
				os.Remove(dirs[i])
			}
		}
	}()
	staged := map[string]string{}
	for _, key := range keys {
		ch := op.files[key]
		if ch.exists && bytes.Equal(ch.before, ch.after) {
			continue
		}
		var missing []string
		for dir := filepath.Dir(ch.path); dir != op.root; dir = filepath.Dir(dir) {
			info, err := os.Stat(dir)
			if err == nil {
				if !info.IsDir() {
					return fmt.Errorf("not a directory: %s", dir)
				}
				break
			}
			if !os.IsNotExist(err) {
				return err
			}
			missing = append(missing, dir)
		}
		for i := len(missing) - 1; i >= 0; i-- {
			if err := os.Mkdir(missing[i], 0755); err != nil {
				return err
			}
			dirs = append(dirs, missing[i])
		}
		tmp, err := writeTemp(filepath.Dir(ch.path), ch.after, ch.mode)
		if err != nil {
			return err
		}
		temps = append(temps, tmp)
		staged[key] = tmp
	}
	for _, key := range keys {
		tmp := staged[key]
		if tmp == "" {
			continue
		}
		ch := op.files[key]
		if ch.exists {
			current, err := os.ReadFile(ch.path)
			if err != nil || !bytes.Equal(current, ch.before) {
				return fmt.Errorf("registration changed while writing: %s", key)
			}
		}
		if err := publish(tmp, ch.path, ch.exists); err != nil {
			return fmt.Errorf("write %s: %w", key, err)
		}
		applied = append(applied, ch)
	}
	return nil
}
func writeTemp(dir string, data []byte, mode os.FileMode) (string, error) {
	f, err := os.CreateTemp(dir, ".skyrix-make-*")
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
