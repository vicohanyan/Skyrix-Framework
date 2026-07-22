package tenantPackage

import (
	"skyrix/internal/config"
	"skyrix/internal/engine/tenantPackage/middleware"
	"skyrix/internal/engine/tenantPackage/repository"
	"skyrix/internal/engine/tenantPackage/schemaResolver"
	"skyrix/internal/engine/tenantPackage/service"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
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
	NewMiddlewareBundle,
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
