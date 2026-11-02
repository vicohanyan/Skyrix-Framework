package providers

import (
	engineredis "gitlab.com/skyrix-lib/framework/engine/redis"
	tenantPackage "gitlab.com/skyrix-lib/framework/extensions/tenant"
	tenantcache "gitlab.com/skyrix-lib/framework/extensions/tenant/cache"
	"gitlab.com/skyrix-lib/framework/extensions/tenant/middleware"
	"gitlab.com/skyrix-lib/framework/extensions/tenant/repository"
	"gitlab.com/skyrix-lib/framework/extensions/tenant/schemaResolver"
	"gitlab.com/skyrix-lib/framework/extensions/tenant/service"
	"gitlab.com/skyrix-lib/template/internal/config"

	"github.com/goforj/wire"
	"github.com/redis/go-redis/v9"
)

var TenantProviderSet = wire.NewSet(
	CoreSet,
	MiddlewareSet,
)

// CoreSet repo/service/resolver/options. No HTTP/router here. The tenant
// cache needs RedisSet in the binary.
var CoreSet = wire.NewSet(
	repository.NewTenantRepository,

	ProvideTenantCache,
	wire.Bind(new(service.Cache), new(*tenantcache.RedisCache)),
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

// ProvideTenantCache returns the Redis tenant cache under the
// <REDIS_KEY_PREFIX>:tenant namespace.
func ProvideTenantCache(client *redis.Client, keys engineredis.Keyspace, cfg *config.Config) (*tenantcache.RedisCache, error) {
	if err := cfg.LoadSection("tenant", []string{"TENANT_CACHE"}, &cfg.TenantCache); err != nil {
		return nil, err
	}
	return tenantcache.NewRedisCache(client, keys, tenantcache.Options{TTL: cfg.TenantCache.TTL})
}

func ProvideTenantHeader(_ *config.Config) string {
	return schemaResolver.DefaultTenantHeader
}

func ProvideTenantResolveOrder(_ *config.Config) []string {
	return []string{"header", "domain"}
}
