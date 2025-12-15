package scope

import "context"

type ctxKey string

const ctxTenantKey ctxKey = "tenant"

func WithTenant(ctx context.Context, tenant string) context.Context {
	return context.WithValue(ctx, ctxTenantKey, tenant)
}

func TenantFrom(ctx context.Context) string {
	if v := ctx.Value(ctxTenantKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
