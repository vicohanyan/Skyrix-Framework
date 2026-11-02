// Package scope carries the data scope of a request — the database schema
// that queries run in — through context.Context.
//
// The package knows nothing about where the schema comes from: an extension
// (for example the tenant middleware) resolves it and stores it with
// WithSchema, and the database layer reads it with SchemaFrom. Models choose
// whether they follow the request schema (Scoped) or always live in the main
// schema (Main).
package scope

// Scope tells the database layer which schema a model's table lives in.
type Scope int

const (
	// Scoped tables live in the request schema (SchemaFrom), falling back
	// to the main schema when the context carries none. It is the default.
	Scoped Scope = iota
	// Main tables always live in the main schema.
	Main
)

// SchemaScoped is implemented by models that declare their Scope. Models
// that do not implement it are Scoped.
type SchemaScoped interface {
	SchemaScope() Scope
}

// MainModel marks an embedding model as Main.
type MainModel struct{}

// SchemaScope returns Main.
func (MainModel) SchemaScope() Scope { return Main }

// ScopedModel marks an embedding model as Scoped.
type ScopedModel struct{}

// SchemaScope returns Scoped.
func (ScopedModel) SchemaScope() Scope { return Scoped }

// TableBasable is implemented by models whose unqualified table name differs
// from the one GORM derives.
type TableBasable interface {
	TableBase() string
}
