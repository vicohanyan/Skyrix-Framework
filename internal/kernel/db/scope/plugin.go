package scope

import (
	"fmt"
	"skyrix/internal/engine"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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

func quoteIdent(s string) string {
	out := `"`
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			out += `""`
		} else {
			out += string(s[i])
		}
	}
	return out + `"`
}

func (p *Plugin) before(db *gorm.DB) {
	if db.Statement.TableExpr != nil {
		return
	}

	ctx := db.Statement.Context
	e := engine.EngineFrom(ctx)
	if e == nil {
		return
	}

	scope := Tenant
	if s, ok := db.Statement.Model.(SchemaScoped); ok && s != nil {
		scope = s.SchemaScope()
	}

	var base string
	if tb, ok := db.Statement.Model.(TableBasable); ok && tb != nil {
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
	if scope == Main {
		schema = e.Main()
	} else {
		if t := TenantFrom(ctx); t != "" {
			schema = t
		} else {
			schema = e.Main()
		}
	}

	full := fmt.Sprintf("%s.%s", quoteIdent(schema), quoteIdent(base))
	db.Statement.TableExpr = &clause.Expr{SQL: full}
}
