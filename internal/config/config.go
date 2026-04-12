package config

import (
	"fmt"
	"math/bits"
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
	ClickHouse   ClickHouseConfig
	TTS          TTSConfig
	STT          STTConfig
	EBPF         EBPFConfig
	Meta         MetaConfig
}

// MetaConfig holds Meta WhatsApp Business API settings.
type MetaConfig struct {
	AccessToken   string `mapstructure:"access_token"`
	PhoneNumberID string `mapstructure:"phone_number_id"`
	AppSecret     string `mapstructure:"app_secret"`    // for webhook HMAC verification
	WebhookToken  string `mapstructure:"webhook_token"` // hub.verify_token
}

// ClickHouseConfig holds ClickHouse connection and vector backend settings.
type ClickHouseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Database string `mapstructure:"database"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	// VectorBackend selects the default vector storage backend: "pgvector" or "clickhouse".
	VectorBackend string `mapstructure:"vector_backend"`
	// ChunkThreshold is the per-org chunk count above which ClickHouse is preferred.
	ChunkThreshold int64 `mapstructure:"chunk_threshold"`
}

// PostHogConfig holds PostHog analytics settings.
// PostHog is opt-in: when APIKey is empty no events are sent.
type PostHogConfig struct {
	APIKey string `mapstructure:"api_key"`
	Host   string `mapstructure:"host"`
}

// HyperswitchConfig holds Hyperswitch payment orchestration settings.
type HyperswitchConfig struct {
	BaseURL        string `mapstructure:"base_url"`
	APIKey         string `mapstructure:"api_key"`
	WebhookSecret  string `mapstructure:"webhook_secret"`
	RazorpayKeyID  string `mapstructure:"razorpay_key_id"`
}

// LiveKitConfig holds LiveKit WebRTC server settings.
type LiveKitConfig struct {
	// APIURL is the LiveKit server HTTP(S) URL for RoomService API calls.
	APIURL string `mapstructure:"api_url"`
	// WSURL is the WebSocket URL returned to frontend clients for room connections.
	WSURL     string `mapstructure:"ws_url"`
	APIKey    string `mapstructure:"api_key"`
	APISecret string `mapstructure:"api_secret"`
}

// TTSConfig holds text-to-speech provider settings.
type TTSConfig struct {
	// Provider selects the active TTS backend: "cartesia" or "piper".
	Provider string `mapstructure:"provider"`

	// Cartesia Sonic API settings.
	CartesiaAPIKey  string `mapstructure:"cartesia_api_key"`
	CartesiaVoiceID string `mapstructure:"cartesia_voice_id"`
	CartesiaModel   string `mapstructure:"cartesia_model"`
	CartesiaBaseURL string `mapstructure:"cartesia_base_url"`

	// Piper self-hosted TTS settings.
	PiperEndpoint string `mapstructure:"piper_endpoint"`
	PiperVoice    string `mapstructure:"piper_voice"`
}

// STTConfig holds speech-to-text provider settings.
// Provider selects the backend: "deepgram" (cloud) or "whisper" (self-hosted).
// When Provider is empty, Deepgram is used if an API key is set, otherwise whisper.
type STTConfig struct {
	Provider        string `mapstructure:"provider"`
	DeepgramAPIKey  string `mapstructure:"deepgram_api_key"`
	DeepgramModel   string `mapstructure:"deepgram_model"`
	DeepgramBaseURL string `mapstructure:"deepgram_base_url"`
	WhisperEndpoint string `mapstructure:"whisper_endpoint"`
	WhisperModel    string `mapstructure:"whisper_model"`
}

// EBPFConfig holds eBPF feature flags. All features default to false — opt-in only.
// Kernel requirement: Linux ≥ 5.8 with CONFIG_DEBUG_INFO_BTF=y.
//
// Master switch: set RAVEN_EBPF_ENABLED=true to activate the subsystem.
// When false (the default) no eBPF code runs regardless of individual flags.
// On non-Linux platforms the subsystem is always a no-op regardless of this flag.
type EBPFConfig struct {
	// Enabled is the master switch for the entire eBPF subsystem.
	// Set RAVEN_EBPF_ENABLED=true to activate. All other flags are ignored when false.
	Enabled              bool     `mapstructure:"enabled"`
	ObservabilityEnabled bool     `mapstructure:"observability_enabled"`
	AuditEnabled         bool     `mapstructure:"audit_enabled"`
	AuditIPAllowlist     []string `mapstructure:"audit_ip_allowlist"`
	AuditExecAllowlist   []string `mapstructure:"audit_exec_allowlist"`
	// AuditRingBufferSize is the BPF ring buffer size in bytes. Must be a power of 2.
	AuditRingBufferSize int    `mapstructure:"audit_ring_buffer_size"`
	XDPEnabled          bool   `mapstructure:"xdp_enabled"`
	XDPInterface        string `mapstructure:"xdp_interface"`
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

	// Admin REST API credentials for realm auto-provisioning.
	// AdminURL is the base URL of the Keycloak admin REST API,
	// e.g. http://keycloak:8080.
	AdminURL           string `mapstructure:"admin_url"`
	AdminClientID      string `mapstructure:"admin_client_id"`
	AdminClientSecret  string `mapstructure:"admin_client_secret"`

	// InternalSecret is a shared secret for authenticating internal API
	// endpoints such as realm provisioning. Set via RAVEN_KEYCLOAK_INTERNAL_SECRET.
	InternalSecret string `mapstructure:"internal_secret"`
}

// CORSConfig holds Cross-Origin Resource Sharing settings.
type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
}

// RateLimitConfig holds rate limiting defaults and per-tier overrides.
type RateLimitConfig struct {
	DefaultUserLimit int `mapstructure:"default_user_limit"`
	DefaultOrgLimit  int `mapstructure:"default_org_limit"`

	// Per-tier org-level limits (requests per minute).
	FreeGeneralRPM       int `mapstructure:"free_general_rpm"`
	FreeCompletionRPM    int `mapstructure:"free_completion_rpm"`
	ProGeneralRPM        int `mapstructure:"pro_general_rpm"`
	ProCompletionRPM     int `mapstructure:"pro_completion_rpm"`
	EnterpriseGeneralRPM int `mapstructure:"enterprise_general_rpm"`
	EnterpriseCompletionRPM int `mapstructure:"enterprise_completion_rpm"` // -1 = unlimited

	// Widget limits — stricter for public chatbot widget endpoints.
	WidgetRPM int `mapstructure:"widget_rpm"`
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
	v.SetDefault("keycloak.admin_url", "http://localhost:8080")
	v.SetDefault("keycloak.admin_client_id", "admin-cli")
	v.SetDefault("keycloak.admin_client_secret", "")
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
	// Per-tier rate limits (requests per minute).
	v.SetDefault("ratelimit.free_general_rpm", 60)
	v.SetDefault("ratelimit.free_completion_rpm", 10)
	v.SetDefault("ratelimit.pro_general_rpm", 600)
	v.SetDefault("ratelimit.pro_completion_rpm", 120)
	v.SetDefault("ratelimit.enterprise_general_rpm", 6000)
	v.SetDefault("ratelimit.enterprise_completion_rpm", -1) // unlimited
	v.SetDefault("ratelimit.widget_rpm", 30)
	v.SetDefault("queue.concurrency", 10)
	v.SetDefault("queue.max_retry", 5)
	v.SetDefault("seaweedfs.master_url", "http://seaweedfs-master:9333")
	v.SetDefault("posthog.api_key", "")
	v.SetDefault("posthog.host", "https://us.i.posthog.com")
	v.SetDefault("hyperswitch.base_url", "http://localhost:8090")
	v.SetDefault("hyperswitch.api_key", "")
	v.SetDefault("hyperswitch.webhook_secret", "")
	v.SetDefault("hyperswitch.razorpay_key_id", "")
	v.SetDefault("livekit.api_url", "http://localhost:7880")
	v.SetDefault("livekit.ws_url", "ws://localhost:7880")
	v.SetDefault("livekit.api_key", "devkey")
	v.SetDefault("livekit.api_secret", "devsecret")
	v.SetDefault("clickhouse.host", "")
	v.SetDefault("clickhouse.port", 9000)
	v.SetDefault("clickhouse.database", "raven")
	v.SetDefault("clickhouse.user", "default")
	v.SetDefault("clickhouse.password", "")
	v.SetDefault("clickhouse.vector_backend", "pgvector")
	v.SetDefault("clickhouse.chunk_threshold", 5000000)
	// TTS defaults
	v.SetDefault("tts.provider", "cartesia")
	v.SetDefault("tts.cartesia_api_key", "")
	v.SetDefault("tts.cartesia_voice_id", "")
	v.SetDefault("tts.cartesia_model", "sonic-2")
	v.SetDefault("tts.cartesia_base_url", "")
	v.SetDefault("tts.piper_endpoint", "http://localhost:5000")
	v.SetDefault("tts.piper_voice", "en_US-amy-medium")
	// STT defaults
	v.SetDefault("stt.provider", "")
	v.SetDefault("stt.deepgram_api_key", "")
	v.SetDefault("stt.deepgram_model", "nova-2")
	v.SetDefault("stt.deepgram_base_url", "https://api.deepgram.com")
	v.SetDefault("stt.whisper_endpoint", "http://localhost:8000")
	v.SetDefault("stt.whisper_model", "large-v3")
	// eBPF defaults — master switch off; safe for non-Linux and existing deployments
	v.SetDefault("ebpf.enabled", false)
	v.SetDefault("ebpf.observability_enabled", false)
	v.SetDefault("ebpf.audit_enabled", false)
	v.SetDefault("ebpf.audit_ip_allowlist", []string{})
	v.SetDefault("ebpf.audit_exec_allowlist", []string{})
	v.SetDefault("ebpf.audit_ring_buffer_size", 1048576)
	v.SetDefault("ebpf.xdp_enabled", false)
	v.SetDefault("ebpf.xdp_interface", "eth0")
	// Meta WhatsApp Business API defaults
	v.SetDefault("meta.access_token", "")
	v.SetDefault("meta.phone_number_id", "")
	v.SetDefault("meta.app_secret", "")
	v.SetDefault("meta.webhook_token", "")
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

	// Explicitly bind nested keys — AutomaticEnv alone does not reliably surface
	// dotted keys during Unmarshal in all viper versions.
	_ = v.BindEnv("database.url", "RAVEN_DATABASE_URL")
	_ = v.BindEnv("valkey.url", "RAVEN_VALKEY_URL")
	_ = v.BindEnv("grpc.worker_addr", "RAVEN_GRPC_WORKER_ADDR")
	_ = v.BindEnv("keycloak.issuer_url", "RAVEN_KEYCLOAK_ISSUER_URL")
	_ = v.BindEnv("keycloak.audience", "RAVEN_KEYCLOAK_AUDIENCE")
	_ = v.BindEnv("keycloak.admin_url", "RAVEN_KEYCLOAK_ADMIN_URL")
	_ = v.BindEnv("keycloak.admin_client_id", "RAVEN_KEYCLOAK_ADMIN_CLIENT_ID")
	_ = v.BindEnv("keycloak.admin_client_secret", "RAVEN_KEYCLOAK_ADMIN_CLIENT_SECRET")
	_ = v.BindEnv("keycloak.internal_secret", "RAVEN_KEYCLOAK_INTERNAL_SECRET")
	_ = v.BindEnv("server.port", "RAVEN_SERVER_PORT")
	_ = v.BindEnv("server.mode", "RAVEN_SERVER_MODE")
	_ = v.BindEnv("clickhouse.host", "RAVEN_CLICKHOUSE_HOST")
	_ = v.BindEnv("clickhouse.port", "RAVEN_CLICKHOUSE_PORT")
	_ = v.BindEnv("clickhouse.database", "RAVEN_CLICKHOUSE_DATABASE")
	_ = v.BindEnv("clickhouse.user", "RAVEN_CLICKHOUSE_USER")
	_ = v.BindEnv("clickhouse.password", "RAVEN_CLICKHOUSE_PASSWORD")
	_ = v.BindEnv("clickhouse.vector_backend", "RAVEN_CLICKHOUSE_VECTOR_BACKEND")
	_ = v.BindEnv("clickhouse.chunk_threshold", "RAVEN_CLICKHOUSE_CHUNK_THRESHOLD")
	_ = v.BindEnv("tts.provider", "RAVEN_TTS_PROVIDER")
	_ = v.BindEnv("tts.cartesia_api_key", "RAVEN_TTS_CARTESIA_API_KEY")
	_ = v.BindEnv("tts.cartesia_voice_id", "RAVEN_TTS_CARTESIA_VOICE_ID")
	_ = v.BindEnv("tts.cartesia_model", "RAVEN_TTS_CARTESIA_MODEL")
	_ = v.BindEnv("tts.cartesia_base_url", "RAVEN_TTS_CARTESIA_BASE_URL")
	_ = v.BindEnv("tts.piper_endpoint", "RAVEN_TTS_PIPER_ENDPOINT")
	_ = v.BindEnv("tts.piper_voice", "RAVEN_TTS_PIPER_VOICE")
	_ = v.BindEnv("stt.provider", "RAVEN_STT_PROVIDER")
	_ = v.BindEnv("stt.deepgram_api_key", "RAVEN_STT_DEEPGRAM_API_KEY")
	_ = v.BindEnv("stt.deepgram_model", "RAVEN_STT_DEEPGRAM_MODEL")
	_ = v.BindEnv("stt.deepgram_base_url", "RAVEN_STT_DEEPGRAM_BASE_URL")
	_ = v.BindEnv("stt.whisper_endpoint", "RAVEN_STT_WHISPER_ENDPOINT")
	_ = v.BindEnv("stt.whisper_model", "RAVEN_STT_WHISPER_MODEL")
	_ = v.BindEnv("livekit.api_url", "RAVEN_LIVEKIT_API_URL")
	_ = v.BindEnv("livekit.ws_url", "RAVEN_LIVEKIT_WS_URL")
	_ = v.BindEnv("livekit.api_key", "RAVEN_LIVEKIT_API_KEY")
	_ = v.BindEnv("livekit.api_secret", "RAVEN_LIVEKIT_API_SECRET")
	_ = v.BindEnv("encryption.aes_key", "RAVEN_ENCRYPTION_AES_KEY")
	_ = v.BindEnv("ratelimit.free_general_rpm", "RAVEN_RATELIMIT_FREE_GENERAL_RPM")
	_ = v.BindEnv("ratelimit.free_completion_rpm", "RAVEN_RATELIMIT_FREE_COMPLETION_RPM")
	_ = v.BindEnv("ratelimit.pro_general_rpm", "RAVEN_RATELIMIT_PRO_GENERAL_RPM")
	_ = v.BindEnv("ratelimit.pro_completion_rpm", "RAVEN_RATELIMIT_PRO_COMPLETION_RPM")
	_ = v.BindEnv("ratelimit.enterprise_general_rpm", "RAVEN_RATELIMIT_ENTERPRISE_GENERAL_RPM")
	_ = v.BindEnv("ratelimit.enterprise_completion_rpm", "RAVEN_RATELIMIT_ENTERPRISE_COMPLETION_RPM")
	_ = v.BindEnv("ratelimit.widget_rpm", "RAVEN_RATELIMIT_WIDGET_RPM")
	_ = v.BindEnv("otel.endpoint", "RAVEN_OTEL_ENDPOINT")
	_ = v.BindEnv("otel.service_name", "RAVEN_OTEL_SERVICE_NAME")
	_ = v.BindEnv("otel.enabled", "RAVEN_OTEL_ENABLED")
	_ = v.BindEnv("hyperswitch.base_url", "RAVEN_HYPERSWITCH_BASE_URL")
	_ = v.BindEnv("hyperswitch.api_key", "RAVEN_HYPERSWITCH_API_KEY")
	_ = v.BindEnv("hyperswitch.webhook_secret", "RAVEN_HYPERSWITCH_WEBHOOK_SECRET")
	_ = v.BindEnv("hyperswitch.razorpay_key_id", "RAVEN_HYPERSWITCH_RAZORPAY_KEY_ID")
	_ = v.BindEnv("ebpf.enabled", "RAVEN_EBPF_ENABLED")
	_ = v.BindEnv("ebpf.observability_enabled", "RAVEN_EBPF_OBSERVABILITY_ENABLED")
	_ = v.BindEnv("ebpf.audit_enabled", "RAVEN_EBPF_AUDIT_ENABLED")
	_ = v.BindEnv("ebpf.audit_ip_allowlist", "RAVEN_EBPF_AUDIT_IP_ALLOWLIST")
	_ = v.BindEnv("ebpf.audit_exec_allowlist", "RAVEN_EBPF_AUDIT_EXEC_ALLOWLIST")
	_ = v.BindEnv("ebpf.audit_ring_buffer_size", "RAVEN_EBPF_AUDIT_RING_BUFFER_SIZE")
	_ = v.BindEnv("ebpf.xdp_enabled", "RAVEN_EBPF_XDP_ENABLED")
	_ = v.BindEnv("ebpf.xdp_interface", "RAVEN_EBPF_XDP_INTERFACE")
	_ = v.BindEnv("hyperswitch.base_url", "RAVEN_HYPERSWITCH_BASE_URL")
	_ = v.BindEnv("hyperswitch.api_key", "RAVEN_HYPERSWITCH_API_KEY")
	_ = v.BindEnv("hyperswitch.webhook_secret", "RAVEN_HYPERSWITCH_WEBHOOK_SECRET")

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

	// Validate ring buffer size when eBPF is enabled — catches misconfiguration before any BPF load.
	// Validated even when individual AuditEnabled=false so that a stale invalid value doesn't
	// silently survive until someone enables audit later.
	if cfg.EBPF.Enabled {
		size := cfg.EBPF.AuditRingBufferSize
		if size <= 0 || bits.OnesCount(uint(size)) != 1 {
			return nil, fmt.Errorf("ebpf.audit_ring_buffer_size must be a power of 2 > 0, got %d", size)
		}
	}

	return &cfg, nil
}
