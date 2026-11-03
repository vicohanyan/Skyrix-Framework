package config

import (
	"bytes"
	"errors"
	"io"
	"os"
	"time"

	authservice "gitlab.com/skyrix-lib/framework/extensions/auth/service"
	logging "gitlab.com/skyrix-lib/framework/logger"

	sections "gitlab.com/skyrix-lib/framework/config"
)

type Config struct {
	source      []byte
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
	Pass       string `yaml:"DB_PASS" env:"DB_PASS" secret:"true"`
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
	Pass      string `yaml:"REDIS_PASS" env:"REDIS_PASS" secret:"true"`
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

type identity struct {
	Env         string `yaml:"APP_ENV" env:"APP_ENV"`
	ServiceName string `yaml:"SERVICE_NAME" env:"SERVICE_NAME"`
	Version     string `yaml:"APP_VERSION" env:"APP_VERSION"`
}

// Sections returns fresh default targets for the console environment reference.
// Keep this explicit list in sync with the sections used by application providers.
// No files, environment variables or infrastructure are accessed here.
func Sections() []sections.Section {
	return []sections.Section{
		{Key: "app", Value: &identity{}, YAMLPath: []string{}, LegacyEnv: true},
		{Key: "logger", Value: &Logger{}, YAMLPath: []string{"LOGGER"}, LegacyEnv: true},
		{Key: "http", Value: &HttpServer{}, YAMLPath: []string{"HTTP_SERVER"}, LegacyEnv: true},
		{Key: "database", Value: &Database{}, YAMLPath: []string{"DATABASE"}, LegacyEnv: true},
		{Key: "redis", Value: &Redis{}, YAMLPath: []string{"REDIS"}, LegacyEnv: true},
		{Key: "auth", Value: &JWT{}, YAMLPath: []string{"JWT"}, LegacyEnv: true},
		{Key: "tenant", Value: &TenantCache{}, YAMLPath: []string{"TENANT_CACHE"}, LegacyEnv: true},
	}
}

// LoadConfig loads application identity and logger only. Infrastructure sections
// are loaded by the providers actually used by a binary's Wire graph.
func LoadConfig() (*Config, error) {
	cfg := &Config{}
	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = "local"
	}
	for _, path := range []string{"/config/" + appEnv + ".yaml", "config/" + appEnv + ".yaml"} {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, errors.New("config: cannot read file")
		}
		cfg.source = data
		break
	}
	identity := identity{}
	if err := cfg.LoadSection("app", []string{}, &identity); err != nil {
		return nil, err
	}
	cfg.Env, cfg.ServiceName, cfg.Version = identity.Env, identity.ServiceName, identity.Version
	if err := cfg.LoadSection("logger", []string{"LOGGER"}, &cfg.Logger); err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadSection preserves legacy YAML paths and explicit env tags while sharing
// the framework's loading, metadata and validation rules.
func (cfg *Config) LoadSection(key string, path []string, target any) error {
	return cfg.LoadSections(sections.Section{Key: key, Value: target, YAMLPath: path, LegacyEnv: true})
}

// LoadSections loads only sections requested by the selected providers.
func (cfg *Config) LoadSections(selected ...sections.Section) error {
	loader, err := sections.New(selected...)
	if err != nil {
		return err
	}
	var reader io.Reader
	if len(cfg.source) > 0 {
		reader = bytes.NewReader(cfg.source)
	}
	return loader.Load(reader, nil)
}
