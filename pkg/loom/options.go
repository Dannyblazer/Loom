package loom

import (
	"github.com/loom-payments/loom/pkg/idempotency"
	"github.com/loom-payments/loom/pkg/orchestrator"
	"github.com/loom-payments/loom/pkg/types"
)

// Option is a functional option for configuring the Client
type Option func(*ClientConfig)

// WithPaystack configures Paystack as a provider
func WithPaystack(secretKey, publicKey string) Option {
	return func(cfg *ClientConfig) {
		cfg.Paystack = &PaystackConfig{
			SecretKey: secretKey,
			PublicKey: publicKey,
		}
	}
}

// WithPaystackFull configures Paystack with all options
func WithPaystackFull(secretKey, publicKey, webhookSecret, baseURL string) Option {
	return func(cfg *ClientConfig) {
		cfg.Paystack = &PaystackConfig{
			SecretKey:     secretKey,
			PublicKey:     publicKey,
			WebhookSecret: webhookSecret,
			BaseURL:       baseURL,
		}
	}
}

// WithFlutterwave configures Flutterwave as a provider
func WithFlutterwave(secretKey, publicKey string) Option {
	return func(cfg *ClientConfig) {
		cfg.Flutterwave = &FlutterwaveConfig{
			SecretKey: secretKey,
			PublicKey: publicKey,
		}
	}
}

// WithFlutterwaveFull configures Flutterwave with all options
func WithFlutterwaveFull(secretKey, publicKey, encryptionKey, webhookSecret, baseURL string) Option {
	return func(cfg *ClientConfig) {
		cfg.Flutterwave = &FlutterwaveConfig{
			SecretKey:     secretKey,
			PublicKey:     publicKey,
			EncryptionKey: encryptionKey,
			WebhookSecret: webhookSecret,
			BaseURL:       baseURL,
		}
	}
}

// WithProviderPriority sets the provider priority order for failover
func WithProviderPriority(providers ...types.ProviderName) Option {
	return func(cfg *ClientConfig) {
		cfg.ProviderPriority = providers
	}
}

// WithFailover enables or disables automatic failover
func WithFailover(enabled bool) Option {
	return func(cfg *ClientConfig) {
		cfg.FailoverEnabled = enabled
	}
}

// WithCircuitBreaker configures the circuit breaker
func WithCircuitBreaker(config *orchestrator.CircuitBreakerConfig) Option {
	return func(cfg *ClientConfig) {
		cfg.CircuitBreakerConfig = config
	}
}

// WithIdempotencyStore sets a custom idempotency store
func WithIdempotencyStore(store idempotency.Store) Option {
	return func(cfg *ClientConfig) {
		cfg.IdempotencyStore = store
	}
}

// WithPaystackWebhookSecret sets the Paystack webhook secret
func WithPaystackWebhookSecret(secret string) Option {
	return func(cfg *ClientConfig) {
		if cfg.Paystack == nil {
			cfg.Paystack = &PaystackConfig{}
		}
		cfg.Paystack.WebhookSecret = secret
	}
}

// WithFlutterwaveWebhookSecret sets the Flutterwave webhook secret
func WithFlutterwaveWebhookSecret(secret string) Option {
	return func(cfg *ClientConfig) {
		if cfg.Flutterwave == nil {
			cfg.Flutterwave = &FlutterwaveConfig{}
		}
		cfg.Flutterwave.WebhookSecret = secret
	}
}
