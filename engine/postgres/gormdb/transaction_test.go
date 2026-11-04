package gormdb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"

	"gitlab.com/skyrix-lib/framework/engine/scope"
	postgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// A driver-level recorder tests control flow and error identity, not PostgreSQL
// schema isolation. The latter is covered by the opt-in integration test.
type txRecorder struct {
	events                      []string
	beginErr, setErr, commitErr error
	contextKey                  any
	contextValue                any
	lostContext                 bool
}
type recordingDriver struct{ r *txRecorder }
type recordingConn struct{ r *txRecorder }
type recordingTx struct{ r *txRecorder }
type recordingRows struct{}

func (d recordingDriver) Open(string) (driver.Conn, error) { return &recordingConn{d.r}, nil }
func (c *recordingConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (c *recordingConn) Close() error { return nil }
func (c *recordingConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (r *txRecorder) check(ctx context.Context) {
	if r.contextKey != nil && ctx.Value(r.contextKey) != r.contextValue {
		r.lostContext = true
	}
}
func (c *recordingConn) BeginTx(ctx context.Context, _ driver.TxOptions) (driver.Tx, error) {
	c.r.check(ctx)
	c.r.events = append(c.r.events, "BEGIN")
	if c.r.beginErr != nil {
		return nil, c.r.beginErr
	}
	return &recordingTx{c.r}, nil
}
func (c *recordingConn) ExecContext(ctx context.Context, q string, _ []driver.NamedValue) (driver.Result, error) {
	c.r.check(ctx)
	c.r.events = append(c.r.events, q)
	if strings.HasPrefix(q, "SET LOCAL") && c.r.setErr != nil {
		return nil, c.r.setErr
	}
	return driver.RowsAffected(1), nil
}
func (c *recordingConn) QueryContext(ctx context.Context, q string, _ []driver.NamedValue) (driver.Rows, error) {
	c.r.check(ctx)
	c.r.events = append(c.r.events, q)
	return recordingRows{}, nil
}
func (tx *recordingTx) Commit() error {
	tx.r.events = append(tx.r.events, "COMMIT")
	return tx.r.commitErr
}
func (tx *recordingTx) Rollback() error         { tx.r.events = append(tx.r.events, "ROLLBACK"); return nil }
func (recordingRows) Columns() []string         { return []string{"value"} }
func (recordingRows) Close() error              { return nil }
func (recordingRows) Next([]driver.Value) error { return io.EOF }

var recorderID atomic.Int64

func recordedDB(t *testing.T, r *txRecorder) *Database {
	t.Helper()
	name := fmt.Sprintf("gormdb_recorder_%d", recorderID.Add(1))
	sql.Register(name, recordingDriver{r})
	pool, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: pool}), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Use(&Plugin{}); err != nil {
		t.Fatal(err)
	}
	return NewDatabase(db, "Main Schema")
}
func TestWithinTransactionLifecycle(t *testing.T) {
	sentinel := &testTransactionError{"injected"}
	for _, mode := range []string{"success", "begin", "schema", "commit", "callback", "panic"} {
		t.Run(mode, func(t *testing.T) {
			r := &txRecorder{}
			db := recordedDB(t, r)
			switch mode {
			case "begin":
				r.beginErr = sentinel
			case "schema":
				r.setErr = sentinel
			case "commit":
				r.commitErr = sentinel
			}
			called := false
			var err error
			var recovered any
			func() {
				defer func() { recovered = recover() }()
				err = db.WithinTransaction(context.Background(), func(tx *Database) error {
					called = true
					if tx.Main() != db.Main() {
						t.Fatal("lost main schema")
					}
					switch mode {
					case "callback":
						return fmt.Errorf("operation: %w", sentinel)
					case "panic":
						panic(sentinel)
					}
					return nil
				})
			}()
			if called != (mode != "begin" && mode != "schema") {
				t.Fatalf("callback=%v", called)
			}
			if mode == "panic" {
				if recovered != sentinel {
					t.Fatalf("panic=%v", recovered)
				}
			} else if mode != "success" {
				var typed *testTransactionError
				if !errors.Is(err, sentinel) || !errors.As(err, &typed) {
					t.Fatalf("identity lost: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			want := "COMMIT"
			if mode == "schema" || mode == "callback" || mode == "panic" {
				want = "ROLLBACK"
			}
			if mode == "begin" {
				want = "BEGIN"
			}
			if got := r.events[len(r.events)-1]; got != want {
				t.Fatalf("events=%v", r.events)
			}
		})
	}
}

type testTransactionError struct{ message string }

func (e *testTransactionError) Error() string { return e.message }
func TestTransactionContextScopeAndSavepoints(t *testing.T) {
	type key struct{}
	r := &txRecorder{contextKey: key{}, contextValue: "request"}
	db := recordedDB(t, r)
	ctx := scope.WithSchema(context.WithValue(context.Background(), key{}, "request"), `Tenant"A`)
	if err := db.WithContext(ctx).Error; err != nil {
		t.Fatal(err)
	}
	if len(r.events) != 0 {
		t.Fatalf("WithContext executed SQL: %v", r.events)
	}
	err := db.WithinTransaction(ctx, func(tx *Database) error {
		originalPool := tx.DB.Statement.ConnPool
		// The same transaction survives a repository-style rewrap and WithContext.
		repo := NewDatabase(tx.DB, tx.Main())
		session := repo.WithContext(ctx)
		if session.Statement.ConnPool != originalPool {
			t.Fatal("lost transaction")
		}
		if err := session.Exec("repository one").Error; err != nil {
			return err
		}
		if err := repo.WithContext(ctx).Exec("repository two").Error; err != nil {
			return err
		}
		count := len(r.events)
		changed := scope.WithSchema(ctx, "other")
		if err := tx.WithContext(changed).Exec("must not execute").Error; !errors.Is(err, ErrScopeMismatch) {
			t.Fatalf("scope error=%v", err)
		}
		if err := tx.WithinTransaction(changed, func(*Database) error { t.Fatal("scope callback"); return nil }); !errors.Is(err, ErrScopeMismatch) {
			t.Fatal(err)
		}
		if len(r.events) != count {
			t.Fatal("scope mismatch executed SQL")
		}
		nestedErr := errors.New("nested")
		if err := tx.WithinTransaction(ctx, func(inner *Database) error {
			if inner.DB.Statement.ConnPool != originalPool {
				t.Fatal("nested transaction changed connection")
			}
			return nestedErr
		}); !errors.Is(err, nestedErr) {
			return fmt.Errorf("nested: %w", err)
		}
		return tx.WithContext(ctx).Exec("outer survives").Error
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.lostContext {
		t.Fatal("context lost")
	}
	if r.events[1] != `SET LOCAL search_path = "Tenant""A", "Main Schema", public` {
		t.Fatalf("SQL=%q", r.events[1])
	}
	var begin, save, rollbackTo int
	for _, event := range r.events {
		if event == "BEGIN" {
			begin++
		}
		if strings.HasPrefix(event, "SAVEPOINT ") {
			save++
		}
		if strings.HasPrefix(event, "ROLLBACK TO SAVEPOINT ") {
			rollbackTo++
		}
	}
	if begin != 1 || save != 1 || rollbackTo != 1 {
		t.Fatalf("events=%v", r.events)
	}
}
func TestSchemaValidation(t *testing.T) {
	r := &txRecorder{}
	db := recordedDB(t, r)
	bad := scope.WithSchema(context.Background(), "bad\x00schema")
	if err := db.WithinTransaction(bad, func(*Database) error { t.Fatal("invalid callback"); return nil }); !errors.Is(err, ErrInvalidSchema) {
		t.Fatal(err)
	}
	if len(r.events) != 0 {
		t.Fatal(r.events)
	}
}
func TestCanceledTransactionDoesNotCallCallback(t *testing.T) {
	r := &txRecorder{}
	db := recordedDB(t, r)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := db.WithinTransaction(ctx, func(*Database) error { t.Fatal("canceled callback"); return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestUnmanagedTransactionRejected(t *testing.T) {
	r := &txRecorder{}
	db := recordedDB(t, r)
	raw := db.DB.Begin()
	defer raw.Rollback()
	err := NewDatabase(raw, db.Main()).WithinTransaction(context.Background(), func(*Database) error { t.Fatal("unmanaged callback"); return nil })
	if !errors.Is(err, ErrTransactionRequired) {
		t.Fatal(err)
	}
}

func TestTransactionMainAndScopeCannotChange(t *testing.T) {
	r := &txRecorder{}
	db := recordedDB(t, r)
	ctx := scope.WithSchema(context.Background(), "tenant")
	if err := db.WithinTransaction(ctx, func(tx *Database) error {
		copy := NewDatabase(tx.DB, "other main")
		if err := copy.WithContext(ctx).Error; !errors.Is(err, ErrScopeMismatch) {
			t.Fatalf("main mismatch=%v", err)
		}
		disabled := NewDatabase(tx.DB.Session(&gorm.Session{DisableNestedTransaction: true}), tx.Main())
		if err := disabled.WithinTransaction(ctx, func(*Database) error { t.Fatal("disabled nested callback"); return nil }); err == nil {
			t.Fatal("disabled nesting accepted")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPluginAndSearchPathPreserveLiteralNames(t *testing.T) {
	for _, name := range []string{"MixedCase", " space ", `quoted"name`} {
		t.Run(name, func(t *testing.T) {
			r := &txRecorder{}
			db := recordedDB(t, r)
			ctx := scope.WithSchema(context.Background(), name)
			query := db.WithContext(ctx).Session(&gorm.Session{DryRun: true}).Find(&[]integrationRecord{})
			if query.Error != nil {
				t.Fatal(query.Error)
			}
			if !strings.Contains(query.Statement.SQL.String(), quoteIdent(name)+`."records"`) {
				t.Fatal(query.Statement.SQL.String())
			}
			if len(r.events) != 0 {
				t.Fatal("CRUD session changed pool")
			}
			if err := db.WithinTransaction(ctx, func(tx *Database) error {
				fallback := tx.WithContext(context.Background()).Error
				if !errors.Is(fallback, ErrScopeMismatch) {
					t.Fatalf("missing scope accepted: %v", fallback)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(r.events[1], quoteIdent(name)) {
				t.Fatal(r.events)
			}
		})
	}
}
