package config

import "time"

// TenantCache configures the tenant caches of gitlab.com/skyrix-lib/tenant:
// TTL bounds both the in-memory and the Redis cache, counted from the
// database load. Its Redis keys live under REDIS_KEY_PREFIX in the "tenant"
// namespace. Also list it in Sections:
//
//	{Key: "tenant", Value: &TenantCache{}, YAMLPath: []string{"TENANT_CACHE"}},
type TenantCache struct {
	TTL time.Duration `yaml:"TENANT_CACHE_TTL" env:"TENANT_CACHE_TTL" env-default:"3m"`
}
