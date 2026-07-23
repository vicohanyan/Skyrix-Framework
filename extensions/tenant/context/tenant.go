package context

import "context"

type ctxKey string

const (
	ctxSchemaKey ctxKey = "tenant_schema"
	ctxByKey     ctxKey = "tenant_resolved_by"
)

func WithSchema(ctx context.Context, schema string) context.Context {
	return context.WithValue(ctx, ctxSchemaKey, schema)
}

func SchemaFrom(ctx context.Context) string {
	if v := ctx.Value(ctxSchemaKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func WithResolvedBy(ctx context.Context, by string) context.Context {
	return context.WithValue(ctx, ctxByKey, by)
}

func ResolvedByFrom(ctx context.Context) string {
	if v := ctx.Value(ctxByKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
