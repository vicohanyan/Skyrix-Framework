package atlas

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"gitlab.com/skyrix-lib/framework/database/migrate"
)

// SumFile is the Atlas checksum file of the migration directory. SQL
// migrations and atlas.sum are committed together; the migration runner
// ignores the file and never needs it.
const SumFile = migrate.AtlasSumFile

// HashCommand is the explicit console command that rewrites atlas.sum.
// Generators never rewrite a checksum that is missing or wrong: the user
// reviews the SQL files and runs it deliberately. It needs no Atlas CLI;
// "atlas migrate hash --dir file://migrations" produces the same file.
const HashCommand = "db migrate:hash"

var (
	// ErrSumMissing reports SQL migrations without atlas.sum.
	ErrSumMissing = errors.New("checksum file is missing")
	// ErrSumMismatch reports an atlas.sum that does not match the SQL migrations.
	ErrSumMismatch = errors.New("checksum file does not match the SQL migrations")
)

// Workspace is a private copy of the project's migration directory (its SQL
// files and atlas.sum). Generators let Atlas write only into the copy and
// publish the result into the project in one step, so a failed or empty
// generation leaves the project untouched.
type Workspace struct {
	root    string // absolute project root
	tmp     string // temporary root; the copy lives in tmp/migrations
	files   []string
	sum     []byte
	hasSum  bool
	publish func(temp, path string, exists bool) error
}

// OpenWorkspace copies the migration directory of the project at root into
// a temporary directory. A missing directory is an empty one. Close removes
// the copy.
func OpenWorkspace(root string) (*Workspace, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("migration project root: %w", err)
	}
	if err := rejectSymlink(filepath.Join(absolute, MigrationsDir)); err != nil {
		return nil, err
	}
	files, err := MigrationFiles(absolute)
	if err != nil {
		return nil, err
	}
	sum, hasSum, err := readOptional(filepath.Join(absolute, MigrationsDir, SumFile))
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "skyrix-migrations-*")
	if err != nil {
		return nil, fmt.Errorf("create migration workspace: %w", err)
	}
	w := &Workspace{root: absolute, tmp: tmp, files: files, sum: sum, hasSum: hasSum, publish: publishFile}
	if err := w.copyIn(); err != nil {
		return nil, errors.Join(err, w.Close())
	}
	return w, nil
}

func (w *Workspace) copyIn() error {
	if err := os.MkdirAll(w.Dir(), 0o755); err != nil {
		return fmt.Errorf("create migration workspace: %w", err)
	}
	for _, name := range w.files {
		source := filepath.Join(w.root, MigrationsDir, name)
		if err := rejectSymlink(source); err != nil {
			return err
		}
		data, err := os.ReadFile(source)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(w.Dir(), name), data, 0o644); err != nil {
			return fmt.Errorf("copy migration %s: %w", name, err)
		}
	}
	if w.hasSum {
		if err := os.WriteFile(filepath.Join(w.Dir(), SumFile), w.sum, 0o644); err != nil {
			return fmt.Errorf("copy %s: %w", SumFile, err)
		}
	}
	return nil
}

// Dir is the absolute path of the copied migration directory.
func (w *Workspace) Dir() string { return filepath.Join(w.tmp, MigrationsDir) }

// Files returns the names of the SQL migrations that were copied.
func (w *Workspace) Files() []string { return slices.Clone(w.files) }

// Close removes the temporary copy.
func (w *Workspace) Close() error {
	if err := os.RemoveAll(w.tmp); err != nil {
		return fmt.Errorf("remove migration workspace: %w", err)
	}
	return nil
}

// Verify checks the existing atlas.sum before anything is generated. An
// empty directory without atlas.sum is valid (the first generation creates
// it). SQL migrations without atlas.sum, or an atlas.sum that differs from
// the one computed for them (see Sum), are errors: the checksum is never
// rewritten silently.
func (w *Workspace) Verify() error {
	if !w.hasSum {
		if len(w.files) == 0 {
			return nil
		}
		return fmt.Errorf("%w: %s contains %d SQL migration(s) but no %s; review them and run %q explicitly, then commit the SQL files and %s together",
			ErrSumMissing, MigrationsDir, len(w.files), SumFile, HashCommand, SumFile)
	}
	computed, err := w.Sum()
	if err != nil {
		return fmt.Errorf("verify %s/%s: %w", MigrationsDir, SumFile, err)
	}
	if !bytes.Equal(computed, w.sum) {
		return fmt.Errorf("%w: %s/%s differs from the checksum of the SQL files (a migration was edited, added or removed by hand); review the changes and run %q explicitly. The checksum does not replace the history of applied migrations in the database",
			ErrSumMismatch, MigrationsDir, SumFile, HashCommand)
	}
	return nil
}

// Sum computes atlas.sum for the SQL files currently in the copy.
func (w *Workspace) Sum() ([]byte, error) { return Sum(w.Dir()) }

// Rehash recomputes atlas.sum for the project's SQL migrations as they are
// and replaces the file atomically; SQL files are never modified. It reports
// whether atlas.sum changed. A directory without SQL migrations and without
// atlas.sum is left as it is. It fails, writing nothing, when a file name
// does not follow the migration format or the directory changed meanwhile.
func (w *Workspace) Rehash() (changed bool, result error) {
	if len(w.files) == 0 && !w.hasSum {
		return false, nil
	}
	if err := ValidateFiles(w.files); err != nil {
		return false, fmt.Errorf("unsupported migration file names; nothing was written: %w", err)
	}
	sum, err := w.Sum()
	if err != nil {
		return false, err
	}
	if w.hasSum && bytes.Equal(sum, w.sum) {
		return false, nil
	}
	dir := filepath.Join(w.root, MigrationsDir)
	if err := w.checkUnchanged(dir); err != nil {
		return false, err
	}
	temp, err := writeTemp(dir, sum, 0o644)
	if err != nil {
		return false, err
	}
	defer os.Remove(temp)
	if err := w.publish(temp, filepath.Join(dir, SumFile), w.hasSum); err != nil {
		return false, fmt.Errorf("write %s/%s: %w", MigrationsDir, SumFile, err)
	}
	return true, nil
}

