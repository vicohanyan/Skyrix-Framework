package migrate_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"gitlab.com/skyrix-lib/framework/database/migrate"
)

// The integration tests need a PostgreSQL database. Set
// SKYRIX_TEST_POSTGRES_DSN to a keyword/value DSN, for example
// "host=localhost port=5432 user=postgres dbname=postgres sslmode=disable".
// Every test runs in its own schema.
const dsnEnv = "SKYRIX_TEST_POSTGRES_DSN"

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv(dsnEnv)
	if dsn == "" {
		t.Skipf("%s is not set", dsnEnv)
	}

	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })

	schema := fmt.Sprintf("migrate_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })

	db, err := sql.Open("pgx", dsn+" search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newRunner(t *testing.T, db *sql.DB, source fstest.MapFS, options ...migrate.Option) *migrate.Runner {
	t.Helper()
	runner, err := migrate.NewRunner(db, source, options...)
	if err != nil {
		t.Fatal(err)
	}
	return runner
}

func sqlFile(content string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(content)} }

func countRows(t *testing.T, db *sql.DB, query string) int {
	t.Helper()
	var count int
	if err := db.QueryRow(query).Scan(&count); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return count
}

func TestUpAppliesPendingMigrationsInOneRun(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	source := fstest.MapFS{
		"20260101000000_create_items.sql": sqlFile("CREATE TABLE items (id int PRIMARY KEY);\nCREATE TABLE tags (id int);"),
		"20260102000000_seed_items.sql":   sqlFile("INSERT INTO items VALUES (1), (2);"),
	}

	result, err := newRunner(t, db, source, migrate.WithAppVersion("v1.0.0")).Up(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.Run != 1 || len(result.Applied) != 2 {
		t.Fatalf("first Up = %+v", result)
	}
	if n := countRows(t, db, "SELECT count(*) FROM items"); n != 2 {
		t.Fatalf("items = %d", n)
	}

	result, err = newRunner(t, db, source).Up(ctx)
	if err != nil || len(result.Applied) != 0 {
		t.Fatalf("repeated Up = %+v, %v", result, err)
	}

	source["20260103000000_add_column.sql"] = sqlFile("ALTER TABLE items ADD COLUMN title text;")
	result, err = newRunner(t, db, source).Up(ctx)
	if err != nil || result.Run != 2 || len(result.Applied) != 1 {
		t.Fatalf("second run Up = %+v, %v", result, err)
	}

	status, err := newRunner(t, db, source).Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		run        int
		appVersion string
	}{{1, "v1.0.0"}, {1, "v1.0.0"}, {2, ""}}
	for index, item := range status.Migrations {
		if !item.Applied || item.Record.Run != want[index].run || item.Record.AppVersion != want[index].appVersion || item.Record.AppliedAt.IsZero() {
			t.Fatalf("status[%d] = %+v", index, item)
		}
	}
	var nullVersions int
	if err := db.QueryRow("SELECT count(*) FROM skyrix_migrations WHERE app_version IS NULL").Scan(&nullVersions); err != nil || nullVersions != 1 {
		t.Fatalf("NULL app_version rows = %d, %v", nullVersions, err)
	}
}

func TestUpStopsAtFailedMigrationAndKeepsEarlierOnes(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	source := fstest.MapFS{
		"20260101000000_create_items.sql": sqlFile("CREATE TABLE items (id int);"),
		"20260102000000_broken.sql":       sqlFile("CREATE TABLE half_done (id int);\nSELECT * FROM missing_table;"),
		"20260103000000_never_runs.sql":   sqlFile("CREATE TABLE never (id int);"),
	}

	_, err := newRunner(t, db, source).Up(ctx)
	if err == nil || !strings.Contains(err.Error(), "20260102000000_broken") {
		t.Fatalf("Up() error = %v, want failed migration id", err)
	}
	status, err := newRunner(t, db, source).Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Migrations[0].Applied || status.Migrations[1].Applied || status.Migrations[2].Applied {
		t.Fatalf("status after failure = %+v", status.Migrations)
	}
	if n := countRows(t, db, "SELECT count(*) FROM pg_tables WHERE tablename IN ('half_done', 'never') AND schemaname = current_schema()"); n != 0 {
		t.Fatalf("failed migration left %d tables", n)
	}

	source["20260102000000_broken.sql"] = sqlFile("CREATE TABLE half_done (id int);")
	result, err := newRunner(t, db, source).Up(ctx)
	if err != nil || result.Run != 2 || len(result.Applied) != 2 {
		t.Fatalf("Up after fix = %+v, %v", result, err)
	}
}

func TestNoTxMigrationRunsOutsideTransaction(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	source := fstest.MapFS{
		"20260101000000_create_items.sql":     sqlFile("CREATE TABLE items (id int);"),
		"20260102000000_index_items.notx.sql": sqlFile("CREATE INDEX CONCURRENTLY items_id_idx ON items (id);"),
	}
	if _, err := newRunner(t, db, source).Up(ctx); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, db, "SELECT count(*) FROM pg_indexes WHERE indexname = 'items_id_idx' AND schemaname = current_schema()"); n != 1 {
		t.Fatalf("concurrent index count = %d", n)
	}

	transactional := fstest.MapFS{
		"20260101000000_create_items.sql": sqlFile("CREATE TABLE items2 (id int);"),
		"20260102000000_index_items.sql":  sqlFile("CREATE INDEX CONCURRENTLY items2_id_idx ON items2 (id);"),
	}
	if _, err := newRunner(t, openTestDB(t), transactional).Up(ctx); err == nil {
		t.Fatal("CREATE INDEX CONCURRENTLY in a transactional migration succeeded")
	}
}

