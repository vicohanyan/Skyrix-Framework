package config

import (
	"fmt"
	"os"
	"time"

	authservice "gitlab.com/skyrix-lib/framework/extensions/auth/service"
	logging "gitlab.com/skyrix-lib/framework/logger"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env         string `yaml:"APP_ENV" env:"APP_ENV"`
	ServiceName string `yaml:"SERVICE_NAME" env:"SERVICE_NAME"`
	// Version is the application version, recorded with applied migrations.
	// Set it at deploy time, for example APP_VERSION=v1.2.3.
	Version string `yaml:"APP_VERSION" env:"APP_VERSION"`

	Logger      `yaml:"LOGGER" env:"LOGGER"`
	HttpServer  `yaml:"HTTP_SERVER" env:"HTTP_SERVER"`
	Database    `yaml:"DATABASE" env:"DATABASE"`
	Redis       `yaml:"REDIS" env:"REDIS"`
	JWT         `yaml:"JWT" env:"JWT"`
	TenantCache `yaml:"TENANT_CACHE" env:"TENANT_CACHE"`
}

type Logger = logging.Config

type HttpServer struct {
	Address string        `yaml:"APP_ADDRESS" env:"APP_ADDRESS"`
	Port    int           `yaml:"APP_PORT" env:"APP_PORT"`
	Timeout time.Duration `yaml:"APP_REQUEST_TIMEOUT" env:"APP_REQUEST_TIMEOUT" env-default:"5s"`
	// ReadHeaderTimeout bounds reading the request headers (Slowloris
	// protection); zero or negative falls back to 10s.
	ReadHeaderTimeout time.Duration `yaml:"APP_READ_HEADER_TIMEOUT" env:"APP_READ_HEADER_TIMEOUT" env-default:"10s"`
}
type Database struct {
	Host       string `yaml:"DB_HOST" env:"DB_HOST"`
	Port       int    `yaml:"DB_PORT" env:"DB_PORT"`
	User       string `yaml:"DB_USER" env:"DB_USER"`
	Pass       string `yaml:"DB_PASS" env:"DB_PASS"`
	Name       string `yaml:"DB_NAME" env:"DB_NAME"`
	MainSchema string `yaml:"DB_MAIN_SCHEMA" env:"DB_MAIN_SCHEMA"`
}

// Redis is the Redis connection. KeyPrefix is required (no default): it is the
// root of every key this service writes, so services sharing a Redis database
// do not collide. TLS, pool and timeouts keep the go-redis defaults; set them
// in providers.ProvideRedisOptions when a service needs them.
type Redis struct {
	Host      string `yaml:"REDIS_HOST" env:"REDIS_HOST"`
	Port      int    `yaml:"REDIS_PORT" env:"REDIS_PORT"`
	Pass      string `yaml:"REDIS_PASS" env:"REDIS_PASS"`
	Db        int    `yaml:"REDIS_DB" env:"REDIS_DB"`
	KeyPrefix string `yaml:"REDIS_KEY_PREFIX" env:"REDIS_KEY_PREFIX"`
}

// JWT configures token verification (and signing for token issuers).
type JWT = authservice.Config

// TenantCache configures the tenant cache. Its Redis keys live under
// REDIS_KEY_PREFIX in the "tenant" namespace.
type TenantCache struct {
	TTL time.Duration `yaml:"TENANT_CACHE_TTL" env:"TENANT_CACHE_TTL" env-default:"3m"`
}

func LoadConfig() (*Config, error) {
	cfg := &Config{}

	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = "local"
	}

	configPaths := []string{
		fmt.Sprintf("/config/%s.yaml", appEnv),
		fmt.Sprintf("config/%s.yaml", appEnv),
	}

	var loaded bool
	for _, path := range configPaths {
		if _, err := os.Stat(path); err != nil {
			continue
		}

		if err := cleanenv.ReadConfig(path, cfg); err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
		}

		loaded = true
		break
	}

	if !loaded {
		fmt.Printf("Warning: config file for APP_ENV=%s not found, falling back to environment variables\n", appEnv)
	}

	if err := cleanenv.ReadEnv(cfg); err != nil {
		return nil, fmt.Errorf("failed to read environment variables: %w", err)
	}

	return cfg, nil
}
