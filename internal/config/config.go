package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application.
type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Valkey    ValkeyConfig
	GRPC      GRPCConfig
	OTel      OTelConfig
	CORS      CORSConfig
	RateLimit RateLimitConfig
}

// CORSConfig holds Cross-Origin Resource Sharing settings.
type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
}

// RateLimitConfig holds rate limiting defaults.
type RateLimitConfig struct {
	DefaultUserLimit int `mapstructure:"default_user_limit"`
	DefaultOrgLimit  int `mapstructure:"default_org_limit"`
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
	// CORS allowed origins can be overridden via the RAVEN_CORS_ALLOWED_ORIGINS
	// environment variable as a comma-separated list.
	// Example: RAVEN_CORS_ALLOWED_ORIGINS=https://app1.com,https://app2.com
	v.SetDefault("cors.allowed_origins", []string{
		"http://localhost:5173",
		"https://raven-frontend.pages.dev",
	})
	// Explicitly bind so Viper surfaces the env var when unmarshaling slice fields.
	_ = v.BindEnv("cors.allowed_origins", "RAVEN_CORS_ALLOWED_ORIGINS")
	v.SetDefault("ratelimit.default_user_limit", 1000)
	v.SetDefault("ratelimit.default_org_limit", 10000)

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

	if cfg.RateLimit.DefaultUserLimit <= 0 {
		return nil, fmt.Errorf("ratelimit.default_user_limit must be > 0, got %d", cfg.RateLimit.DefaultUserLimit)
	}
	if cfg.RateLimit.DefaultOrgLimit <= 0 {
		return nil, fmt.Errorf("ratelimit.default_org_limit must be > 0, got %d", cfg.RateLimit.DefaultOrgLimit)
	}

	return &cfg, nil
}
