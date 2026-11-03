// Package migrations embeds the application's SQL migrations.
//
// Files follow the Atlas directory format: <YYYYMMDDHHMMSS>_<name>.sql, or
// <YYYYMMDDHHMMSS>_<name>.notx.sql for migrations that must run outside a
// transaction. Generate schema changes with `cobra db migrate:diff <name>` and
// create hand-written migrations with `cobra make migration <name>`; both
// keep atlas.sum in step with the SQL files.
//
// The Template ships without SQL migrations. The embed pattern also matches
// the README.md placeholder (and this file), so the package compiles with an
// empty directory; FS exposes only the *.sql files to the migration runner.
package migrations

import (
	"embed"
	"io/fs"
	"path"
)

//go:embed *
var files embed.FS

// FS contains the SQL migrations (*.sql in this directory) and nothing else.
var FS fs.FS = sqlFS{files}

type sqlFS struct{ fsys fs.FS }

func (s sqlFS) Open(name string) (fs.File, error) {
	if name == "." {
		file, err := s.fsys.Open(name)
		if err != nil {
			return nil, err
		}
		dir, ok := file.(fs.ReadDirFile)
		if !ok {
			file.Close()
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
		}
		return sqlDir{dir}, nil
	}
	if !fs.ValidPath(name) || path.Dir(name) != "." || path.Ext(name) != ".sql" {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return s.fsys.Open(name)
}

type sqlDir struct{ fs.ReadDirFile }

func (d sqlDir) ReadDir(n int) ([]fs.DirEntry, error) {
	var out []fs.DirEntry
	for {
		entries, err := d.ReadDirFile.ReadDir(n)
		for _, entry := range entries {
			if !entry.IsDir() && path.Ext(entry.Name()) == ".sql" {
				out = append(out, entry)
			}
		}
		if n <= 0 || len(out) > 0 || err != nil {
			return out, err
		}
	}
}
