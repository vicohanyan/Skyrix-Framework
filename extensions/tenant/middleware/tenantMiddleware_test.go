package middleware_test

import (
	"gitlab.com/skyrix-lib/framework/logger"

	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"gitlab.com/skyrix-lib/framework/engine/postgres/gormdb"
	"gitlab.com/skyrix-lib/framework/engine/scope"
	tenantcontext "gitlab.com/skyrix-lib/framework/extensions/tenant/context"
	"gitlab.com/skyrix-lib/framework/extensions/tenant/entity"
	"gitlab.com/skyrix-lib/framework/extensions/tenant/middleware"
	"gitlab.com/skyrix-lib/framework/extensions/tenant/schemaResolver"
	"gitlab.com/skyrix-lib/framework/extensions/tenant/service"
)

// TestTenantMiddlewareStoresSchemaInScope checks that the resolved schema
// reaches the database layer through engine/scope.
func TestTenantMiddlewareStoresSchemaInScope(t *testing.T) {
	schema := "Tenant_A"
	svc := service.NewTenantService(noopLogger{}, nil, staticCache{
		"acme": {Namespace: "acme", Schema: &schema, IsActive: true},
	})
	resolver := schemaResolver.NewSchemaResolver(svc, "", []string{"header"})
	mw := middleware.NewTenantMiddleware(noopLogger{}, gormdb.NewDatabase(nil, "core"), resolver)

	tests := []struct {
		name       string
		header     string
		wantSchema string
		wantBy     string
	}{
		{name: "header", header: "acme", wantSchema: "tenant_a", wantBy: "header"},
		{name: "fallback to main schema", wantSchema: "core", wantBy: "default"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var gotSchema, gotBy string
			handler := mw.Handle(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				gotSchema = scope.SchemaFrom(r.Context())
				gotBy = tenantcontext.ResolvedByFrom(r.Context())
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if test.header != "" {
				req.Header.Set(schemaResolver.DefaultTenantHeader, test.header)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if gotSchema != test.wantSchema || gotBy != test.wantBy {
				t.Fatalf("schema, resolvedBy = %q, %q; want %q, %q", gotSchema, gotBy, test.wantSchema, test.wantBy)
			}
			if got := rec.Header().Get("X-Tenant-Resolved-By"); got != test.wantBy {
				t.Fatalf("X-Tenant-Resolved-By = %q, want %q", got, test.wantBy)
			}
		})
	}
}

type staticCache map[string]*entity.Tenant

func (c staticCache) ByNamespace(_ context.Context, namespace string) (*entity.Tenant, bool, error) {
	t, ok := c[namespace]
	return t, ok, nil
}

func (staticCache) ByDomain(context.Context, string) (*entity.Tenant, bool, error) {
	return nil, false, nil
}

func (staticCache) Put(context.Context, *entity.Tenant) error { return nil }

type noopLogger struct{}

func (noopLogger) Error(string, ...interface{}) {}
func (noopLogger) Info(string, ...interface{})  {}
func (noopLogger) Debug(string, ...interface{}) {}
func (noopLogger) Warn(string, ...interface{})  {}

func (noopLogger) With(...interface{}) logger.Interface { return noopLogger{} }
