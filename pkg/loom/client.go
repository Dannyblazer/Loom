package loom

import (
	"context"
	"fmt"

	"github.com/loom-payments/loom/config"
	"github.com/loom-payments/loom/pkg/idempotency"
	"github.com/loom-payments/loom/pkg/orchestrator"
	"github.com/loom-payments/loom/pkg/provider"
	"github.com/loom-payments/loom/pkg/provider/flutterwave"
	"github.com/loom-payments/loom/pkg/provider/paystack"
	"github.com/loom-payments/loom/pkg/types"
)

// Client is the main entry point for the Loom payment orchestration library
type Client struct {
	config       *ClientConfig
	registry     *provider.Registry
	orchestrator *orchestrator.Orchestrator
	idempotency  idempotency.Store

	// Service interfaces
	Charges         ChargeService
	Refunds         RefundService
	Transfers       TransferService
	VirtualAccounts VirtualAccountService
	Webhooks        WebhookService
}

// ClientConfig contains client configuration
type ClientConfig struct {
	// Provider configurations
	Paystack    *PaystackConfig
	Flutterwave *FlutterwaveConfig

	// Orchestration settings
	ProviderPriority []types.ProviderName
	FailoverEnabled  bool

	// Circuit breaker settings
	CircuitBreakerConfig *orchestrator.CircuitBreakerConfig

	// Idempotency store (optional)
	IdempotencyStore idempotency.Store
}

// PaystackConfig contains Paystack configuration
type PaystackConfig struct {
	SecretKey     string
	PublicKey     string
	WebhookSecret string
	BaseURL       string
}

// FlutterwaveConfig contains Flutterwave configuration
type FlutterwaveConfig struct {
	SecretKey     string
	PublicKey     string
	EncryptionKey string
	WebhookSecret string
	BaseURL       string
}

// NewClient creates a new Loom client with the provided options
func NewClient(opts ...Option) (*Client, error) {
	cfg := &ClientConfig{
		ProviderPriority: []types.ProviderName{types.ProviderPaystack, types.ProviderFlutterwave},
		FailoverEnabled:  true,
	}

	// Apply options
	for _, opt := range opts {
		opt(cfg)
	}

	// Create provider registry
	registry := provider.NewRegistry()

	// Register Paystack if configured
	if cfg.Paystack != nil && cfg.Paystack.SecretKey != "" {
		p := paystack.New(&provider.Config{
			SecretKey:     cfg.Paystack.SecretKey,
			PublicKey:     cfg.Paystack.PublicKey,
			WebhookSecret: cfg.Paystack.WebhookSecret,
			BaseURL:       cfg.Paystack.BaseURL,
		})
		if err := registry.Register(p); err != nil {
			return nil, fmt.Errorf("failed to register Paystack: %w", err)
		}
	}

	// Register Flutterwave if configured
	if cfg.Flutterwave != nil && cfg.Flutterwave.SecretKey != "" {
		f := flutterwave.New(&flutterwave.FlutterwaveConfig{
			Config: &provider.Config{
				SecretKey:     cfg.Flutterwave.SecretKey,
				PublicKey:     cfg.Flutterwave.PublicKey,
				WebhookSecret: cfg.Flutterwave.WebhookSecret,
				BaseURL:       cfg.Flutterwave.BaseURL,
			},
			EncryptionKey: cfg.Flutterwave.EncryptionKey,
		})
		if err := registry.Register(f); err != nil {
			return nil, fmt.Errorf("failed to register Flutterwave: %w", err)
		}
	}

	// Ensure at least one provider is registered
	if registry.Count() == 0 {
		return nil, fmt.Errorf("at least one payment provider must be configured")
	}

	// Create orchestrator
	orch := orchestrator.New(registry, &orchestrator.Config{
		PriorityOrder:        cfg.ProviderPriority,
		FailoverEnabled:      cfg.FailoverEnabled,
		CircuitBreakerConfig: cfg.CircuitBreakerConfig,
	})

	// Use provided idempotency store or create in-memory one
	var idempStore idempotency.Store
	if cfg.IdempotencyStore != nil {
		idempStore = cfg.IdempotencyStore
	} else {
		idempStore = idempotency.NewMemoryStore(nil)
	}

	client := &Client{
		config:       cfg,
		registry:     registry,
		orchestrator: orch,
		idempotency:  idempStore,
	}

	// Initialize services
	client.Charges = &chargeService{client: client}
	client.Refunds = &refundService{client: client}
	client.Transfers = &transferService{client: client}
	client.VirtualAccounts = &virtualAccountService{client: client}
	client.Webhooks = &webhookService{client: client, registry: registry}

	return client, nil
}

// NewClientFromConfig creates a client from a config.Config
func NewClientFromConfig(cfg *config.Config) (*Client, error) {
	opts := []Option{
		WithProviderPriority(cfg.Orchestration.ProviderPriority...),
		WithFailover(cfg.Orchestration.FailoverEnabled),
	}

	if cfg.Paystack.Enabled {
		opts = append(opts, WithPaystack(cfg.Paystack.SecretKey, cfg.Paystack.PublicKey))
	}

	if cfg.Flutterwave.Enabled {
		opts = append(opts, WithFlutterwave(cfg.Flutterwave.SecretKey, cfg.Flutterwave.PublicKey))
	}

	return NewClient(opts...)
}

