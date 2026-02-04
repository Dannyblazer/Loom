package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/loom-payments/loom/pkg/types"
)

// Config contains all configuration for Loom
type Config struct {
	// Server configuration
	Server ServerConfig

	// Database configuration
	Database DatabaseConfig

	// Provider configurations
	Paystack    PaystackConfig
	Flutterwave FlutterwaveConfig

	// Orchestration settings
	Orchestration OrchestrationConfig

	// Idempotency settings
	Idempotency IdempotencyConfig

	// Logging settings
	Logging LoggingConfig
}

// ServerConfig contains HTTP server configuration
type ServerConfig struct {
	Port        string
	Environment string
	BaseURL     string
	ReadTimeout time.Duration
	WriteTimeout time.Duration
}

// DatabaseConfig contains database configuration
type DatabaseConfig struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// PaystackConfig contains Paystack-specific configuration
type PaystackConfig struct {
	SecretKey     string
	PublicKey     string
	WebhookSecret string
	BaseURL       string
	Enabled       bool
	Timeout       time.Duration
}

// FlutterwaveConfig contains Flutterwave-specific configuration
type FlutterwaveConfig struct {
	SecretKey     string
	PublicKey     string
	EncryptionKey string
	WebhookSecret string
	BaseURL       string
	Enabled       bool
	Timeout       time.Duration
}

// OrchestrationConfig contains orchestration settings
type OrchestrationConfig struct {
	ProviderPriority          []types.ProviderName
	FailoverEnabled           bool
	CircuitBreakerMaxFailures int
	CircuitBreakerTimeout     time.Duration
	CircuitBreakerHalfOpen    int
}

