package migrate

import (
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestLoadSortsMigrationsAndIgnoresAtlasSum(t *testing.T) {
	source := fstest.MapFS{
		"20260102000000_add_index.notx.sql": {Data: []byte("CREATE INDEX CONCURRENTLY i ON t (c);")},
		"20260101000000_create_t.sql":       {Data: []byte("CREATE TABLE t (c int);")},
		"atlas.sum":                         {Data: []byte("h1:...")},
		"nested/readme.txt":                 {Data: []byte("ignored directory")},
	}
	migrations, err := Load(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 2 {
		t.Fatalf("loaded %d migrations, want 2", len(migrations))
	}
	first, second := migrations[0], migrations[1]
	if first.Version != "20260101000000" || first.Name != "create_t" || first.NoTx {
		t.Fatalf("first migration = %+v", first)
	}
	if second.Version != "20260102000000" || second.Name != "add_index" || !second.NoTx {
		t.Fatalf("second migration = %+v", second)
	}
	if len(first.Checksum) != 64 || first.Checksum == second.Checksum {
		t.Fatalf("checksums = %q, %q", first.Checksum, second.Checksum)
	}
}

func TestLoadRejectsInvalidSources(t *testing.T) {
	tests := map[string]struct {
		source fstest.MapFS
		want   string
	}{
		"invalid name": {
			source: fstest.MapFS{"001_create.sql": {Data: []byte("SELECT 1;")}},
			want:   "must be named",
		},
		"uppercase name": {
			source: fstest.MapFS{"20260101000000_Create.sql": {Data: []byte("SELECT 1;")}},
			want:   "must be named",
		},
		"invalid timestamp": {
			source: fstest.MapFS{"20261399000000_create.sql": {Data: []byte("SELECT 1;")}},
			want:   "YYYYMMDDHHMMSS",
		},
		"down migration": {
			source: fstest.MapFS{"20260101000000_create.down.sql": {Data: []byte("SELECT 1;")}},
			want:   "must be named",
		},
		"duplicate version": {
			source: fstest.MapFS{
				"20260101000000_a.sql":      {Data: []byte("SELECT 1;")},
				"20260101000000_b.notx.sql": {Data: []byte("SELECT 1;")},
			},
			want: "duplicate migration version 20260101000000",
		},
		"empty file": {
			source: fstest.MapFS{"20260101000000_empty.sql": {Data: []byte(" \n")}},
			want:   "is empty",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Load(test.source)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestFileName(t *testing.T) {
	version := NewVersion(time.Date(2026, 11, 2, 15, 4, 5, 0, time.FixedZone("UTC+4", 4*3600)))
	if version != "20261102110405" {
		t.Fatalf("NewVersion() = %q, want UTC 20261102110405", version)
	}
	name, err := FileName(version, "add_users", true)
	if err != nil || name != "20261102110405_add_users.notx.sql" {
		t.Fatalf("FileName() = %q, %v", name, err)
	}
	if _, err := FileName(version, "Add-Users", false); err == nil {
		t.Fatal("FileName() accepted an invalid name")
	}
}

func TestWithTableValidatesIdentifiers(t *testing.T) {
	for _, valid := range []string{"migrations", "platform.skyrix_migrations"} {
		if _, err := quoteTable(valid); err != nil {
			t.Fatalf("quoteTable(%q) error = %v", valid, err)
		}
	}
	for _, invalid := range []string{"", "a.b.c", `bad"name`, "1table"} {
		if _, err := quoteTable(invalid); err == nil {
			t.Fatalf("quoteTable(%q) error = nil", invalid)
		}
	}
}
