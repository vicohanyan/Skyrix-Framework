// Package context stores tenant resolution details in context.Context.
//
// The resolved schema itself is stored with engine/scope (scope.WithSchema)
// so that the database layer can read it without depending on the tenant
// extension.
package context

import "context"

type ctxKey string

const ctxByKey ctxKey = "tenant_resolved_by"

// WithResolvedBy records how the tenant was resolved ("header", "domain" or
// "default").
func WithResolvedBy(ctx context.Context, by string) context.Context {
	return context.WithValue(ctx, ctxByKey, by)
}

// ResolvedByFrom returns the value stored by WithResolvedBy, or "".
func ResolvedByFrom(ctx context.Context) string {
	if v := ctx.Value(ctxByKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
