package kernel

import (
	postgres "gitlab.com/skyrix-lib/framework/internal/postgres"
	frameworkredis "gitlab.com/skyrix-lib/framework/internal/redis"
	logging "gitlab.com/skyrix-lib/framework/logging"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type PostgresOptions struct {
	Host       string
	Port       int
	User       string
	Password   string
	Database   string
	MainSchema string
}

type RedisOptions struct {
	Host     string
	Port     int
	Password string
	DB       int
}

func ProvidePostgres(cfg PostgresOptions, log logging.Interface) (*gorm.DB, func(), error) {
	db, err := postgres.InitPostgres(postgres.Options{
		Host: cfg.Host, Port: cfg.Port, User: cfg.User, Password: cfg.Password,
		Database: cfg.Database, MainSchema: cfg.MainSchema,
	})
	if err != nil {
		log.Error("Unable to initialize postgres database", "error", err)
		return nil, nil, err
	}
	cleanup := func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			log.Info("Closing postgres database connection")
			_ = sqlDB.Close()
		}
	}
	return db, cleanup, nil
}

func ProvideRedis(cfg RedisOptions, log logging.Interface) (*redis.Client, func(), error) {
	client, err := frameworkredis.InitRedis(frameworkredis.Options{
		Host: cfg.Host, Port: cfg.Port, Password: cfg.Password, DB: cfg.DB,
	})
	if err != nil {
		log.Error("Unable to initialize redis client", "error", err)
		return nil, nil, err
	}
	cleanup := func() {
		log.Info("Closing redis client connection")
		_ = client.Close()
	}
	return client, cleanup, nil
}
