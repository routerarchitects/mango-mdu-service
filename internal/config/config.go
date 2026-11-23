package config

import (
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	subsystem "github.com/routerarchitects/ow-common-mods/fiber/system-routes"
	"github.com/routerarchitects/ow-common-mods/servicediscovery"
	"github.com/routerarchitects/ra-common-mods/kafka"
	"github.com/routerarchitects/ra-common-mods/logger"
)

type ServerConfig struct {
	HTTPPort       int           `env:"HTTP_PORT" envDefault:"16010"`
	PrivatePort    int           `env:"PRIVATE_HTTP_PORT" envDefault:"17010"`
	TLS_CERT       string        `env:"INTERNAL_RESTAPI_HOST_CERT"`
	TLS_KEY        string        `env:"INTERNAL_RESTAPI_HOST_KEY"`
	TLS_ROOTCA     string        `env:"INTERNAL_RESTAPI_HOST_ROOTCA"`
	WSWriteTimeout time.Duration `env:"WS_WRITE_TIMEOUT" envDefault:"10s"`
}

type PostgresConfig struct {
	StorageType string `env:"STORAGE_TYPE" envDefault:"postgresql"`
	Host        string `env:"STORAGE_TYPE_POSTGRESQL_HOST" envDefault:"localhost"`
	Port        int    `env:"STORAGE_TYPE_POSTGRESQL_PORT" envDefault:"5432"`
	Username    string `env:"STORAGE_TYPE_POSTGRESQL_USERNAME" envDefault:"postgres"`
	Password    string `env:"STORAGE_TYPE_POSTGRESQL_PASSWORD" envDefault:"postgres"`
	Database    string `env:"STORAGE_TYPE_POSTGRESQL_DATABASE" envDefault:"postgres"`
	SSLMode     string `env:"STORAGE_TYPE_POSTGRESQL_SSLMODE" envDefault:"disable"`
}

type KafkaConfig struct {
	kafka.Config
}

type DiscoveryConfig struct {
	Enabled                 bool   `env:"DISCOVERY_ENABLED" envDefault:"true"`
	InternalAPIKey          string `env:"INTERNAL_API_KEY"`
	PublicEndpointOverride  string `env:"PUBLIC_ENDPOINT"`
	PrivateEndpointOverride string `env:"PRIVATE_ENDPOINT"`
	servicediscovery.Config
}

type RPCConfig struct {
	Enabled bool `env:"SERVICE_RPC_ENABLED" envDefault:"true"`
}

type AuthConfig struct {
	Enabled bool `env:"AUTH_ENABLED" envDefault:"true"`
}

type LoggerConfig struct {
	logger.Config
}

type SubsystemConfig struct {
	subsystem.Config
}

type Config struct {
	Server    ServerConfig
	Database  PostgresConfig
	Kafka     KafkaConfig
	Discovery DiscoveryConfig
	RPC       RPCConfig
	Auth      AuthConfig
	Logger    LoggerConfig
	Subsystem SubsystemConfig
}

// GetAPISecret returns the configured internal API secret, preferring INTERNAL_API_KEY if set,
// then falling back to SYSTEM_INSTANCE_KEY.
func (c *Config) GetAPISecret() string {
	if s := strings.TrimSpace(c.Discovery.InternalAPIKey); s != "" {
		return s
	}
	return strings.TrimSpace(c.Discovery.InstanceKey)
}

// GetPublicEndpoint returns the public endpoint URL, preferring PUBLIC_ENDPOINT if set,
// then falling back to SYSTEM_URI_PUBLIC.
func (c *Config) GetPublicEndpoint() string {
	if s := strings.TrimSpace(c.Discovery.PublicEndpointOverride); s != "" {
		return s
	}
	return strings.TrimSpace(c.Discovery.PublicEndpoint)
}

// GetPrivateEndpoint returns the private endpoint URL, preferring PRIVATE_ENDPOINT if set,
// then falling back to SYSTEM_URI_PRIVATE.
func (c *Config) GetPrivateEndpoint() string {
	if s := strings.TrimSpace(c.Discovery.PrivateEndpointOverride); s != "" {
		return s
	}
	return strings.TrimSpace(c.Discovery.PrivateEndpoint)
}

// Load parses environment variables into the Config struct.
func Load() (*Config, error) {
	var c Config
	if err := env.Parse(&c); err != nil {
		return nil, err
	}
	return &c, nil
}
