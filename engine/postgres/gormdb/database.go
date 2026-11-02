package gormdb

import (
	"context"
	"fmt"
	"strings"

	"gitlab.com/skyrix-lib/framework/engine/scope"

	"gorm.io/gorm"
)

// DB is the GORM-level database contract: a context-bound session in the
// right schema and the name of the main schema.
type DB interface {
	// WithContext returns a new session bound to the supplied context
	// (search_path/schema adjustments are applied by implementations).
	WithContext(ctx context.Context) *gorm.DB
	// Main returns the name of the primary schema/database.
	Main() string
}

var _ DB = (*Database)(nil)

// Database is a GORM connection with PostgreSQL schema switching: every
// WithContext session runs in the request schema from engine/scope.
type Database struct {
	*gorm.DB
	MainSchema string
}

// NewDatabase wraps a base GORM connection with schema switching helpers.
// Register Plugin on mainDB to qualify table names by scope.
func NewDatabase(mainDB *gorm.DB, mainSchema string) *Database {
	return &Database{DB: mainDB, MainSchema: mainSchema}
}

// quoteIdent escapes an identifier for use in a PostgreSQL search_path list.
func quoteIdent(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, `"`, `""`)
	return `"` + s + `"`
}

// SetSchema applies PostgreSQL search_path for the given request schema.
// If it is empty, it uses only the main schema + public.
// This MUST be called on the *session* (db) that will execute queries.
func (d *Database) SetSchema(db *gorm.DB, schema string) error {
	schema = strings.ToLower(strings.TrimSpace(schema))
	main := strings.ToLower(strings.TrimSpace(d.MainSchema))

	var stmt string
	if schema == "" {
		// Main schema + public
		stmt = fmt.Sprintf(`SET search_path = %s, public`, quoteIdent(main))
	} else {
		// Request schema + main schema + public
		stmt = fmt.Sprintf(`SET search_path = %s, %s, public`, quoteIdent(schema), quoteIdent(main))
	}

	return db.Exec(stmt).Error
}

func (d *Database) Main() string { return d.MainSchema }

func (d *Database) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// WithContext returns a GORM session bound to the given context AND
// automatically sets PostgreSQL search_path based on the request schema in
// context (scope.SchemaFrom).
// - If a schema is present in ctx: search_path = schema, main, public
// - If it is absent:               search_path = main, public
func (d *Database) WithContext(ctx context.Context) *gorm.DB {
	schema := scope.SchemaFrom(ctx)
	ctx = withMainSchema(ctx, d)
	db := d.DB.WithContext(ctx)
	_ = d.SetSchema(db, schema)
	return db
}
