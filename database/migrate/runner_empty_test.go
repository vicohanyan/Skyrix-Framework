package migrate_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gitlab.com/skyrix-lib/framework/database/migrate"
)

// recordingDriver is a database/sql driver without a database: it records
// every statement, accepts every Exec and answers every query with no rows.
type recordingDriver struct {
	mu         sync.Mutex
	statements []string
}

func (d *recordingDriver) Open(string) (driver.Conn, error) { return &recordingConn{d}, nil }

func (d *recordingDriver) record(query string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.statements = append(d.statements, strings.Join(strings.Fields(query), " "))
}

type recordingConn struct{ d *recordingDriver }

func (c *recordingConn) Prepare(query string) (driver.Stmt, error) {
	return &recordingStmt{c.d, query}, nil
}
func (c *recordingConn) Close() error { return nil }
func (c *recordingConn) Begin() (driver.Tx, error) {
	c.d.record("BEGIN")
	return recordingTx{c.d}, nil
}

type recordingTx struct{ d *recordingDriver }

func (t recordingTx) Commit() error   { t.d.record("COMMIT"); return nil }
func (t recordingTx) Rollback() error { t.d.record("ROLLBACK"); return nil }

type recordingStmt struct {
	d     *recordingDriver
	query string
}

func (s *recordingStmt) Close() error  { return nil }
func (s *recordingStmt) NumInput() int { return -1 }
func (s *recordingStmt) Exec([]driver.Value) (driver.Result, error) {
	s.d.record(s.query)
	return driver.RowsAffected(0), nil
}
func (s *recordingStmt) Query([]driver.Value) (driver.Rows, error) {
	s.d.record(s.query)
	return emptyRows{}, nil
}

type emptyRows struct{}

func (emptyRows) Columns() []string {
	return []string{"version", "name", "checksum", "run", "app_version", "applied_at"}
}
func (emptyRows) Close() error              { return nil }
func (emptyRows) Next([]driver.Value) error { return io.EOF }

var registerRecording sync.Once
var recording = &recordingDriver{}

// TestUpWithoutMigrationsAppliesNothing runs the runner over an empty
// migration source (as the Template ships it: no SQL files, no atlas.sum):
// it applies and records nothing and writes no files.
func TestUpWithoutMigrationsAppliesNothing(t *testing.T) {
	registerRecording.Do(func() { sql.Register("skyrix-recording", recording) })
	recording.mu.Lock()
	recording.statements = nil
	recording.mu.Unlock()

	project := t.TempDir()
	dir := filepath.Join(project, "migrations")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	snapshot := func() []string {
		var files []string
		filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
			files = append(files, path)
			return err
		})
		return files
	}
	before := snapshot()

	db, err := sql.Open("skyrix-recording", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	runner, err := migrate.NewRunner(db, os.DirFS(dir))
	if err != nil {
		t.Fatal(err)
	}
	if n := len(runner.Migrations()); n != 0 {
		t.Fatalf("migrations = %d, want 0", n)
	}
	result, err := runner.Up(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Applied) != 0 {
		t.Fatalf("applied = %v", result.Applied)
	}
	if len(recording.statements) == 0 {
		t.Fatal("the runner did not reach the driver")
	}
	// Only the lock, the history table bookkeeping and the read of applied
	// migrations reach the database: no migration SQL, no transaction, no insert.
	for _, statement := range recording.statements {
		allowed := strings.HasPrefix(statement, "SELECT pg_advisory_lock(") ||
			strings.HasPrefix(statement, "SELECT pg_advisory_unlock(") ||
			strings.HasPrefix(statement, "CREATE TABLE IF NOT EXISTS ") ||
			strings.HasPrefix(statement, "SELECT version, name, checksum")
		if !allowed {
			t.Errorf("unexpected statement %q", statement)
		}
	}
	if after := snapshot(); strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Fatalf("files changed:\nbefore %v\nafter  %v", before, after)
	}
	if _, err := os.Stat(filepath.Join(dir, migrate.AtlasSumFile)); !os.IsNotExist(err) {
		t.Fatalf("atlas.sum was created: %v", err)
	}
}
