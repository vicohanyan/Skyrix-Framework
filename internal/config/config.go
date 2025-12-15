package config

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env         string `yaml:"APP_ENV" env:"APP_ENV"`
	Logger      `yaml:"LOGGER" env:"LOGGER"`
	HttpServer  `yaml:"HTTP_SERVER" env:"HTTP_SERVER"`
	Database    `yaml:"DATABASE" env:"DATABASE"`
	Redis       `yaml:"REDIS" env:"REDIS"`
	JWT         `yaml:"JWT" env:"JWT"`
	Queue       `yaml:"QUEUE" env:"QUEUE"`
	OAuth       `yaml:"OAUTH" env:"OAUTH"`
	TenantCache `yaml:"TENANT_CACHE" env:"TENANT_CACHE"`
}

type Logger struct {
	LogType  string `yaml:"LOG_TYPE" env:"LOG_TYPE" env-default:"text"`         // text, json
	LogLevel string `yaml:"LOG_LEVEL" env:"LOG_LEVEL" env-default:"info"`       // Log level (debug, info, warn, error)
	LogDir   string `yaml:"LOG_DIR" env:"LOG_DIR" env-default:"./logs/"`        // Log directory
	LogFile  string `yaml:"LOG_FILE" env:"LOG_FILE" env-default:"delivery.log"` // Log file name
}

type HttpServer struct {
	Address string        `yaml:"APP_ADDRESS" env:"APP_ADDRESS"`
	Port    int           `yaml:"APP_PORT" env:"APP_PORT"`
	Timeout time.Duration `yaml:"APP_REQUEST_TIMEOUT" env:"APP_REQUEST_TIMEOUT" env-default:"5s"`
}
type Database struct {
	Host       string `yaml:"DB_HOST" env:"DB_HOST"`
	Port       int    `yaml:"DB_PORT" env:"DB_PORT"`
	User       string `yaml:"DB_USER" env:"DB_USER"`
	Pass       string `yaml:"DB_PASS" env:"DB_PASS"`
	Name       string `yaml:"DB_NAME" env:"DB_NAME"`
	MainSchema string `yaml:"DB_MAIN_SCHEMA" env:"DB_MAIN_SCHEMA"`
}

type Redis struct {
	Host string `yaml:"REDIS_HOST" env:"REDIS_HOST"`
	Port int    `yaml:"REDIS_PORT" env:"REDIS_PORT"`
	Pass string `yaml:"REDIS_PASS" env:"REDIS_PASS"`
	Db   int    `yaml:"REDIS_DB" env:"REDIS_DB"`
}

type JWT struct {
	PublicKeyPath  string `yaml:"JWT_PUBLIC_KEY_PATH" env:"JWT_PUBLIC_KEY_PATH"`
	PrivateKeyPath string `yaml:"JWT_PRIVATE_KEY_PATH" env:"JWT_PRIVATE_KEY_PATH"`
	Expiration     int    `yaml:"JWT_EXPIRE" env:"JWT_EXPIRE" env-default:"24"` // Token expiration time in hours
	Algorithm      string `yaml:"JWT_ALGORITHM" env:"JWT_ALGORITHM"`
}

type Queue struct {
	Host             string        `yaml:"queue_host" env:"QUEUE_HOST" env-default:"nats"`
	Port             int           `yaml:"queue_port" env:"QUEUE_PORT" env-default:"4222"`
	ServiceName      string        `yaml:"queue_service_name" env:"QUEUE_SERVICE_NAME" env-default:"delivery-service"`
	RetryTimeout     time.Duration `yaml:"queue_retry_timeout" env:"QUEUE_RETRY_TIMEOUT" env-default:"5s"`
	ReconnectTimeout time.Duration `yaml:"queue_reconnect_timeout" env:"QUEUE_RECONNECT_TIMEOUT" env-default:"2s"`
}

type OAuth struct {
	GoogleClientID    string `yaml:"GOOGLE_CLIENT_ID" env:"OAUTH_GOOGLE_CLIENT_ID"`
	FacebookAppID     string `yaml:"FACEBOOK_APP_ID" env:"OAUTH_FACEBOOK_APP_ID"`
	FacebookAppSecret string `yaml:"FACEBOOK_APP_SECRET" env:"OAUTH_FACEBOOK_APP_SECRET"`
	AppleClientID     string `yaml:"APPLE_CLIENT_ID" env:"OAUTH_APPLE_CLIENT_ID"`
}

type TenantCache struct {
	TTL       time.Duration `yaml:"TENANT_CACHE_TTL" env:"TENANT_CACHE_TTL" env-default:"3m"`
	KeyPrefix string        `yaml:"TENANT_CACHE_KEY_PREFIX" env:"TENANT_CACHE_KEY_PREFIX" env-default:"skyrix-delivery"`
}

func LoadConfig() (*Config, error) {
	cfg := &Config{}

	if err := cleanenv.ReadConfig("/config/local.yaml", cfg); err != nil {
		fmt.Printf("Warning: failed to read config file, falling back to environment variables: %v\n", err)
	}
	if err := cleanenv.ReadEnv(cfg); err != nil {
		return nil, fmt.Errorf("failed to read environment variables: %w", err)
	}
	return cfg, nil
}
