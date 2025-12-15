package engine

import (
	"context"
	"fmt"
	tenantContext "skyrix/internal/engine/tenantPackage/context"
	"strings"

	"gorm.io/gorm"
)

type Database struct {
	*gorm.DB
	MainSchema string
}

// NewDatabaseService wraps a base GORM connection with schema switching helpers.
func NewDatabaseService(mainDB *gorm.DB, mainSchema string) *Database {
	return &Database{DB: mainDB, MainSchema: mainSchema}
}

// quoteIdent escapes an identifier for use in a PostgreSQL search_path list.
func quoteIdent(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, `"`, `""`)
	return `"` + s + `"`
}

// SetSchema applies PostgreSQL search_path for the given tenant.
// If tenant is empty, it uses only the main schema + public.
// This MUST be called on the *session* (db) that will execute queries.
func (d *Database) SetSchema(db *gorm.DB, tenant string) error {
	tenant = strings.ToLower(strings.TrimSpace(tenant))
	main := strings.ToLower(strings.TrimSpace(d.MainSchema))

	var stmt string
	if tenant == "" {
		// Core DB path: main schema + public
		stmt = fmt.Sprintf(`SET search_path = %s, public`, quoteIdent(main))
	} else {
		// Tenant DB path: tenant schema + main schema + public
		stmt = fmt.Sprintf(`SET search_path = %s, %s, public`, quoteIdent(tenant), quoteIdent(main))
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
// automatically sets PostgreSQL search_path based on the tenant in context.
// - If tenant is present in ctx: search_path = tenant, main, public
// - If tenant is absent:         search_path = main, public
// WithContext returns a GORM session bound to the given context AND
// automatically sets PostgreSQL search_path based on the tenant in context.
// - If tenant is present in ctx: search_path = tenant, main, public
// - If tenant is absent:         search_path = main, public
func (d *Database) WithContext(ctx context.Context) *gorm.DB {
	db := d.DB.WithContext(ctx)
	schema := tenantContext.SchemaFrom(ctx)
	_ = d.SetSchema(db, schema)
	return db
}
