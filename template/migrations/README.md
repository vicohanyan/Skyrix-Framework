# SQL migrations

The Template ships with an empty migration directory: no SQL files and no
`atlas.sum`. This file is a placeholder that keeps the directory and its
embed pattern valid; only `*.sql` files are embedded as migrations.

- `cobra make migration <name>` creates an empty file for hand-written SQL
  (no Atlas CLI needed); after writing the SQL run `cobra db migrate:hash`.
- Optional: `cobra db migrate:diff <name>` generates a migration from the
  models listed in `tools/schemaloader/main.go` (needs the Atlas CLI).
- The first migration creates `atlas.sum`; later ones verify it before
  writing and then update it.
- Commit the SQL files and `atlas.sum` together.
- Never edit an applied migration. If an unapplied file was changed on
  purpose, review it and run `cobra db migrate:hash` explicitly. `atlas.sum` protects the files; the history of applied
  migrations lives in the database (`cobra db migrate:status`).