// IdempotencyConfig contains idempotency settings
type IdempotencyConfig struct {
	Enabled         bool
	TTL             time.Duration
	LockDuration    time.Duration
	CleanupInterval time.Duration
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level  string
	Format string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if not found)
	_ = godotenv.Load()

	cfg := &Config{
		Server: ServerConfig{
			Port:         getEnvOrDefault("PORT", "8080"),
			Environment:  getEnvOrDefault("ENVIRONMENT", "development"),
			BaseURL:      getEnvOrDefault("BASE_URL", "http://localhost:8080"),
			ReadTimeout:  parseDuration(os.Getenv("SERVER_READ_TIMEOUT"), 30*time.Second),
			WriteTimeout: parseDuration(os.Getenv("SERVER_WRITE_TIMEOUT"), 30*time.Second),
		},
		Database: DatabaseConfig{
			URL:             os.Getenv("DATABASE_URL"),
			MaxOpenConns:    parseInt(os.Getenv("DB_MAX_OPEN_CONNS"), 25),
			MaxIdleConns:    parseInt(os.Getenv("DB_MAX_IDLE_CONNS"), 10),
			ConnMaxLifetime: parseDuration(os.Getenv("DB_CONN_MAX_LIFETIME"), 5*time.Minute),
			ConnMaxIdleTime: parseDuration(os.Getenv("DB_CONN_MAX_IDLE_TIME"), 5*time.Minute),
		},
		Paystack: PaystackConfig{
			SecretKey:     os.Getenv("PAYSTACK_SECRET_KEY"),
			PublicKey:     os.Getenv("PAYSTACK_PUBLIC_KEY"),
			WebhookSecret: os.Getenv("PAYSTACK_WEBHOOK_SECRET"),
			BaseURL:       getEnvOrDefault("PAYSTACK_BASE_URL", "https://api.paystack.co"),
			Enabled:       os.Getenv("PAYSTACK_ENABLED") != "false",
			Timeout:       parseDuration(os.Getenv("PAYSTACK_TIMEOUT"), 30*time.Second),
		},
		Flutterwave: FlutterwaveConfig{
			SecretKey:     os.Getenv("FLUTTERWAVE_SECRET_KEY"),
			PublicKey:     os.Getenv("FLUTTERWAVE_PUBLIC_KEY"),
			EncryptionKey: os.Getenv("FLUTTERWAVE_ENCRYPTION_KEY"),
			WebhookSecret: os.Getenv("FLUTTERWAVE_WEBHOOK_SECRET"),
			BaseURL:       getEnvOrDefault("FLUTTERWAVE_BASE_URL", "https://api.flutterwave.com/v3"),
			Enabled:       os.Getenv("FLUTTERWAVE_ENABLED") != "false",
			Timeout:       parseDuration(os.Getenv("FLUTTERWAVE_TIMEOUT"), 30*time.Second),
		},
		Orchestration: OrchestrationConfig{
			ProviderPriority:          parseProviderPriority(os.Getenv("PROVIDER_PRIORITY")),
			FailoverEnabled:           os.Getenv("FAILOVER_ENABLED") != "false",
			CircuitBreakerMaxFailures: parseInt(os.Getenv("CIRCUIT_BREAKER_MAX_FAILURES"), 5),
			CircuitBreakerTimeout:     parseDuration(os.Getenv("CIRCUIT_BREAKER_TIMEOUT"), 30*time.Second),
			CircuitBreakerHalfOpen:    parseInt(os.Getenv("CIRCUIT_BREAKER_HALF_OPEN"), 3),
		},
		Idempotency: IdempotencyConfig{
			Enabled:         os.Getenv("IDEMPOTENCY_ENABLED") != "false",
			TTL:             parseDuration(os.Getenv("IDEMPOTENCY_TTL"), 24*time.Hour),
			LockDuration:    parseDuration(os.Getenv("IDEMPOTENCY_LOCK_DURATION"), 30*time.Second),
			CleanupInterval: parseDuration(os.Getenv("IDEMPOTENCY_CLEANUP_INTERVAL"), 1*time.Hour),
		},
		Logging: LoggingConfig{
			Level:  getEnvOrDefault("LOG_LEVEL", "info"),
			Format: getEnvOrDefault("LOG_FORMAT", "json"),
		},
	}

	return cfg, cfg.Validate()
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Database is optional for library-only usage
	// But required for HTTP server mode

	// At least one provider must be enabled
	if !c.Paystack.Enabled && !c.Flutterwave.Enabled {
		return fmt.Errorf("at least one payment provider must be enabled")
	}

	// Validate Paystack config if enabled
	if c.Paystack.Enabled {
		if c.Paystack.SecretKey == "" {
			return fmt.Errorf("PAYSTACK_SECRET_KEY is required when Paystack is enabled")
		}
	}

	// Validate Flutterwave config if enabled
	if c.Flutterwave.Enabled {
		if c.Flutterwave.SecretKey == "" {
			return fmt.Errorf("FLUTTERWAVE_SECRET_KEY is required when Flutterwave is enabled")
		}
	}

	return nil
}

// IsProduction returns true if running in production environment
func (c *Config) IsProduction() bool {
	return c.Server.Environment == "production"
}

// IsDevelopment returns true if running in development environment
func (c *Config) IsDevelopment() bool {
	return c.Server.Environment == "development"
}

// Helper functions

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseInt(value string, defaultValue int) int {
	if value == "" {
		return defaultValue
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return i
}

func parseDuration(value string, defaultValue time.Duration) time.Duration {
	if value == "" {
		return defaultValue
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}
	return d
}

func parseProviderPriority(value string) []types.ProviderName {
	if value == "" {
		return []types.ProviderName{types.ProviderPaystack, types.ProviderFlutterwave}
	}

	parts := strings.Split(value, ",")
	providers := make([]types.ProviderName, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		switch p {
		case "paystack":
			providers = append(providers, types.ProviderPaystack)
		case "flutterwave":
			providers = append(providers, types.ProviderFlutterwave)
		case "monnify":
			providers = append(providers, types.ProviderMonnify)
		case "stripe":
			providers = append(providers, types.ProviderStripe)
		}
	}

	if len(providers) == 0 {
		return []types.ProviderName{types.ProviderPaystack, types.ProviderFlutterwave}
	}
	return providers
}
