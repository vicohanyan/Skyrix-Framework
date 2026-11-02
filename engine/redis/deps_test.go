package redis_test

import (
	"strings"
	"testing"

	"gitlab.com/skyrix-lib/framework/internal/depcheck"
)

// TestRedisImportsOnlyInfrastructure keeps the Redis connection package free
// of its consumers (auth, tenant, jobs, ...): it may import only the standard
// library, go-redis, the kernel contracts and the logger.
func TestRedisImportsOnlyInfrastructure(t *testing.T) {
	depcheck.Check(t, "gitlab.com/skyrix-lib/framework/engine/redis", func(pkg depcheck.Package) bool {
		switch {
		case pkg.Standard:
			return true
		case pkg.ImportPath == "github.com/redis/go-redis/v9",
			strings.HasPrefix(pkg.ImportPath, "github.com/redis/go-redis/v9/"):
			return true
		case pkg.ImportPath == "gitlab.com/skyrix-lib/framework/kernel",
			pkg.ImportPath == "gitlab.com/skyrix-lib/framework/logger":
			return true
		}
		return false
	})
}
