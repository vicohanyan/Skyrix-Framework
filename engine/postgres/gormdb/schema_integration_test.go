package gormdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gitlab.com/skyrix-lib/framework/engine/scope"
	postgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type integrationRecord struct {
	ID    int
	Value string
}

func (integrationRecord) TableName() string { return "records" }

type integrationMainRecord struct {
	scope.MainModel
	ID    int
	Value string
}

func (integrationMainRecord) TableName() string { return "records" }

// SKYRIX_TEST_POSTGRES_DSN must point at a DEDICATED test database with permission
// to create/drop schemas. Never infer a DSN from application configuration.
func TestTransactionSchemaIsolationAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("SKYRIX_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("dedicated SKYRIX_TEST_POSTGRES_DSN is not set")
	}
	raw, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := raw.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	pool.SetMaxOpenConns(2)
	pool.SetMaxIdleConns(2)
	if err := raw.Use(&Plugin{}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	prefix := fmt.Sprintf("uow_%d", time.Now().UnixNano())
	main, tenantA, tenantB := prefix+"_Main", prefix+"_A", prefix+"_B"
	for _, name := range []string{main, tenantA, tenantB} {
		if err := raw.WithContext(ctx).Exec("CREATE SCHEMA " + quoteIdent(name)).Error; err != nil {
			t.Fatal(err)
		}
		name := name
		t.Cleanup(func() { _ = raw.Exec("DROP SCHEMA " + quoteIdent(name) + " CASCADE").Error })
		for _, table := range []string{"records", "details"} {
			if err := raw.WithContext(ctx).Exec("CREATE TABLE " + quoteIdent(name) + "." + quoteIdent(table) + " (id integer PRIMARY KEY, value text NOT NULL)").Error; err != nil {
				t.Fatal(err)
			}
		}
		if err := raw.WithContext(ctx).Exec("INSERT INTO "+quoteIdent(name)+".records VALUES (1, ?)", name).Error; err != nil {
			t.Fatal(err)
		}
		if err := raw.WithContext(ctx).Exec("INSERT INTO "+quoteIdent(name)+".details VALUES (1, ?)", name).Error; err != nil {
			t.Fatal(err)
		}
	}
	db := NewDatabase(raw, main)
	// Hold both pool connections while checking their baseline, forcing two
	// distinct connections; inspect both again after concurrent transactions.
	checkPaths := func(want string) string {
		t.Helper()
		connections := make([]*sql.Conn, 0, 2)
		defer func() {
			for _, conn := range connections {
				_ = conn.Close()
			}
		}()
		var baseline string
		for i := 0; i < 2; i++ {
			conn, err := pool.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			connections = append(connections, conn)
			var path string
			if err := conn.QueryRowContext(ctx, "SHOW search_path").Scan(&path); err != nil {
				t.Fatal(err)
			}
			if i == 0 {
				baseline = path
			}
			if path != baseline {
				t.Fatalf("pool paths differ: %q / %q", baseline, path)
			}
			if want != "" && path != want {
				t.Fatalf("search_path leaked: got %q want %q", path, want)
			}
		}
		return baseline
	}
	baseline := checkPaths("")
	stop := errors.New("last repository failed")
	for _, rollback := range []bool{false, true} {
		ready := make(chan struct{}, 2)
		release := make(chan struct{})
		results := make(chan error, 2)
		var wg sync.WaitGroup
		for _, schema := range []string{tenantA, tenantB} {
			schema := schema
			wg.Add(1)
			go func() {
				defer wg.Done()
				txCtx := scope.WithSchema(ctx, schema)
				results <- db.WithinTransaction(txCtx, func(tx *Database) error {
					// Execute Raw/join BEFORE any repository WithContext could install schema.
					var value string
					if err := tx.DB.Raw("SELECT r.value FROM records r JOIN details d ON r.id=d.id WHERE r.id=1").Scan(&value).Error; err != nil {
						return err
					}
					if value != schema {
						return fmt.Errorf("join selected %q instead of %q", value, schema)
					}
					if err := tx.DB.Raw("SELECT value FROM records WHERE id=1").Row().Scan(&value); err != nil {
						return err
					}
					if value != schema {
						return fmt.Errorf("Row selected %q", value)
					}
					var scoped integrationRecord
					if err := tx.WithContext(txCtx).First(&scoped, 1).Error; err != nil {
						return err
					}
					var global integrationMainRecord
					if err := tx.WithContext(txCtx).First(&global, 1).Error; err != nil {
						return err
					}
					if scoped.Value != schema || global.Value != main {
						return fmt.Errorf("CRUD routing: scoped=%q main=%q", scoped.Value, global.Value)
					}
					// Separate repository-style wrappers share the same SQL transaction.
					one, two := NewDatabase(tx.DB, main), NewDatabase(tx.DB, main)
					if err := one.WithContext(txCtx).Exec("INSERT INTO records VALUES (2, 'first')").Error; err != nil {
						return err
					}
					if err := two.WithContext(txCtx).Exec("INSERT INTO details VALUES (2, 'last')").Error; err != nil {
						return err
					}
					ready <- struct{}{}
					select {
					case <-release:
					case <-ctx.Done():
						return ctx.Err()
					}
					if rollback {
						return two.WithContext(txCtx).Exec("INSERT INTO details VALUES (1, 'duplicate')").Error
					}
					return nil
				})
			}()
		}
		// Do not deadlock if a callback failed before reaching the barrier.
		for i := 0; i < 2; i++ {
			select {
			case <-ready:
			case err := <-results:
				t.Fatalf("callback before barrier: %v", err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		}
		close(release)
		wg.Wait()
		for i := 0; i < 2; i++ {
			err := <-results
			if rollback {
				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
					t.Fatalf("last repository error: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		}
		checkPaths(baseline)
		for _, schema := range []string{tenantA, tenantB} {
			var count int64
			if err := raw.Table(quoteIdent(schema) + ".records").Where("id=2").Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if !rollback && count != 1 || rollback && count != 0 {
				t.Fatalf("rollback=%v count=%d", rollback, count)
			}
			for _, table := range []string{"records", "details"} {
				if err := raw.Exec("DELETE FROM " + quoteIdent(schema) + "." + table + " WHERE id=2").Error; err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	// Same-scope nested work rolls back to a savepoint while outer work commits.
	txCtx := scope.WithSchema(ctx, tenantA)
	if err := db.WithinTransaction(txCtx, func(tx *Database) error {
		err := tx.WithinTransaction(txCtx, func(inner *Database) error {
			if err := inner.DB.Exec("INSERT INTO records VALUES (3, 'nested')").Error; err != nil {
				return err
			}
			return stop
		})
		if !errors.Is(err, stop) {
			return fmt.Errorf("nested result: %w", err)
		}
		return tx.DB.Exec("INSERT INTO records VALUES (4, 'outer')").Error
	}); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := raw.Table(quoteIdent(tenantA) + ".records").Where("id=3").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("savepoint: count=%d err=%v", count, err)
	}
	// Panic must roll back and propagate; the connection must not retain scope.
	func() {
		defer func() {
			if recovered := recover(); recovered != stop {
				t.Fatalf("panic=%v", recovered)
			}
		}()
		_ = db.WithinTransaction(txCtx, func(tx *Database) error {
			if err := tx.DB.Exec("INSERT INTO records VALUES (6, 'panic')").Error; err != nil {
				t.Fatal(err)
			}
			panic(stop)
		})
	}()
	if err := raw.Table(quoteIdent(tenantA) + ".records").Where("id=6").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("panic rollback: count=%d err=%v", count, err)
	}
	checkPaths(baseline)
	var fallback integrationRecord
	if err := db.WithContext(ctx).First(&fallback, 1).Error; err != nil || fallback.Value != main {
		t.Fatalf("fallback=%q err=%v", fallback.Value, err)
	}
	// Cancellation after a write must not commit even if the callback returns nil.
	cancelCtx, cancelTx := context.WithCancel(txCtx)
	err = db.WithinTransaction(cancelCtx, func(tx *Database) error {
		if err := tx.DB.Exec("INSERT INTO records VALUES (5, 'cancel')").Error; err != nil {
			return err
		}
		cancelTx()
		return nil
	})
	if err == nil {
		t.Fatal("canceled transaction committed")
	}
	if err := raw.Table(quoteIdent(tenantA) + ".records").Where("id=5").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("cancel: count=%d err=%v", count, err)
	}
	// Cancellation can discard a connection; compare all surviving/new connections.
	checkPaths(baseline)
}