func TestModifiedAndOrphanMigrations(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	source := fstest.MapFS{
		"20260101000000_create_items.sql": sqlFile("CREATE TABLE items (id int);"),
		"20260102000000_create_tags.sql":  sqlFile("CREATE TABLE tags (id int);"),
	}
	if _, err := newRunner(t, db, source).Up(ctx); err != nil {
		t.Fatal(err)
	}

	changed := fstest.MapFS{
		"20260101000000_create_items.sql": sqlFile("CREATE TABLE items (id bigint);"),
		"20260103000000_create_notes.sql": sqlFile("CREATE TABLE notes (id int);"),
	}
	status, err := newRunner(t, db, changed).Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Modified) != 1 || status.Modified[0].Version != "20260101000000" {
		t.Fatalf("modified = %+v", status.Modified)
	}
	if len(status.Orphans) != 1 || status.Orphans[0].Version != "20260102000000" {
		t.Fatalf("orphans = %+v", status.Orphans)
	}

	_, err = newRunner(t, db, changed).Up(ctx)
	var modified *migrate.ModifiedError
	if !errors.As(err, &modified) || !strings.Contains(err.Error(), "20260101000000_create_items") {
		t.Fatalf("Up() error = %v, want ModifiedError", err)
	}
	if n := countRows(t, db, "SELECT count(*) FROM pg_tables WHERE tablename = 'notes' AND schemaname = current_schema()"); n != 0 {
		t.Fatal("Up applied migrations despite modified files")
	}

	result, err := newRunner(t, db, changed, migrate.WithAllowModified(true)).Up(ctx)
	if err != nil || len(result.Applied) != 1 {
		t.Fatalf("Up with allow-modified = %+v, %v", result, err)
	}
}

func TestBaselineMarksMigrationsWithoutRunningThem(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if _, err := db.Exec("CREATE TABLE items (id int)"); err != nil {
		t.Fatal(err)
	}
	source := fstest.MapFS{
		"20260101000000_create_items.sql": sqlFile("CREATE TABLE items (id int);"),
		"20260102000000_create_tags.sql":  sqlFile("CREATE TABLE tags (id int);"),
		"20260103000000_create_notes.sql": sqlFile("CREATE TABLE notes (id int);"),
	}

	if _, err := newRunner(t, db, source).Baseline(ctx, "20260101000001"); err == nil {
		t.Fatal("Baseline accepted a version without a migration file")
	}
	marked, err := newRunner(t, db, source, migrate.WithAppVersion("prod-adopt")).Baseline(ctx, "20260102000000")
	if err != nil || len(marked) != 2 {
		t.Fatalf("Baseline = %+v, %v", marked, err)
	}
	if n := countRows(t, db, "SELECT count(*) FROM pg_tables WHERE tablename = 'tags' AND schemaname = current_schema()"); n != 0 {
		t.Fatal("Baseline executed a migration")
	}

	result, err := newRunner(t, db, source).Up(ctx)
	if err != nil || result.Run != 2 || len(result.Applied) != 1 || result.Applied[0].Name != "create_notes" {
		t.Fatalf("Up after baseline = %+v, %v", result, err)
	}
}

func TestConcurrentUpAppliesEachMigrationOnce(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	source := fstest.MapFS{
		"20260101000000_create_items.sql": sqlFile("CREATE TABLE items (id int);\nSELECT pg_sleep(0.3);"),
		"20260102000000_seed_items.sql":   sqlFile("INSERT INTO items VALUES (1);"),
	}

	const runners = 4
	var wg sync.WaitGroup
	errs := make(chan error, runners)
	for range runners {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runner, err := migrate.NewRunner(db, source)
			if err == nil {
				_, err = runner.Up(ctx)
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Up error = %v", err)
		}
	}
	if n := countRows(t, db, "SELECT count(*) FROM items"); n != 1 {
		t.Fatalf("items = %d, want 1", n)
	}
	if n := countRows(t, db, "SELECT count(DISTINCT run) FROM skyrix_migrations"); n != 1 {
		t.Fatalf("runs = %d, want 1", n)
	}
}

func TestUpStepsAndUpToApplyPartialRuns(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	source := fstest.MapFS{
		"20260101000000_create_a.sql": sqlFile("CREATE TABLE a (id int);"),
		"20260102000000_create_b.sql": sqlFile("CREATE TABLE b (id int);"),
		"20260103000000_create_c.sql": sqlFile("CREATE TABLE c (id int);"),
		"20260104000000_create_d.sql": sqlFile("CREATE TABLE d (id int);"),
	}
	runner := newRunner(t, db, source)

	if _, err := runner.UpSteps(ctx, 0); err == nil {
		t.Fatal("UpSteps(0) error = nil")
	}
	if _, err := runner.UpTo(ctx, "20260102000001"); err == nil {
		t.Fatal("UpTo accepted a version without a migration file")
	}

	result, err := runner.UpSteps(ctx, 1)
	if err != nil || result.Run != 1 || len(result.Applied) != 1 || result.Applied[0].Name != "create_a" {
		t.Fatalf("UpSteps(1) = %+v, %v", result, err)
	}
	result, err = runner.UpTo(ctx, "20260103000000")
	if err != nil || result.Run != 2 || len(result.Applied) != 2 || result.Applied[1].Name != "create_c" {
		t.Fatalf("UpTo = %+v, %v", result, err)
	}
	result, err = runner.UpTo(ctx, "20260102000000")
	if err != nil || len(result.Applied) != 0 {
		t.Fatalf("UpTo of an applied version = %+v, %v", result, err)
	}
	result, err = runner.UpSteps(ctx, 10)
	if err != nil || result.Run != 3 || len(result.Applied) != 1 || result.Applied[0].Name != "create_d" {
		t.Fatalf("UpSteps(10) = %+v, %v", result, err)
	}
}
