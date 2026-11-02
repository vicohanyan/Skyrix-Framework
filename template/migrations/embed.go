// Package migrations embeds the application's SQL migrations.
//
// Files follow the Atlas directory format: <YYYYMMDDHHMMSS>_<name>.sql, or
// <YYYYMMDDHHMMSS>_<name>.notx.sql for migrations that must run outside a
// transaction. Generate schema changes with `cobra db migrate:diff <name>` and
// create hand-written migrations with `cobra make migration <name>`.
package migrations

import "embed"

// FS contains the migration files.
//
//go:embed *.sql
var FS embed.FS