// HealthCheck checks the health of all configured providers
func (c *Client) HealthCheck(ctx context.Context) map[types.ProviderName]error {
	return c.orchestrator.HealthCheck(ctx)
}

// GetProviderStatus returns the circuit breaker status for all providers
func (c *Client) GetProviderStatus() map[types.ProviderName]string {
	status := c.orchestrator.GetProviderStatus()
	result := make(map[types.ProviderName]string)
	for name, state := range status {
		result[name] = state.String()
	}
	return result
}

// GetProvider returns a specific provider by name
func (c *Client) GetProvider(name types.ProviderName) (provider.Provider, error) {
	return c.registry.Get(name)
}

// ChargeService provides charge-related operations
type ChargeService interface {
	Initialize(ctx context.Context, req *types.ChargeRequest) (*types.ChargeResponse, error)
	Verify(ctx context.Context, providerName types.ProviderName, reference string) (*types.ChargeResponse, error)
}

// RefundService provides refund-related operations
type RefundService interface {
	Create(ctx context.Context, req *types.RefundRequest) (*types.RefundResponse, error)
	Get(ctx context.Context, providerName types.ProviderName, refundID string) (*types.RefundResponse, error)
}

// TransferService provides transfer-related operations
type TransferService interface {
	Initiate(ctx context.Context, req *types.TransferRequest) (*types.TransferResponse, error)
	Verify(ctx context.Context, providerName types.ProviderName, reference string) (*types.TransferResponse, error)
	ListBanks(ctx context.Context, providerName types.ProviderName, country string) ([]types.Bank, error)
	ResolveAccount(ctx context.Context, providerName types.ProviderName, accountNumber, bankCode string) (*types.AccountInfo, error)
}

// VirtualAccountService provides virtual account operations
type VirtualAccountService interface {
	Create(ctx context.Context, req *types.VirtualAccountRequest) (*types.VirtualAccountResponse, error)
	Get(ctx context.Context, providerName types.ProviderName, accountID string) (*types.VirtualAccountResponse, error)
}

// WebhookService provides webhook handling
type WebhookService interface {
	VerifyAndParse(providerName types.ProviderName, payload []byte, signature string) (*types.WebhookEvent, error)
}

// Service implementations

type chargeService struct {
	client *Client
}

func (s *chargeService) Initialize(ctx context.Context, req *types.ChargeRequest) (*types.ChargeResponse, error) {
	return s.client.orchestrator.InitializeCharge(ctx, req)
}

func (s *chargeService) Verify(ctx context.Context, providerName types.ProviderName, reference string) (*types.ChargeResponse, error) {
	return s.client.orchestrator.VerifyCharge(ctx, providerName, reference)
}

type refundService struct {
	client *Client
}

func (s *refundService) Create(ctx context.Context, req *types.RefundRequest) (*types.RefundResponse, error) {
	return s.client.orchestrator.CreateRefund(ctx, req)
}

func (s *refundService) Get(ctx context.Context, providerName types.ProviderName, refundID string) (*types.RefundResponse, error) {
	p, err := s.client.registry.Get(providerName)
	if err != nil {
		return nil, err
	}
	return p.GetRefund(ctx, refundID)
}

type transferService struct {
	client *Client
}

func (s *transferService) Initiate(ctx context.Context, req *types.TransferRequest) (*types.TransferResponse, error) {
	return s.client.orchestrator.InitiateTransfer(ctx, req)
}

func (s *transferService) Verify(ctx context.Context, providerName types.ProviderName, reference string) (*types.TransferResponse, error) {
	return s.client.orchestrator.VerifyTransfer(ctx, providerName, reference)
}

func (s *transferService) ListBanks(ctx context.Context, providerName types.ProviderName, country string) ([]types.Bank, error) {
	return s.client.orchestrator.ListBanks(ctx, providerName, country)
}

func (s *transferService) ResolveAccount(ctx context.Context, providerName types.ProviderName, accountNumber, bankCode string) (*types.AccountInfo, error) {
	return s.client.orchestrator.ResolveAccount(ctx, providerName, accountNumber, bankCode)
}

type virtualAccountService struct {
	client *Client
}

func (s *virtualAccountService) Create(ctx context.Context, req *types.VirtualAccountRequest) (*types.VirtualAccountResponse, error) {
	return s.client.orchestrator.CreateVirtualAccount(ctx, req)
}

func (s *virtualAccountService) Get(ctx context.Context, providerName types.ProviderName, accountID string) (*types.VirtualAccountResponse, error) {
	return s.client.orchestrator.GetVirtualAccount(ctx, providerName, accountID)
}

type webhookService struct {
	client   *Client
	registry *provider.Registry
}

func (s *webhookService) VerifyAndParse(providerName types.ProviderName, payload []byte, signature string) (*types.WebhookEvent, error) {
	p, err := s.registry.Get(providerName)
	if err != nil {
		return nil, err
	}

	if !p.VerifyWebhookSignature(payload, signature) {
		return nil, types.ErrInvalidSignature
	}

	return p.ParseWebhookEvent(payload)
}
