package gormdb

import (
	"context"
	"testing"

	"gitlab.com/skyrix-lib/framework/engine/scope"

	"gorm.io/gorm"
)

type testMainModel struct{ scope.MainModel }
type testScopedModel struct{ scope.ScopedModel }

func TestPluginQualifiesMainAndTenantTables(t *testing.T) {
	tests := []struct {
		name   string
		model  any
		schema string
		want   string
	}{
		{name: "main model", model: &testMainModel{}, schema: "tenant_a", want: `"public"."records"`},
		{name: "scoped model", model: &testScopedModel{}, schema: "tenant_a", want: `"tenant_a"."records"`},
		{name: "scoped fallback", model: &testScopedModel{}, want: `"public"."records"`},
		{name: "model without scope", model: &struct{}{}, schema: "tenant_a", want: `"tenant_a"."records"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := withMainSchema(context.Background(), NewDatabase(nil, "public"))
			ctx = scope.WithSchema(ctx, test.schema)
			db := &gorm.DB{Statement: &gorm.Statement{
				Context: ctx,
				Model:   test.model,
				Table:   "records",
			}}

			(&Plugin{}).before(db)

			if db.Statement.TableExpr == nil || db.Statement.TableExpr.SQL != test.want {
				t.Fatalf("qualified table = %#v, want %q", db.Statement.TableExpr, test.want)
			}
		})
	}
}
