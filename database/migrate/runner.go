package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"
)

// Logger is the logging contract used by the runner. The framework logger
// satisfies it.
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
}

// Option configures a Runner.
type Option func(*Runner) error

// WithTable sets the table that records applied migrations
// ("table" or "schema.table"). The default is skyrix_migrations.
func WithTable(name string) Option {
	return func(r *Runner) error {
		quoted, err := quoteTable(name)
		if err != nil {
			return err
		}
		r.table = quoted
		return nil
	}
}

// WithAppVersion records the application version with every migration
// applied by the runner. An empty version is stored as NULL.
func WithAppVersion(version string) Option {
	return func(r *Runner) error {
		r.appVersion = strings.TrimSpace(version)
		return nil
	}
}

// WithLogger sets the runner logger.
func WithLogger(log Logger) Option {
	return func(r *Runner) error {
		if log != nil {
			r.log = log
		}
		return nil
	}
}

// WithDialect sets the database dialect. The default is Postgres.
func WithDialect(dialect Dialect) Option {
	return func(r *Runner) error {
		if dialect == nil {
			return fmt.Errorf("migration dialect is nil")
		}
		r.dialect = dialect
		return nil
	}
}

// WithAllowModified lets Up and Baseline continue when applied migration files
// were changed. Use it only deliberately: the database no longer matches the
// files and the change is never re-applied.
func WithAllowModified(allow bool) Option {
	return func(r *Runner) error {
		r.allowModified = allow
		return nil
	}
}

// Runner applies forward-only migrations from an fs.FS.
type Runner struct {
	db            *sql.DB
	migrations    []Migration
	table         string
	appVersion    string
	dialect       Dialect
	log           Logger
	allowModified bool
}

// NewRunner loads and validates the migrations in source. It does not touch
// the database.
func NewRunner(db *sql.DB, source fs.FS, options ...Option) (*Runner, error) {
	if db == nil {
		return nil, fmt.Errorf("migration database is nil")
	}
	migrations, err := Load(source)
	if err != nil {
		return nil, err
	}
	table, _ := quoteTable(DefaultTable)
	runner := &Runner{
		db:         db,
		migrations: migrations,
		table:      table,
		dialect:    Postgres{},
		log:        nopLogger{},
	}
	for _, option := range options {
		if err := option(runner); err != nil {
			return nil, err
		}
	}
	return runner, nil
}

// Migrations returns the loaded migrations sorted by version.
func (r *Runner) Migrations() []Migration {
	return append([]Migration(nil), r.migrations...)
}

// Record is a migration recorded as applied in the database.
type Record struct {
	Version    string
	Name       string
	Checksum   string
	Run        int
	AppVersion string
	AppliedAt  time.Time
}

// MigrationStatus describes one migration file and its database state.
type MigrationStatus struct {
	Migration
	Applied  bool
	Modified bool
	Record   Record
}

// Status is the state of all migrations.
type Status struct {
	Migrations []MigrationStatus
	// Modified lists applied migrations whose file checksum differs from the
	// checksum recorded in the database.
	Modified []MigrationStatus
	// Orphans lists migrations recorded in the database without a file.
	Orphans []Record
}

// Pending returns the migrations that are not applied.
func (s Status) Pending() []Migration {
	var pending []Migration
	for _, item := range s.Migrations {
		if !item.Applied {
			pending = append(pending, item.Migration)
		}
	}
	return pending
}

// UpResult describes a completed Up call.
type UpResult struct {
	Run     int
	Applied []Migration
}

// ModifiedError reports applied migrations whose files were changed.
type ModifiedError struct {
	Migrations []MigrationStatus
}

func (e *ModifiedError) Error() string {
	ids := make([]string, len(e.Migrations))
	for index, item := range e.Migrations {
		ids[index] = item.ID()
	}
	return fmt.Sprintf("applied migrations were modified: %s; migrations are forward-only, so restore the original files and add a new migration with the change", strings.Join(ids, ", "))
}

// Status reports the state of every migration without changing the database.
func (r *Runner) Status(ctx context.Context) (Status, error) {
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("open migration connection: %w", err)
	}
	defer conn.Close()

	exists, err := r.dialect.TableExists(ctx, conn, r.table)
	if err != nil {
		return Status{}, fmt.Errorf("check migration table %s: %w", r.table, err)
	}
	var records []Record
	if exists {
		if records, err = r.applied(ctx, conn); err != nil {
			return Status{}, err
		}
	}
	return r.status(records), nil
}

