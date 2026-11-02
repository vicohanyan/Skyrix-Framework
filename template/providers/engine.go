package providers

import (
	"net"
	"strconv"

	enginepostgres "gitlab.com/skyrix-lib/framework/engine/postgres"
	"gitlab.com/skyrix-lib/framework/engine/postgres/gormdb"
	engineredis "gitlab.com/skyrix-lib/framework/engine/redis"
	"gitlab.com/skyrix-lib/template/internal/config"

	"github.com/goforj/wire"
	"gorm.io/gorm"
)

// PostgresSet opens PostgreSQL through GORM (engine/postgres/gormdb) and
// provides *gorm.DB, *gormdb.Database and the lifecycle
// *enginepostgres.Component. Include it
// only in binaries that use the database, and add the component to the
// binary's runtime before the components that use it.
var PostgresSet = wire.NewSet(
	ProvideDatabaseConfig,
	ProvidePostgresOptions,
	gormdb.Provide,
	gormdb.NewComponent,
	ProvideDatabaseService,
)

// RedisSet connects to Redis and provides *redis.Client (go-redis), the
// application root engineredis.Keyspace (REDIS_KEY_PREFIX) and the lifecycle
// *engineredis.Component. Include it only in binaries that use Redis, and add
// the component to the binary's runtime before the components that use it.
var RedisSet = wire.NewSet(
	ProvideRedisConfig,
	ProvideRedisOptions,
	engineredis.Provide,
	engineredis.ProvideKeyspace,
	engineredis.NewComponent,
)

func ProvideDatabaseConfig(cfg *config.Config) *config.Database {
	return &cfg.Database
}

func ProvidePostgresOptions(cfg *config.Database) enginepostgres.Config {
	return enginepostgres.Config{
		Host: cfg.Host, Port: cfg.Port, User: cfg.User, Password: cfg.Pass,
		Database: cfg.Name, MainSchema: cfg.MainSchema,
	}
}

// ProvideDatabaseService registers the schema-routing plugin and returns the
// schema-aware database.
func ProvideDatabaseService(db *gorm.DB, cfg *config.Config) (*gormdb.Database, error) {
	if err := db.Use(&gormdb.Plugin{}); err != nil {
		return nil, err
	}
	return gormdb.NewDatabase(db, cfg.Database.MainSchema), nil
}

func ProvideRedisConfig(cfg *config.Config) *config.Redis {
	return &cfg.Redis
}

func ProvideRedisOptions(cfg *config.Redis) engineredis.Config {
	return engineredis.Config{
		Addr:      net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Password:  cfg.Pass,
		DB:        cfg.Db,
		KeyPrefix: cfg.KeyPrefix,
	}
}
