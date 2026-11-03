package providers

// Tenant wiring for an application that enables gitlab.com/skyrix-lib/tenant.
// This file is the working example from README "Tenants"; the Template test
// TestTenantFixture copies it into a temporary project, generates Wire and
// runs it. It is not part of the default Template build.

import (
	authmiddleware "gitlab.com/skyrix-lib/auth/middleware"
	"gitlab.com/skyrix-lib/framework/engine/postgres/gormdb"
	engineredis "gitlab.com/skyrix-lib/framework/engine/redis"
	"gitlab.com/skyrix-lib/framework/logger"
	"gitlab.com/skyrix-lib/template/internal/config"
	tenantcache "gitlab.com/skyrix-lib/tenant/cache"
	tenantmiddleware "gitlab.com/skyrix-lib/tenant/middleware"
	"gitlab.com/skyrix-lib/tenant/repository"
	"gitlab.com/skyrix-lib/tenant/schemaResolver"
	"gitlab.com/skyrix-lib/tenant/service"

	"github.com/goforj/wire"
	"github.com/redis/go-redis/v9"
)

// TenantSet resolves the tenant of API requests and provides
// *tenantmiddleware.TenantMiddleware. It needs ConfigSet, PostgresSet and
// RedisSet (the shared tenant cache).
var TenantSet = wire.NewSet(
	ProvideTenantCacheConfig,
	repository.NewTenantRepository,
	wire.Bind(new(service.Repository), new(*repository.TenantRepository)),
	ProvideTenantCache,
	wire.Bind(new(service.Cache), new(*tenantcache.RedisCache)),
	ProvideTenantService,
	ProvideTenantResolver,
	ProvideTenantMiddleware,
)

// ProvideTenantCacheConfig loads TENANT_CACHE only in graphs that use tenants.
func ProvideTenantCacheConfig(cfg *config.Config) (*config.TenantCache, error) {
	var tenantCache config.TenantCache
	if err := cfg.LoadSection("tenant", []string{"TENANT_CACHE"}, &tenantCache); err != nil {
		return nil, err
	}
	return &tenantCache, nil
}

// ProvideTenantCache returns the Redis tenant cache under
// <REDIS_KEY_PREFIX>:tenant.
func ProvideTenantCache(client *redis.Client, keys engineredis.Keyspace, cfg *config.TenantCache) (*tenantcache.RedisCache, error) {
	return tenantcache.NewRedisCache(client, keys, tenantcache.Options{TTL: cfg.TTL})
}

// ProvideTenantService bounds the in-memory cache by the same TTL.
func ProvideTenantService(log logger.Interface, repo service.Repository, cache service.Cache, cfg *config.TenantCache) *service.TenantService {
	return service.NewTenantService(log, repo, cache, service.Options{TTL: cfg.TTL})
}

// ProvideTenantResolver resolves by the X-Tenant header, then by Host.
func ProvideTenantResolver(svc *service.TenantService) (*schemaResolver.SchemaResolver, error) {
	return schemaResolver.NewSchemaResolver(svc, schemaResolver.Options{
		Header: schemaResolver.DefaultTenantHeader,
		Order:  []string{"header", "domain"},
		// TrustForwardedHost: true, only behind a proxy that overwrites
		// X-Forwarded-Host on every request.
	})
}

// ProvideTenantMiddleware answers requests without a tenant with 400/404.
func ProvideTenantMiddleware(log logger.Interface, db *gormdb.Database, resolver *schemaResolver.SchemaResolver) *tenantmiddleware.TenantMiddleware {
	return tenantmiddleware.NewTenantMiddleware(log, db, resolver, tenantmiddleware.Options{
		// MainFallback: true, to serve requests that carry no tenant
		// identifier in the main schema (an explicit application policy).
	})
}

// ProvideTenantAPIMiddleware replaces ProvideAPIMiddleware in the HTTP graph:
// authentication first, so unauthenticated requests cause no tenant lookup,
// then tenant resolution. Resolving a tenant does not authorize the caller
// for it; check membership in the application.
func ProvideTenantAPIMiddleware(auth *authmiddleware.AuthMiddleware, tenant *tenantmiddleware.TenantMiddleware) APIMiddleware {
	return APIMiddleware{auth.Handle, tenant.Handle}
}