// Up applies all pending migrations in version order. All migrations applied
// by one call share a run number. Each migration runs in its own transaction
// together with its record, except .notx migrations, which run without a
// transaction and are recorded after they succeed. Up stops at the first
// failure; migrations applied before it stay applied.
func (r *Runner) Up(ctx context.Context) (UpResult, error) {
	return r.up(ctx, func(pending []Migration) []Migration { return pending })
}

// UpSteps applies at most steps pending migrations, oldest first, as one run.
func (r *Runner) UpSteps(ctx context.Context, steps int) (UpResult, error) {
	if steps < 1 {
		return UpResult{}, fmt.Errorf("migration steps must be at least 1, got %d", steps)
	}
	return r.up(ctx, func(pending []Migration) []Migration {
		if len(pending) > steps {
			return pending[:steps]
		}
		return pending
	})
}

// UpTo applies the pending migrations up to and including version as one run.
// The version must match a migration file.
func (r *Runner) UpTo(ctx context.Context, version string) (UpResult, error) {
	if err := ValidateVersion(version); err != nil {
		return UpResult{}, err
	}
	if !r.hasVersion(version) {
		return UpResult{}, fmt.Errorf("target version %s does not match any migration file", version)
	}
	return r.up(ctx, func(pending []Migration) []Migration {
		var selected []Migration
		for _, migration := range pending {
			if migration.Version <= version {
				selected = append(selected, migration)
			}
		}
		return selected
	})
}

// up applies the pending migrations chosen by selectMigrations as one run.
func (r *Runner) up(ctx context.Context, selectMigrations func([]Migration) []Migration) (UpResult, error) {
	var result UpResult
	err := r.locked(ctx, func(conn *sql.Conn, status Status) error {
		selected := selectMigrations(status.Pending())
		if len(selected) == 0 {
			r.log.Info("no migrations to apply")
			return nil
		}
		r.warnOutOfOrder(status)

		run := r.nextRun(status)
		result.Run = run
		for _, migration := range selected {
			r.log.Info("applying migration", "version", migration.Version, "name", migration.Name, "run", run, "notx", migration.NoTx)
			if err := r.apply(ctx, conn, migration, run); err != nil {
				return fmt.Errorf("apply migration %s: %w", migration.ID(), err)
			}
			result.Applied = append(result.Applied, migration)
		}
		r.log.Info("database migrations applied", "run", run, "count", len(result.Applied))
		return nil
	})
	return result, err
}

// Baseline marks every pending migration up to and including untilVersion as
// applied without executing it. It is used to adopt an existing database whose
// schema already matches those migrations. It returns the marked migrations.
func (r *Runner) Baseline(ctx context.Context, untilVersion string) ([]Migration, error) {
	if err := ValidateVersion(untilVersion); err != nil {
		return nil, err
	}
	if !r.hasVersion(untilVersion) {
		return nil, fmt.Errorf("baseline version %s does not match any migration file", untilVersion)
	}

	var marked []Migration
	err := r.locked(ctx, func(conn *sql.Conn, status Status) error {
		var candidates []Migration
		for _, migration := range status.Pending() {
			if migration.Version <= untilVersion {
				candidates = append(candidates, migration)
			}
		}
		if len(candidates) == 0 {
			r.log.Info("baseline found no pending migrations", "until", untilVersion)
			return nil
		}

		run := r.nextRun(status)
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin baseline transaction: %w", err)
		}
		for _, migration := range candidates {
			if err := r.record(ctx, tx, migration, run); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("record baseline migration %s: %w", migration.ID(), err)
			}
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit baseline: %w", err)
		}
		marked = candidates
		r.log.Info("baseline recorded", "until", untilVersion, "run", run, "count", len(marked))
		return nil
	})
	return marked, err
}

