package postgres_test

import (
	"context"
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

// Open does not contact the server; the component's Start does.
func TestOpenIsLazyAndStartPings(t *testing.T) {
	db, err := postgres.Open(postgres.Config{Host: "127.0.0.1", Port: 1, Database: "none", MaxOpenConns: 3})
	if err != nil {
		t.Fatalf("Open() error = %v, want lazy open", err)
	}
	sqlDB, _ := db.DB()
	if got := sqlDB.Stats().MaxOpenConnections; got != 3 {
		t.Fatalf("MaxOpenConnections = %d, want 3", got)
	}
	component, err := postgres.NewComponent(db)
	if err != nil {
		t.Fatal(err)
	}
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
}

func TestOpenRejectsInvalidConfig(t *testing.T) {
	if _, err := postgres.Open(postgres.Config{}); err == nil {
		t.Fatal("Open(empty config) error = nil")
	}
	if _, err := postgres.OpenSQL(context.Background(), postgres.Config{}); err == nil {
		t.Fatal("OpenSQL(empty config) error = nil")
	}
	if _, err := postgres.NewComponent(nil); err == nil {
		t.Fatal("NewComponent(nil) error = nil")
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
	checkLifecycle(t, postgres.NewSQLComponent(sqlDB))

	db, err := postgres.Open(cfg)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	component, err := postgres.NewComponent(db)
	if err != nil {
		t.Fatal(err)
	}
	checkLifecycle(t, component)
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
// its consumers (engine.Database, tenant, domains).
func TestPostgresImportsOnlyInfrastructure(t *testing.T) {
	depcheck.Check(t, "gitlab.com/skyrix-lib/framework/engine/postgres", func(pkg depcheck.Package) bool {
		switch pkg.ImportPath {
		case "gorm.io/gorm", "gorm.io/driver/postgres", "github.com/jackc/pgx/v5/stdlib",
			"gitlab.com/skyrix-lib/framework/kernel", "gitlab.com/skyrix-lib/framework/logger":
			return true
		}
		return pkg.Standard
	})
}
