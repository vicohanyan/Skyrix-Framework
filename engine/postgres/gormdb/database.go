package gormdb

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gitlab.com/skyrix-lib/framework/engine/scope"
	"gorm.io/gorm"
)

// DB provides context-bound CRUD sessions. Arbitrary SQL needs explicit schema
// qualification or WithinTransaction; WithContext never changes pooled sessions.
type DB interface {
	WithContext(ctx context.Context) *gorm.DB
	Main() string
}

var _ DB = (*Database)(nil)

var (
	ErrScopeMismatch       = errors.New("gormdb: transaction schema scope mismatch")
	ErrTransactionRequired = errors.New("gormdb: managed transaction required")
	ErrInvalidSchema       = errors.New("gormdb: schema contains a NUL byte")
)

// Database wraps GORM with schema-qualified CRUD and explicit scoped transactions.
// Register Plugin on the base GORM connection. Do not mutate MainSchema or DB
// while a Database is in use.
type Database struct {
	*gorm.DB
	MainSchema string
}

func NewDatabase(mainDB *gorm.DB, mainSchema string) *Database {
	return &Database{DB: mainDB, MainSchema: mainSchema}
}

// Identifiers are literal PostgreSQL names, not SQL fragments. Preserve case and
// whitespace, including for quoted identifiers. An empty main schema is public.
func quoteIdent(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

func (d *Database) Main() string {
	if d.MainSchema == "" {
		return "public"
	}
	return d.MainSchema
}

type transactionScopeKey struct{}
type transactionScope struct{ main, schema string }

func (s transactionScope) Main() string { return s.main }

func selectedScope(ctx context.Context, main string) (transactionScope, error) {
	schema := scope.SchemaFrom(ctx)
	if schema == "" {
		schema = main
	}
	if strings.ContainsRune(main, 0) || strings.ContainsRune(schema, 0) {
		return transactionScope{}, ErrInvalidSchema
	}
	return transactionScope{main: main, schema: schema}, nil
}

func boundScope(db *gorm.DB) *transactionScope {
	if db.Statement == nil || db.Statement.Context == nil {
		return nil
	}
	s, _ := db.Statement.Context.Value(transactionScopeKey{}).(*transactionScope)
	return s
}

func setLocalSchema(db *gorm.DB, s transactionScope) error {
	path := quoteIdent(s.schema)
	if s.main != s.schema {
		path += ", " + quoteIdent(s.main)
	}
	path += ", public"
	if err := db.Exec("SET LOCAL search_path = " + path).Error; err != nil {
		return fmt.Errorf("gormdb: set local schema: %w", err)
	}
	return nil
}

func (d *Database) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// WithContext binds context and CRUD schema metadata without executing SQL.
// In a managed transaction the effective request and main schema must match the
// transaction scope; a mismatch is returned in the GORM session's Error field.
// Context deadlines may change, but cannot extend the lifetime of BeginTx's ctx.
// Raw/Row/Scan, joins and explicit Table expressions outside a transaction need
// explicit schema-qualified SQL; the plugin does not rewrite arbitrary SQL.
func (d *Database) WithContext(ctx context.Context) *gorm.DB {
	selected, err := selectedScope(ctx, d.Main())
	bound := boundScope(d.DB)
	if err == nil && bound != nil && selected != *bound {
		err = ErrScopeMismatch
	}
	if bound != nil {
		ctx = context.WithValue(ctx, transactionScopeKey{}, bound)
	}
	ctx = withMainSchema(ctx, selected)
	db := d.DB.WithContext(ctx)
	if err != nil {
		db.AddError(err)
	}
	return db
}

// WithinTransaction runs fn on one SQL transaction with a checked SET LOCAL
// before fn. Return nil to commit; errors and panics roll back through GORM.
// No retries are performed: a commit error can have an unknown outcome.
// Nested calls on the callback's tx use GORM savepoints in the SAME transaction;
// their success is provisional until the outer commit. Scope changes are errors.
// Calling the root Database instead is a separate transaction, not propagation.
// An unmanaged GORM transaction is rejected; enter through this method first.
func (d *Database) WithinTransaction(ctx context.Context, fn func(tx *Database) error) error {
	session := d.WithContext(ctx)
	if session.Error != nil {
		return session.Error
	}
	bound := boundScope(d.DB)
	if bound == nil {
		if _, ok := d.DB.Statement.ConnPool.(gorm.TxCommitter); ok {
			return ErrTransactionRequired
		}
	} else if d.DB.DisableNestedTransaction {
		return errors.New("gormdb: nested transactions require GORM savepoints")
	}
	selected, err := selectedScope(ctx, d.Main())
	if err != nil {
		return err
	}
	return session.Transaction(func(tx *gorm.DB) error {
		tx = tx.WithContext(context.WithValue(tx.Statement.Context, transactionScopeKey{}, &selected))
		if err := setLocalSchema(tx, selected); err != nil {
			return err
		}
		return fn(NewDatabase(tx, selected.main))
	})
}
