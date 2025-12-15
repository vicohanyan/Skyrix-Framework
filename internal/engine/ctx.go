package engine

import (
	"context"

	"gorm.io/gorm"
)

type ctxKey string

const ctxEngineKey ctxKey = "engine"

type Engine interface {
	SetSchema(db *gorm.DB, tenantSchema string) error
	Main() string
}

func WithEngine(ctx context.Context, e Engine) context.Context {
	return context.WithValue(ctx, ctxEngineKey, e)
}

func EngineFrom(ctx context.Context) Engine {
	if v := ctx.Value(ctxEngineKey); v != nil {
		if e, ok := v.(Engine); ok {
			return e
		}
	}
	return nil
}
