package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
)

// DefaultTable is the default name of the table that records applied migrations.
const DefaultTable = "skyrix_migrations"

var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`)

// Dialect contains the database-specific parts of the runner.
//
// Lock and Unlock run on the connection that executes the migrations, so the
// lock may be session-scoped. The SQL statements receive the already quoted
// table name; placeholders follow the driver's syntax.
type Dialect interface {
	Lock(ctx context.Context, conn *sql.Conn, table string) error
	Unlock(ctx context.Context, conn *sql.Conn, table string) error
	TableExists(ctx context.Context, conn *sql.Conn, table string) (bool, error)
	CreateTableSQL(table string) string
	SelectAppliedSQL(table string) string
	InsertAppliedSQL(table string) string
}

// Postgres is the PostgreSQL dialect. It serialises runners with a
// session-level pg_advisory_lock derived from the table name.
type Postgres struct{}

func (Postgres) Lock(ctx context.Context, conn *sql.Conn, table string) error {
	_, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", advisoryLockKey(table))
	return err
}

func (Postgres) Unlock(ctx context.Context, conn *sql.Conn, table string) error {
	_, err := conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", advisoryLockKey(table))
	return err
}

func (Postgres) TableExists(ctx context.Context, conn *sql.Conn, table string) (bool, error) {
	var exists bool
	err := conn.QueryRowContext(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&exists)
	return exists, err
}

func (Postgres) CreateTableSQL(table string) string {
	return `CREATE TABLE IF NOT EXISTS ` + table + ` (
	version     text PRIMARY KEY,
	name        text NOT NULL,
	checksum    text NOT NULL,
	run         integer NOT NULL,
	app_version text NULL,
	applied_at  timestamptz NOT NULL DEFAULT now()
)`
}

func (Postgres) SelectAppliedSQL(table string) string {
	return `SELECT version, name, checksum, run, app_version, applied_at FROM ` + table + ` ORDER BY version`
}

func (Postgres) InsertAppliedSQL(table string) string {
	return `INSERT INTO ` + table + ` (version, name, checksum, run, app_version, applied_at) VALUES ($1, $2, $3, $4, $5, now())`
}

// quoteTable validates a table name of the form "table" or "schema.table" and
// returns it with each part quoted.
func quoteTable(name string) (string, error) {
	parts := strings.Split(name, ".")
	if len(parts) > 2 {
		return "", fmt.Errorf("migration table %q must be <table> or <schema>.<table>", name)
	}
	quoted := make([]string, len(parts))
	for index, part := range parts {
		if !identifierPattern.MatchString(part) {
			return "", fmt.Errorf("migration table %q contains an invalid identifier %q", name, part)
		}
		quoted[index] = `"` + part + `"`
	}
	return strings.Join(quoted, "."), nil
}

func advisoryLockKey(table string) int64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte("skyrix-migrate:" + table))
	return int64(hash.Sum64())
}
