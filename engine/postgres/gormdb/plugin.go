package gormdb

import (
	"context"
	"fmt"
	"strings"

	"gitlab.com/skyrix-lib/framework/engine/scope"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// mainSchemer reports the main schema of the session's Database.
type mainSchemer interface {
	Main() string
}

type ctxKey struct{}

// withMainSchema stores the Database that opened the session, so that the
// Plugin knows the main schema.
func withMainSchema(ctx context.Context, m mainSchemer) context.Context {
	return context.WithValue(ctx, ctxKey{}, m)
}

func mainSchemerFrom(ctx context.Context) mainSchemer {
	m, _ := ctx.Value(ctxKey{}).(mainSchemer)
	return m
}

// Plugin qualifies the table of every query, create, update and delete with
// its schema: scope.Main models use the main schema, other models the
// request schema from scope.SchemaFrom (the main schema when there is none).
// It applies only to sessions opened by Database.WithContext.
type Plugin struct{}

func (p *Plugin) Name() string { return "schema-router" }

func (p *Plugin) Initialize(db *gorm.DB) error {
	cb := db.Callback()
	cb.Query().Before("gorm:query").Register("schema-router:query", p.before)
	cb.Create().Before("gorm:create").Register("schema-router:create", p.before)
	cb.Update().Before("gorm:update").Register("schema-router:update", p.before)
	cb.Delete().Before("gorm:delete").Register("schema-router:delete", p.before)
	return nil
}

func (p *Plugin) before(db *gorm.DB) {
	if db.Statement.TableExpr != nil {
		return
	}

	ctx := db.Statement.Context
	e := mainSchemerFrom(ctx)
	if e == nil {
		return
	}

	modelScope := scope.Scoped
	if s, ok := db.Statement.Model.(scope.SchemaScoped); ok && s != nil {
		modelScope = s.SchemaScope()
	}

	var base string
	if tb, ok := db.Statement.Model.(scope.TableBasable); ok && tb != nil {
		base = tb.TableBase()
	} else if db.Statement.Table != "" {
		base = db.Statement.Table
	} else if db.Statement.Schema != nil {
		base = db.Statement.Schema.Table
	}
	if base == "" {
		return
	}
	if strings.Contains(base, ".") {
		return
	}

	var schema string
	if modelScope == scope.Main {
		schema = e.Main()
	} else {
		if s := scope.SchemaFrom(ctx); s != "" {
			schema = s
		} else {
			schema = e.Main()
		}
	}

	full := fmt.Sprintf("%s.%s", quotePluginIdent(schema), quotePluginIdent(base))
	db.Statement.TableExpr = &clause.Expr{SQL: full}
}

// quotePluginIdent preserves literal identifier names, just like SET LOCAL.
func quotePluginIdent(s string) string {
	return quoteIdent(s)
}
