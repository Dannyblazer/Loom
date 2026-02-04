package provider

import (
	"context"
	"time"

	"github.com/loom-payments/loom/pkg/types"
)

// Feature represents a capability a provider supports
type Feature string

const (
	FeatureCharges         Feature = "charges"
	FeatureRefunds         Feature = "refunds"
	FeaturePartialRefunds  Feature = "partial_refunds"
	FeatureTransfers       Feature = "transfers"
	FeatureBulkTransfers   Feature = "bulk_transfers"
	FeatureVirtualAccounts Feature = "virtual_accounts"
	FeatureRecurring       Feature = "recurring"
	FeatureSplitPayments   Feature = "split_payments"
	FeatureSubscriptions   Feature = "subscriptions"
)

// Provider defines the interface all payment providers must implement
type Provider interface {
	// Metadata
	Name() types.ProviderName
	SupportedCurrencies() []types.Currency
	SupportedFeatures() []Feature

	// Health check
	HealthCheck(ctx context.Context) error

	// Charges
	ChargeProvider

	// Refunds
	RefundProvider

	// Transfers
	TransferProvider

	// Virtual Accounts
	VirtualAccountProvider

	// Webhooks
	WebhookProvider
}

// ChargeProvider defines charge-related operations
type ChargeProvider interface {
	InitializeCharge(ctx context.Context, req *types.ChargeRequest) (*types.ChargeResponse, error)
	VerifyCharge(ctx context.Context, reference string) (*types.ChargeResponse, error)
}

// RefundProvider defines refund-related operations
type RefundProvider interface {
	CreateRefund(ctx context.Context, req *types.RefundRequest) (*types.RefundResponse, error)
	GetRefund(ctx context.Context, refundID string) (*types.RefundResponse, error)
}

// TransferProvider defines transfer-related operations
type TransferProvider interface {
	InitiateTransfer(ctx context.Context, req *types.TransferRequest) (*types.TransferResponse, error)
	VerifyTransfer(ctx context.Context, reference string) (*types.TransferResponse, error)
	ListBanks(ctx context.Context, country string) ([]types.Bank, error)
	ResolveAccount(ctx context.Context, accountNumber, bankCode string) (*types.AccountInfo, error)
	CreateRecipient(ctx context.Context, req *types.CreateRecipientRequest) (*types.RecipientResponse, error)
}

// VirtualAccountProvider defines virtual account operations
type VirtualAccountProvider interface {
	CreateVirtualAccount(ctx context.Context, req *types.VirtualAccountRequest) (*types.VirtualAccountResponse, error)
	GetVirtualAccount(ctx context.Context, accountID string) (*types.VirtualAccountResponse, error)
	DeactivateVirtualAccount(ctx context.Context, accountID string) error
}

// WebhookProvider defines webhook-related operations
type WebhookProvider interface {
	VerifyWebhookSignature(payload []byte, signature string) bool
	ParseWebhookEvent(payload []byte) (*types.WebhookEvent, error)
}

// Config contains provider-specific configuration
type Config struct {
	SecretKey     string
	PublicKey     string
	WebhookSecret string
	BaseURL       string
	Timeout       time.Duration
	RetryConfig   *RetryConfig
}

// RetryConfig contains retry configuration
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

// DefaultRetryConfig returns sensible retry defaults
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   5 * time.Second,
	}
}

// SupportsFeature checks if a provider supports a given feature
func SupportsFeature(p Provider, feature Feature) bool {
	for _, f := range p.SupportedFeatures() {
		if f == feature {
			return true
		}
	}
	return false
}

// SupportsCurrency checks if a provider supports a given currency
func SupportsCurrency(p Provider, currency types.Currency) bool {
	for _, c := range p.SupportedCurrencies() {
		if c == currency {
			return true
		}
	}
	return false
}
