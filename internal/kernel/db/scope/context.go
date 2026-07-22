package scope

import (
	"context"

	"gorm.io/gorm"
)

type ctxKey string

const (
	ctxTenantKey ctxKey = "tenant"
	ctxEngineKey ctxKey = "engine"
)

type Engine interface {
	Main() string
	SetSchema(*gorm.DB, string) error
}

func WithTenant(ctx context.Context, tenant string) context.Context {
	return context.WithValue(ctx, ctxTenantKey, tenant)
}

func WithEngine(ctx context.Context, engine Engine) context.Context {
	return context.WithValue(ctx, ctxEngineKey, engine)
}

func engineFrom(ctx context.Context) Engine {
	engine, _ := ctx.Value(ctxEngineKey).(Engine)
	return engine
}

func TenantFrom(ctx context.Context) string {
	if v := ctx.Value(ctxTenantKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
