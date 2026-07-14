package config

import (
	"fmt"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env         string `yaml:"APP_ENV" env:"APP_ENV"`
	ServiceName string `yaml:"SERVICE_NAME" env:"SERVICE_NAME"`

	Logger      `yaml:"LOGGER" env:"LOGGER"`
	HttpServer  `yaml:"HTTP_SERVER" env:"HTTP_SERVER"`
	Database    `yaml:"DATABASE" env:"DATABASE"`
	Redis       `yaml:"REDIS" env:"REDIS"`
	JWT         `yaml:"JWT" env:"JWT"`
	Queue       `yaml:"QUEUE" env:"QUEUE"`
	OAuth       `yaml:"OAUTH" env:"OAUTH"`
	TenantCache `yaml:"TENANT_CACHE" env:"TENANT_CACHE"`
	Maps        `yaml:"MAPS" env:"MAPS"`
}

type Logger struct {
	LogOutput string `yaml:"LOG_OUTPUT" env:"LOG_OUTPUT" env-default:"stdout"` // stdout, stderr, file
	LogType   string `yaml:"LOG_TYPE" env:"LOG_TYPE" env-default:"json"`       // text, json
	LogLevel  string `yaml:"LOG_LEVEL" env:"LOG_LEVEL" env-default:"info"`     // debug, info, warn, error
	LogDir    string `yaml:"LOG_DIR" env:"LOG_DIR" env-default:""`             // used only for file output
	LogFile   string `yaml:"LOG_FILE" env:"LOG_FILE" env-default:""`           // used only for file output
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
	Issuer         string `yaml:"JWT_ISSUER" env:"JWT_ISSUER"`
	Audience       string `yaml:"JWT_AUDIENCE" env:"JWT_AUDIENCE"`
}

type Queue struct {
	Enabled          bool          `yaml:"QUEUE_ENABLED"           env:"QUEUE_ENABLED"           env-default:"false"`
	Host             string        `yaml:"QUEUE_HOST"              env:"QUEUE_HOST"              env-default:"nats"`
	Port             int           `yaml:"QUEUE_PORT"              env:"QUEUE_PORT"              env-default:"4222"`
	ServiceName      string        `yaml:"QUEUE_NAME"              env:"QUEUE_NAME"              env-default:"skyrix-framework"`
	User             string        `yaml:"QUEUE_USER"              env:"QUEUE_USER"`
	Pass             string        `yaml:"QUEUE_PASS"              env:"QUEUE_PASS"`
	RetryTimeout     time.Duration `yaml:"QUEUE_RETRY_TIMEOUT"     env:"QUEUE_RETRY_TIMEOUT"     env-default:"5s"`
	ReconnectTimeout time.Duration `yaml:"QUEUE_RECONNECT_TIMEOUT" env:"QUEUE_RECONNECT_TIMEOUT" env-default:"2s"`
	DLQPrefix        string        `yaml:"DLQ_PREFIX"              env:"DLQ_PREFIX"              env-default:"dlq."`
}

func (q *Queue) EventBusEnabled() bool             { return q != nil && q.Enabled }
func (q *Queue) NATSHost() string                  { return q.Host }
func (q *Queue) NATSPort() int                     { return q.Port }
func (q *Queue) NATSUser() string                  { return q.User }
func (q *Queue) NATSPassword() string              { return q.Pass }
func (q *Queue) NATSConnectionName() string        { return q.ServiceName }
func (q *Queue) NATSConnectTimeout() time.Duration { return q.RetryTimeout }
func (q *Queue) NATSReconnectWait() time.Duration  { return q.ReconnectTimeout }
func (q *Queue) NATSDefaultDLQPrefix() string      { return q.DLQPrefix }

type OAuth struct {
	GoogleClientID    string `yaml:"GOOGLE_CLIENT_ID" env:"OAUTH_GOOGLE_CLIENT_ID"`
	FacebookAppID     string `yaml:"FACEBOOK_APP_ID" env:"OAUTH_FACEBOOK_APP_ID"`
	FacebookAppSecret string `yaml:"FACEBOOK_APP_SECRET" env:"OAUTH_FACEBOOK_APP_SECRET"`
	AppleClientID     string `yaml:"APPLE_CLIENT_ID" env:"OAUTH_APPLE_CLIENT_ID"`
}

type TenantCache struct {
	TTL       time.Duration `yaml:"TENANT_CACHE_TTL" env:"TENANT_CACHE_TTL" env-default:"3m"`
	KeyPrefix string        `yaml:"TENANT_CACHE_KEY_PREFIX" env:"TENANT_CACHE_KEY_PREFIX" env-default:"skyrix-framework"`
}

type Maps struct {
	GeocodeProvider          string        `yaml:"MAPS_GEOCODE_PROVIDER" env:"MAPS_GEOCODE_PROVIDER" env-default:"YANDEX"`
	RoutingProvider          string        `yaml:"MAPS_ROUTING_PROVIDER" env:"MAPS_ROUTING_PROVIDER" env-default:"INTERNAL"`
	GeocodeCacheTTL          time.Duration `yaml:"MAPS_GEOCODE_CACHE_TTL" env:"MAPS_GEOCODE_CACHE_TTL" env-default:"720h"`
	RouteCacheTTL            time.Duration `yaml:"MAPS_ROUTE_CACHE_TTL" env:"MAPS_ROUTE_CACHE_TTL" env-default:"720h"`
	InternalRouteCoefficient float64       `yaml:"MAPS_INTERNAL_ROUTE_COEFFICIENT" env:"MAPS_INTERNAL_ROUTE_COEFFICIENT" env-default:"1.35"`
	InternalAverageSpeedKMH  float64       `yaml:"MAPS_INTERNAL_AVERAGE_SPEED_KMH" env:"MAPS_INTERNAL_AVERAGE_SPEED_KMH" env-default:"20"`
	ExternalRequestTimeout   time.Duration `yaml:"MAPS_EXTERNAL_REQUEST_TIMEOUT" env:"MAPS_EXTERNAL_REQUEST_TIMEOUT" env-default:"3s"`

	GoogleAPIKey           string `yaml:"MAPS_GOOGLE_API_KEY" env:"MAPS_GOOGLE_API_KEY"`
	GoogleGeocodingBaseURL string `yaml:"MAPS_GOOGLE_GEOCODING_BASE_URL" env:"MAPS_GOOGLE_GEOCODING_BASE_URL" env-default:"https://maps.googleapis.com/maps/api/geocode/json"`

	YandexAPIKey         string `yaml:"MAPS_YANDEX_API_KEY" env:"MAPS_YANDEX_API_KEY"`
	YandexGeocodeBaseURL string `yaml:"MAPS_YANDEX_GEOCODE_BASE_URL" env:"MAPS_YANDEX_GEOCODE_BASE_URL" env-default:"https://geocode-maps.yandex.ru/1.x/"`

	OSMNominatimBaseURL string `yaml:"MAPS_OSM_NOMINATIM_BASE_URL" env:"MAPS_OSM_NOMINATIM_BASE_URL" env-default:"https://nominatim.openstreetmap.org/search"`
	OSMUserAgent        string `yaml:"MAPS_OSM_USER_AGENT" env:"MAPS_OSM_USER_AGENT"`

	OSRMBaseURL string `yaml:"MAPS_OSRM_BASE_URL" env:"MAPS_OSRM_BASE_URL" env-default:"https://router.project-osrm.org"`
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
