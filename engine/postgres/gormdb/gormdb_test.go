package gormdb_test

import (
	"context"
	"os"
	"testing"
	"time"

	"gitlab.com/skyrix-lib/framework/engine/postgres"
	"gitlab.com/skyrix-lib/framework/engine/postgres/gormdb"

	"github.com/jackc/pgx/v5/pgconn"
)

// Open does not contact the server; the component's Start does.
func TestOpenIsLazyAndStartPings(t *testing.T) {
	db, err := gormdb.Open(postgres.Config{Host: "127.0.0.1", Port: 1, Database: "none", MaxOpenConns: 3})
	if err != nil {
		t.Fatalf("Open() error = %v, want lazy open", err)
	}
	sqlDB, _ := db.DB()
	if got := sqlDB.Stats().MaxOpenConnections; got != 3 {
		t.Fatalf("MaxOpenConnections = %d, want 3", got)
	}
	component, err := gormdb.NewComponent(db)
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
	if _, err := gormdb.Open(postgres.Config{}); err == nil {
		t.Fatal("Open(empty config) error = nil")
	}
	if _, err := gormdb.NewComponent(nil); err == nil {
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
	db, err := gormdb.Open(postgres.Config{
		Host: parsed.Host, Port: int(parsed.Port), User: parsed.User, Password: parsed.Password,
		Database: parsed.Database, SSLMode: "disable",
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	component, err := gormdb.NewComponent(db)
	if err != nil {
		t.Fatal(err)
	}
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
}
