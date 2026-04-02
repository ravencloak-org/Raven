package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application.
type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	Valkey     ValkeyConfig
	GRPC       GRPCConfig
	OTel       OTelConfig
	Keycloak   KeycloakConfig
	CORS       CORSConfig
	RateLimit  RateLimitConfig
	Queue      QueueConfig
	Encryption EncryptionConfig
	SeaweedFS  SeaweedFSConfig
	Upload     UploadConfig
	PostHog      PostHogConfig
	Hyperswitch  HyperswitchConfig
	LiveKit      LiveKitConfig
}

// PostHogConfig holds PostHog analytics settings.
// PostHog is opt-in: when APIKey is empty no events are sent.
type PostHogConfig struct {
	APIKey string `mapstructure:"api_key"`
	Host   string `mapstructure:"host"`
}

// HyperswitchConfig holds Hyperswitch payment orchestration settings.
type HyperswitchConfig struct {
	BaseURL       string `mapstructure:"base_url"`
	APIKey        string `mapstructure:"api_key"`
	WebhookSecret string `mapstructure:"webhook_secret"`
}

// LiveKitConfig holds LiveKit WebRTC server settings.
type LiveKitConfig struct {
	Host      string `mapstructure:"host"`
	APIKey    string `mapstructure:"api_key"`
	APISecret string `mapstructure:"api_secret"`
}

// QueueConfig holds Asynq job queue settings.
type QueueConfig struct {
	Concurrency int `mapstructure:"concurrency"`
	MaxRetry    int `mapstructure:"max_retry"`
}

// EncryptionConfig holds settings for data-at-rest encryption (e.g. LLM API keys).
type EncryptionConfig struct {
	AESKey string `mapstructure:"aes_key"`
}

// SeaweedFSConfig holds SeaweedFS connection settings.
type SeaweedFSConfig struct {
	MasterURL string `mapstructure:"master_url"`
}

// UploadConfig holds file upload settings.
type UploadConfig struct {
	MaxSizeBytes int64    `mapstructure:"max_size_bytes"`
	AllowedTypes []string `mapstructure:"allowed_types"`
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
	v.SetDefault("keycloak.issuer_url", "http://localhost:8080/auth/realms/raven")
	v.SetDefault("keycloak.audience", "raven")
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
	v.SetDefault("queue.concurrency", 10)
	v.SetDefault("queue.max_retry", 5)
	v.SetDefault("seaweedfs.master_url", "http://seaweedfs-master:9333")
	v.SetDefault("posthog.api_key", "")
	v.SetDefault("posthog.host", "https://us.i.posthog.com")
	v.SetDefault("hyperswitch.base_url", "http://localhost:8090")
	v.SetDefault("hyperswitch.api_key", "")
	v.SetDefault("hyperswitch.webhook_secret", "")
	v.SetDefault("livekit.host", "ws://localhost:7880")
	v.SetDefault("livekit.api_key", "devkey")
	v.SetDefault("livekit.api_secret", "devsecret")
	v.SetDefault("upload.max_size_bytes", 52428800) // 50 MB
	v.SetDefault("upload.allowed_types", []string{
		"application/pdf",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"text/html",
		"text/markdown",
		"text/plain",
		"text/csv",
	})

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
