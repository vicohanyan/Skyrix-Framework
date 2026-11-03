package migrations

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"gitlab.com/skyrix-lib/framework/database/migrate"
)

// The Template ships without SQL migrations: the package compiles and the
// runner sees an empty, valid source.
func TestShippedWithoutMigrations(t *testing.T) {
	loaded, err := migrate.Load(FS)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 0 {
		t.Fatalf("Template ships %d migration(s), want none", len(loaded))
	}
	if err := fstest.TestFS(FS); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"README.md", "embed.go", migrate.AtlasSumFile} {
		if _, err := fs.Stat(FS, name); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("FS exposes %s: %v", name, err)
		}
	}
}

// Only *.sql files at the top level reach the runner.
func TestFSExposesOnlySQLFiles(t *testing.T) {
	filtered := sqlFS{fstest.MapFS{
		"README.md":                         {Data: []byte("placeholder")},
		"embed.go":                          {Data: []byte("package migrations")},
		migrate.AtlasSumFile:                {Data: []byte("h1:...")},
		"20261103000000_create_tasks.sql":   {Data: []byte("CREATE TABLE tasks (id int);")},
		"20261103000001_index.notx.sql":     {Data: []byte("CREATE INDEX CONCURRENTLY i ON tasks (id);")},
		"nested/20261103000002_ignored.sql": {Data: []byte("SELECT 1;")},
	}}
	if err := fstest.TestFS(filtered, "20261103000000_create_tasks.sql", "20261103000001_index.notx.sql"); err != nil {
		t.Fatal(err)
	}
	loaded, err := migrate.Load(filtered)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 || loaded[0].Name != "create_tasks" || !loaded[1].NoTx {
		t.Fatalf("loaded = %+v", loaded)
	}
}