// Add writes a new migration file into the copy.
func (w *Workspace) Add(name string, data []byte) error {
	if name != filepath.Base(name) || filepath.Ext(name) != ".sql" {
		return fmt.Errorf("invalid migration file name %q", name)
	}
	file, err := os.OpenFile(filepath.Join(w.Dir(), name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("stage migration %s: %w", name, err)
	}
	_, writeErr := file.Write(data)
	return errors.Join(writeErr, file.Close())
}

// Created returns the SQL files of the copy that are not in the project.
func (w *Workspace) Created() ([]string, error) {
	after, err := MigrationFiles(w.tmp)
	if err != nil {
		return nil, err
	}
	return NewFiles(w.files, after), nil
}

// Publish copies the created migrations and sum into the project. It first
// checks that the project directory still matches the copied state, creates
// the files exclusively and replaces atlas.sum atomically as the last step.
// On failure it removes the files it created (and the directory, if it
// created it), leaving atlas.sum and every other file as they were.
func (w *Workspace) Publish(created []string, sum []byte) (result error) {
	if len(created) == 0 {
		return errors.New("no migrations to publish")
	}
	dir := filepath.Join(w.root, MigrationsDir)
	if err := w.checkUnchanged(dir); err != nil {
		return err
	}

	var (
		createdDirs []string
		written     []string
		temps       []string
	)
	defer func() {
		for _, temp := range temps {
			os.Remove(temp)
		}
		if result == nil {
			return
		}
		for i := len(written) - 1; i >= 0; i-- {
			if err := os.Remove(written[i]); err != nil && !os.IsNotExist(err) {
				result = errors.Join(result, fmt.Errorf("remove %s: %w", written[i], err))
			}
		}
		for i := len(createdDirs) - 1; i >= 0; i-- {
			os.Remove(createdDirs[i])
		}
	}()

	var missing []string
	for current := dir; current != w.root; current = filepath.Dir(current) {
		if _, err := os.Lstat(current); os.IsNotExist(err) {
			missing = append(missing, current)
		} else if err != nil {
			return err
		} else {
			break
		}
	}
	for i := len(missing) - 1; i >= 0; i-- {
		if err := os.Mkdir(missing[i], 0o755); err != nil {
			return fmt.Errorf("create migrations directory: %w", err)
		}
		createdDirs = append(createdDirs, missing[i])
	}
	stage := func(data []byte) (string, error) {
		temp, err := writeTemp(dir, data, 0o644)
		if err == nil {
			temps = append(temps, temp)
		}
		return temp, err
	}
	for _, name := range created {
		data, err := os.ReadFile(filepath.Join(w.Dir(), name))
		if err != nil {
			return fmt.Errorf("read generated migration %s: %w", name, err)
		}
		temp, err := stage(data)
		if err != nil {
			return err
		}
		path := filepath.Join(dir, name)
		if err := w.publish(temp, path, false); err != nil {
			return fmt.Errorf("write %s/%s: %w", MigrationsDir, name, err)
		}
		written = append(written, path)
	}
	temp, err := stage(sum)
	if err != nil {
		return err
	}
	// atlas.sum is replaced last and atomically, so a failure before or
	// during this step leaves the original checksum in place.
	if err := w.publish(temp, filepath.Join(dir, SumFile), w.hasSum); err != nil {
		return fmt.Errorf("write %s/%s: %w", MigrationsDir, SumFile, err)
	}
	return nil
}

// checkUnchanged verifies that the project directory still has the SQL file
// names and atlas.sum that were copied.
func (w *Workspace) checkUnchanged(dir string) error {
	if err := rejectSymlink(dir); err != nil {
		return err
	}
	current, err := MigrationFiles(w.root)
	if err != nil {
		return err
	}
	currentSum, currentHasSum, err := readOptional(filepath.Join(dir, SumFile))
	if err != nil {
		return err
	}
	if !slices.Equal(current, w.files) || currentHasSum != w.hasSum || !bytes.Equal(currentSum, w.sum) {
		return fmt.Errorf("%s changed during generation; nothing was written", MigrationsDir)
	}
	return nil
}

// publishFile creates new files exclusively and replaces existing ones atomically.
func publishFile(temp, path string, exists bool) error {
	if !exists {
		return os.Link(temp, path)
	}
	return os.Rename(temp, path)
}

func writeTemp(dir string, data []byte, mode os.FileMode) (string, error) {
	file, err := os.CreateTemp(dir, ".skyrix-migration-*")
	if err != nil {
		return "", err
	}
	name := file.Name()
	if err = file.Chmod(mode); err == nil {
		_, err = file.Write(data)
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}

func readOptional(path string) ([]byte, bool, error) {
	if err := rejectSymlink(path); err != nil {
		return nil, false, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", path, err)
	}
	return data, true, nil
}

func rejectSymlink(path string) error {
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("migration path is a symlink: %s", current)
		}
		if filepath.Dir(current) == current {
			return nil
		}
	}
}
