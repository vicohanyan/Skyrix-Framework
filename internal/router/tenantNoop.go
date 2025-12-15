package router

import "net/http"

// NoopTenantMiddleware disables tenant handling while keeping wiring intact.
type NoopTenantMiddleware struct{}

func NewNoopTenantMiddleware() *NoopTenantMiddleware { return &NoopTenantMiddleware{} }

func (m *NoopTenantMiddleware) Wrap(next http.Handler) http.Handler { return next }
