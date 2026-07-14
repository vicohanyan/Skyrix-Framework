package tenantPackage

import (
	"skyrix/internal/engine/tenantPackage/middleware"
)

type Middleware struct {
	Tenant *middleware.TenantMiddleware
	Cors   *middleware.CorsMiddleware
}

func NewMiddlewareBundle(
	tenant *middleware.TenantMiddleware,
	cors *middleware.CorsMiddleware,
) *Middleware {
	return &Middleware{
		Tenant: tenant,
		Cors:   cors,
	}
}
