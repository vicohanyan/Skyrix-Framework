package scope

import "context"

type ctxKey string

const ctxSchemaKey ctxKey = "schema"

// WithSchema returns a copy of ctx carrying the request schema.
func WithSchema(ctx context.Context, schema string) context.Context {
	return context.WithValue(ctx, ctxSchemaKey, schema)
}

// SchemaFrom returns the request schema stored by WithSchema, or "" when
// ctx carries none.
func SchemaFrom(ctx context.Context) string {
	if v := ctx.Value(ctxSchemaKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
