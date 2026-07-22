package scope

import (
	"context"
	"testing"

	"gorm.io/gorm"
)

type testEngine struct{ main string }

func (e testEngine) Main() string                   { return e.main }
func (testEngine) SetSchema(*gorm.DB, string) error { return nil }

type testMainModel struct{ MainModel }
type testTenantModel struct{ TenantModel }

func TestPluginQualifiesMainAndTenantTables(t *testing.T) {
	tests := []struct {
		name   string
		model  any
		tenant string
		want   string
	}{
		{name: "main model", model: &testMainModel{}, tenant: "tenant_a", want: `"public"."records"`},
		{name: "tenant model", model: &testTenantModel{}, tenant: "tenant_a", want: `"tenant_a"."records"`},
		{name: "tenant fallback", model: &testTenantModel{}, want: `"public"."records"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := WithEngine(context.Background(), testEngine{main: "public"})
			ctx = WithTenant(ctx, test.tenant)
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