// locked runs fn on a dedicated connection holding the migration lock, after
// creating the table and checking for modified migrations.
func (r *Runner) locked(ctx context.Context, fn func(*sql.Conn, Status) error) (err error) {
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open migration connection: %w", err)
	}
	defer conn.Close()

	if err := r.dialect.Lock(ctx, conn, r.table); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if unlockErr := r.dialect.Unlock(unlockCtx, conn, r.table); unlockErr != nil {
			err = errors.Join(err, fmt.Errorf("release migration lock: %w", unlockErr))
		}
	}()

	if _, err := conn.ExecContext(ctx, r.dialect.CreateTableSQL(r.table)); err != nil {
		return fmt.Errorf("create migration table %s: %w", r.table, err)
	}
	records, err := r.applied(ctx, conn)
	if err != nil {
		return err
	}
	status := r.status(records)
	for _, orphan := range status.Orphans {
		r.log.Warn("applied migration has no file", "version", orphan.Version, "name", orphan.Name)
	}
	if len(status.Modified) > 0 {
		if !r.allowModified {
			return &ModifiedError{Migrations: status.Modified}
		}
		for _, item := range status.Modified {
			r.log.Warn("ignoring modified applied migration", "version", item.Version, "name", item.Name)
		}
	}
	return fn(conn, status)
}

func (r *Runner) apply(ctx context.Context, conn *sql.Conn, migration Migration, run int) error {
	if migration.NoTx {
		if _, err := conn.ExecContext(ctx, migration.SQL); err != nil {
			return err
		}
		if err := r.record(ctx, conn, migration, run); err != nil {
			return fmt.Errorf("record applied migration: %w", err)
		}
		return nil
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := r.record(ctx, tx, migration, run); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record applied migration: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (r *Runner) record(ctx context.Context, db execer, migration Migration, run int) error {
	var appVersion any
	if r.appVersion != "" {
		appVersion = r.appVersion
	}
	_, err := db.ExecContext(ctx, r.dialect.InsertAppliedSQL(r.table),
		migration.Version, migration.Name, migration.Checksum, run, appVersion)
	return err
}

func (r *Runner) applied(ctx context.Context, conn *sql.Conn) ([]Record, error) {
	rows, err := conn.QueryContext(ctx, r.dialect.SelectAppliedSQL(r.table))
	if err != nil {
		return nil, fmt.Errorf("read applied migrations: %w", err)
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var record Record
		var appVersion sql.NullString
		if err := rows.Scan(&record.Version, &record.Name, &record.Checksum, &record.Run, &appVersion, &record.AppliedAt); err != nil {
			return nil, fmt.Errorf("read applied migrations: %w", err)
		}
		record.AppVersion = appVersion.String
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read applied migrations: %w", err)
	}
	return records, nil
}

func (r *Runner) status(records []Record) Status {
	byVersion := make(map[string]Record, len(records))
	for _, record := range records {
		byVersion[record.Version] = record
	}

	var status Status
	known := make(map[string]struct{}, len(r.migrations))
	for _, migration := range r.migrations {
		known[migration.Version] = struct{}{}
		item := MigrationStatus{Migration: migration}
		if record, ok := byVersion[migration.Version]; ok {
			item.Applied = true
			item.Record = record
			item.Modified = record.Checksum != migration.Checksum
		}
		status.Migrations = append(status.Migrations, item)
		if item.Modified {
			status.Modified = append(status.Modified, item)
		}
	}
	for _, record := range records {
		if _, ok := known[record.Version]; !ok {
			status.Orphans = append(status.Orphans, record)
		}
	}
	return status
}

func (r *Runner) nextRun(status Status) int {
	run := 0
	for _, item := range status.Migrations {
		if item.Applied && item.Record.Run > run {
			run = item.Record.Run
		}
	}
	for _, orphan := range status.Orphans {
		if orphan.Run > run {
			run = orphan.Run
		}
	}
	return run + 1
}

func (r *Runner) warnOutOfOrder(status Status) {
	latest := ""
	for _, item := range status.Migrations {
		if item.Applied && item.Version > latest {
			latest = item.Version
		}
	}
	for _, migration := range status.Pending() {
		if migration.Version < latest {
			r.log.Warn("applying migration older than the latest applied one", "version", migration.Version, "name", migration.Name, "latest_applied", latest)
		}
	}
}

func (r *Runner) hasVersion(version string) bool {
	for _, migration := range r.migrations {
		if migration.Version == version {
			return true
		}
	}
	return false
}

type nopLogger struct{}

func (nopLogger) Info(string, ...interface{}) {}
func (nopLogger) Warn(string, ...interface{}) {}
