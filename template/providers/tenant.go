package providers

import (
	tenantPackage "gitlab.com/skyrix-lib/framework/extensions/tenant"
	"gitlab.com/skyrix-lib/framework/extensions/tenant/middleware"
	"gitlab.com/skyrix-lib/framework/extensions/tenant/repository"
	"gitlab.com/skyrix-lib/framework/extensions/tenant/schemaResolver"
	"gitlab.com/skyrix-lib/framework/extensions/tenant/service"
	"gitlab.com/skyrix-lib/template/internal/config"

	"github.com/goforj/wire"
)

var TenantProviderSet = wire.NewSet(
	CoreSet,
	MiddlewareSet,
)

// CoreSet repo/service/resolver/options. No HTTP/router here.
var CoreSet = wire.NewSet(
	repository.NewTenantRepository,

	ProvideTenantCacheOpts,
	ProvideTenantHeader,
	ProvideTenantResolveOrder,

	service.NewTenantService,
	schemaResolver.NewSchemaResolver,
)

// MiddlewareSet only HTTP middleware components.
var MiddlewareSet = wire.NewSet(
	middleware.NewTenantMiddleware,
	middleware.NewCorsMiddleware,
	tenantPackage.NewMiddlewareBundle,
)

func ProvideTenantCacheOpts(cfg *config.Config) service.CacheOpts {
	return service.CacheOpts{
		TTL:       cfg.TenantCache.TTL,
		KeyPrefix: cfg.TenantCache.KeyPrefix,
	}
}

func ProvideTenantHeader(_ *config.Config) string {
	return schemaResolver.DefaultTenantHeader
}

func ProvideTenantResolveOrder(_ *config.Config) []string {
	return []string{"header", "domain"}
}
