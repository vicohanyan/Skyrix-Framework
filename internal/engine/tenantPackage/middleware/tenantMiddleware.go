package middleware

import (
	"errors"
	"net/http"
	"skyrix/internal/engine/tenantPackage/context"
	"skyrix/internal/engine/tenantPackage/schemaResolver"
	"strings"

	"skyrix/internal/engine"
	"skyrix/internal/logger"
)

type TenantMiddleware struct {
	Log      logger.Interface
	DB       *engine.Database
	Resolver *schemaResolver.SchemaResolver
}

func NewTenantMiddleware(log logger.Interface, db *engine.Database, resolver *schemaResolver.SchemaResolver) *TenantMiddleware {
	return &TenantMiddleware{Log: log, DB: db, Resolver: resolver}
}

func (m *TenantMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		schema, by, err := m.Resolver.ResolveSchema(r)
		if err != nil {
			// fallback to main schema on "soft" errors
			if errors.Is(err, schemaResolver.ErrTenantHeaderMissing) || errors.Is(err, schemaResolver.ErrHostEmpty) || errors.Is(err, schemaResolver.ErrTenantNotFoundHost) {
				schema = m.DB.MainSchema
				by = "default"
			} else {
				schemaResolver.HTTPError(w, err)
				return
			}
		}

		schema = strings.ToLower(strings.TrimSpace(schema))
		if schema == "" || !schemaResolver.ReIdent.MatchString(schema) {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("X-Tenant-Resolved-By", by)
		switch by {
		case "header":
			w.Header().Add("Vary", schemaResolver.DefaultTenantHeader)
		case "domain":
			w.Header().Add("Vary", "Host")
			w.Header().Add("Vary", "X-Forwarded-Host")
		}

		ctx := r.Context()
		ctx = context.WithSchema(ctx, schema)
		ctx = context.WithResolvedBy(ctx, by)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
