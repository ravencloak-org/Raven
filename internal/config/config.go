package config

import (
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Valkey   ValkeyConfig
	GRPC     GRPCConfig
	OTel     OTelConfig
	Keycloak KeycloakConfig
}

// KeycloakConfig holds Keycloak/OIDC settings for JWT validation.
type KeycloakConfig struct {
	IssuerURL string `mapstructure:"issuer_url"`
	Audience  string `mapstructure:"audience"`
	// APIKeyEnabled enables the unvalidated API-key stub (see issue-24).
	// Disabled by default; set RAVEN_KEYCLOAK_APIKEYENABLED=true only in
	// development environments until the real DB-backed lookup is implemented.
	APIKeyEnabled bool `mapstructure:"api_key_enabled"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	URL string `mapstructure:"url"`
}

// ValkeyConfig holds Valkey (Redis-compatible) connection settings.
type ValkeyConfig struct {
	URL string `mapstructure:"url"`
}

// GRPCConfig holds gRPC client settings for the AI worker.
type GRPCConfig struct {
	WorkerAddr string `mapstructure:"worker_addr"`
}

// OTelConfig holds OpenTelemetry settings.
type OTelConfig struct {
	Endpoint    string `mapstructure:"endpoint"`
	ServiceName string `mapstructure:"service_name"`
	Enabled     bool   `mapstructure:"enabled"`
}

// Load reads configuration from environment variables and optional config file.
func Load() (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", "debug")
	v.SetDefault("grpc.worker_addr", "localhost:50051")
	v.SetDefault("otel.endpoint", "")
	v.SetDefault("otel.service_name", "raven-api")
	v.SetDefault("otel.enabled", false)
	v.SetDefault("keycloak.issuer_url", "http://localhost:8080/auth/realms/raven")
	v.SetDefault("keycloak.audience", "raven")

	// Config file (optional)
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	// Environment variables
	v.SetEnvPrefix("RAVEN")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Try to read config file but don't fail if not found
	_ = v.ReadInConfig()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
