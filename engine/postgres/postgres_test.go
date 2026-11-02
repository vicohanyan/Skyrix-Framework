package postgres_test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"gitlab.com/skyrix-lib/framework/engine/postgres"
	"gitlab.com/skyrix-lib/framework/internal/depcheck"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestValidate(t *testing.T) {
	err := postgres.Config{Port: 70000}.Validate()
	for _, want := range []string{"host", "port", "database"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Validate() = %v, want %s error", err, want)
		}
	}
	if err := (postgres.Config{Host: "db", Port: 5432, Database: "app"}).Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
}

func TestDSN(t *testing.T) {
	cfg := postgres.Config{
		Host: "db", Port: 5432, User: "app", Password: `p a'ss\`, Database: "orders", MainSchema: "core",
	}
	want := `host='db' port=5432 user='app' password='p a\'ss\\' dbname='orders' sslmode='disable' search_path='core'`
	if got := cfg.DSN(); got != want {
		t.Fatalf("DSN() = %s\nwant    %s", got, want)
	}
	parsed, err := pgconn.ParseConfig(cfg.DSN())
	if err != nil {
		t.Fatalf("pgconn.ParseConfig(DSN()) error = %v", err)
	}
	if parsed.Password != `p a'ss\` || parsed.Database != "orders" || parsed.RuntimeParams["search_path"] != "core" {
		t.Fatalf("parsed DSN = %+v", parsed)
	}

	cfg.SSLMode, cfg.MainSchema = "require", ""
	if got := cfg.DSN(); !strings.Contains(got, "sslmode='require'") || strings.Contains(got, "search_path") {
		t.Fatalf("DSN() = %s", got)
	}
}

func TestConfigurePool(t *testing.T) {
	db, err := sql.Open("pgx", postgres.Config{Host: "127.0.0.1", Port: 1, Database: "none"}.DSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	postgres.Config{MaxOpenConns: 3}.ConfigurePool(db)
	if got := db.Stats().MaxOpenConnections; got != 3 {
		t.Fatalf("MaxOpenConnections = %d, want 3", got)
	}
}

func TestComponentStartPingsAndStopCloses(t *testing.T) {
	db, err := sql.Open("pgx", postgres.Config{Host: "127.0.0.1", Port: 1, Database: "none"}.DSN())
	if err != nil {
		t.Fatal(err)
	}
	component := postgres.NewComponent(db)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := component.Start(ctx); err == nil {
		t.Fatal("Start() = nil, want ping error")
	}
	if err := component.Stop(ctx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if err := component.Stop(ctx); err != nil {
		t.Fatalf("second Stop() error = %v", err)
	}
	if err := postgres.NewComponent(nil).Check(ctx); err == nil {
		t.Fatal("Check() with nil db = nil, want error")
	}
}

func TestOpenSQLRejectsInvalidConfig(t *testing.T) {
	if _, err := postgres.OpenSQL(context.Background(), postgres.Config{}); err == nil {
		t.Fatal("OpenSQL(empty config) error = nil")
	}
}

// TestAgainstRealPostgres runs the lifecycle against SKYRIX_TEST_POSTGRES_DSN.
func TestAgainstRealPostgres(t *testing.T) {
	dsn := os.Getenv("SKYRIX_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("SKYRIX_TEST_POSTGRES_DSN is not set")
	}
	parsed, err := pgconn.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg := postgres.Config{
		Host: parsed.Host, Port: int(parsed.Port), User: parsed.User, Password: parsed.Password,
		Database: parsed.Database, SSLMode: "disable",
	}
	ctx := context.Background()

	sqlDB, err := postgres.OpenSQL(ctx, cfg)
	if err != nil {
		t.Fatalf("OpenSQL() error = %v", err)
	}
	checkLifecycle(t, postgres.NewComponent(sqlDB))
}

func checkLifecycle(t *testing.T, component *postgres.Component) {
	t.Helper()
	ctx := context.Background()
	if err := component.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := component.Check(ctx); err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if err := component.Stop(ctx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if err := component.Check(ctx); err == nil {
		t.Fatal("Check() after Stop = nil, want error")
	}
}

// TestPostgresImportsOnlyInfrastructure keeps the connection package free of
// GORM and of its consumers (gormdb, tenant, domains): it may import only the
// standard library, the pgx database/sql driver and the kernel contracts.
func TestPostgresImportsOnlyInfrastructure(t *testing.T) {
	depcheck.CheckPackage(t, "gitlab.com/skyrix-lib/framework/engine/postgres", func(pkg depcheck.Package) bool {
		switch pkg.ImportPath {
		case "github.com/jackc/pgx/v5/stdlib", "gitlab.com/skyrix-lib/framework/kernel":
			return true
		}
		return pkg.Standard
	})
}
