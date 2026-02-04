package orchestrator

import (
	"context"
	"fmt"
	"sync"

	"github.com/loom-payments/loom/pkg/provider"
	"github.com/loom-payments/loom/pkg/types"
)

// Orchestrator manages provider selection and failover
type Orchestrator struct {
	registry        *provider.Registry
	circuitBreakers map[types.ProviderName]*CircuitBreaker
	selector        ProviderSelector
	config          *Config
	mu              sync.RWMutex
}

// Config contains orchestrator configuration
type Config struct {
	// Provider priority order (first is highest priority)
	PriorityOrder []types.ProviderName

	// Enable automatic failover to next provider on failure
	FailoverEnabled bool

	// Circuit breaker configuration
	CircuitBreakerConfig *CircuitBreakerConfig
}

// DefaultConfig returns sensible default configuration
func DefaultConfig() *Config {
	return &Config{
		PriorityOrder:        []types.ProviderName{types.ProviderPaystack, types.ProviderFlutterwave},
		FailoverEnabled:      true,
		CircuitBreakerConfig: DefaultCircuitBreakerConfig(),
	}
}

// New creates a new Orchestrator
func New(registry *provider.Registry, config *Config) *Orchestrator {
	if config == nil {
		config = DefaultConfig()
	}
	if config.CircuitBreakerConfig == nil {
		config.CircuitBreakerConfig = DefaultCircuitBreakerConfig()
	}

	o := &Orchestrator{
		registry:        registry,
		circuitBreakers: make(map[types.ProviderName]*CircuitBreaker),
		config:          config,
	}

	// Initialize circuit breakers for all providers
	for _, name := range registry.GetNames() {
		o.circuitBreakers[name] = NewCircuitBreaker(config.CircuitBreakerConfig)
	}

	// Set up provider selector
	o.selector = &PrioritySelector{priority: config.PriorityOrder}

	return o
}

// InitializeCharge attempts to initialize a charge using failover logic
func (o *Orchestrator) InitializeCharge(ctx context.Context, req *types.ChargeRequest) (*types.ChargeResponse, error) {
	providers := o.selectProviders(req.PreferredProvider, req.Amount.Currency, provider.FeatureCharges)
	return executeWithFailover(ctx, providers, o, func(p provider.Provider) (*types.ChargeResponse, error) {
		return p.InitializeCharge(ctx, req)
	})
}

// VerifyCharge verifies a charge (no failover - must use same provider)
func (o *Orchestrator) VerifyCharge(ctx context.Context, providerName types.ProviderName, reference string) (*types.ChargeResponse, error) {
	p, err := o.registry.Get(providerName)
	if err != nil {
		return nil, err
	}
	return p.VerifyCharge(ctx, reference)
}

// CreateRefund creates a refund (typically uses same provider as original charge)
func (o *Orchestrator) CreateRefund(ctx context.Context, req *types.RefundRequest) (*types.RefundResponse, error) {
	// Refunds should go to the same provider as the original transaction
	if req.PreferredProvider != "" {
		p, err := o.registry.Get(req.PreferredProvider)
		if err != nil {
			return nil, err
		}
		return p.CreateRefund(ctx, req)
	}

	// If no provider specified, try all providers (unusual case)
	providers := o.selectProviders("", types.NGN, provider.FeatureRefunds)
	return executeWithFailover(ctx, providers, o, func(p provider.Provider) (*types.RefundResponse, error) {
		return p.CreateRefund(ctx, req)
	})
}

// InitiateTransfer initiates a transfer using failover logic
func (o *Orchestrator) InitiateTransfer(ctx context.Context, req *types.TransferRequest) (*types.TransferResponse, error) {
	providers := o.selectProviders(req.PreferredProvider, req.Amount.Currency, provider.FeatureTransfers)
	return executeWithFailover(ctx, providers, o, func(p provider.Provider) (*types.TransferResponse, error) {
		return p.InitiateTransfer(ctx, req)
	})
}

// VerifyTransfer verifies a transfer
func (o *Orchestrator) VerifyTransfer(ctx context.Context, providerName types.ProviderName, reference string) (*types.TransferResponse, error) {
	p, err := o.registry.Get(providerName)
	if err != nil {
		return nil, err
	}
	return p.VerifyTransfer(ctx, reference)
}

// ListBanks lists banks from a provider
func (o *Orchestrator) ListBanks(ctx context.Context, providerName types.ProviderName, country string) ([]types.Bank, error) {
	var p provider.Provider
	var err error

	if providerName != "" {
		p, err = o.registry.Get(providerName)
	} else {
		// Use first available provider
		providers := o.registry.GetByFeature(provider.FeatureTransfers)
		if len(providers) == 0 {
			return nil, types.ErrNoProvidersAvailable
		}
		p = providers[0]
	}

	if err != nil {
		return nil, err
	}
	return p.ListBanks(ctx, country)
}

// ResolveAccount resolves a bank account
func (o *Orchestrator) ResolveAccount(ctx context.Context, providerName types.ProviderName, accountNumber, bankCode string) (*types.AccountInfo, error) {
	var p provider.Provider
	var err error

	if providerName != "" {
		p, err = o.registry.Get(providerName)
	} else {
		providers := o.registry.GetByFeature(provider.FeatureTransfers)
		if len(providers) == 0 {
			return nil, types.ErrNoProvidersAvailable
		}
		p = providers[0]
	}

	if err != nil {
		return nil, err
	}
	return p.ResolveAccount(ctx, accountNumber, bankCode)
}

// CreateVirtualAccount creates a virtual account using failover logic
func (o *Orchestrator) CreateVirtualAccount(ctx context.Context, req *types.VirtualAccountRequest) (*types.VirtualAccountResponse, error) {
	currency := req.Currency
	if currency == "" {
		currency = types.NGN
	}

	providers := o.selectProviders(req.PreferredProvider, currency, provider.FeatureVirtualAccounts)
	return executeWithFailover(ctx, providers, o, func(p provider.Provider) (*types.VirtualAccountResponse, error) {
		return p.CreateVirtualAccount(ctx, req)
	})
}

// GetVirtualAccount retrieves a virtual account
func (o *Orchestrator) GetVirtualAccount(ctx context.Context, providerName types.ProviderName, accountID string) (*types.VirtualAccountResponse, error) {
	p, err := o.registry.Get(providerName)
	if err != nil {
		return nil, err
	}
	return p.GetVirtualAccount(ctx, accountID)
}

// HealthCheck checks the health of all providers
func (o *Orchestrator) HealthCheck(ctx context.Context) map[types.ProviderName]error {
	results := make(map[types.ProviderName]error)
	providers := o.registry.GetAll()

	for _, p := range providers {
		err := p.HealthCheck(ctx)
		results[p.Name()] = err
	}

	return results
}

// GetProviderStatus returns the status of all circuit breakers
func (o *Orchestrator) GetProviderStatus() map[types.ProviderName]CircuitState {
	o.mu.RLock()
	defer o.mu.RUnlock()

	status := make(map[types.ProviderName]CircuitState)
	for name, cb := range o.circuitBreakers {
		status[name] = cb.State()
	}
	return status
}

// selectProviders selects providers based on criteria
func (o *Orchestrator) selectProviders(preferred types.ProviderName, currency types.Currency, feature provider.Feature) []provider.Provider {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var candidates []provider.Provider

	// If a preferred provider is specified and available, put it first
	if preferred != "" {
		if p, err := o.registry.Get(preferred); err == nil {
			if provider.SupportsCurrency(p, currency) && provider.SupportsFeature(p, feature) {
				cb := o.circuitBreakers[preferred]
				if cb == nil || cb.Allow() {
					candidates = append(candidates, p)
				}
			}
		}
	}

	// Add remaining providers by priority
	for _, name := range o.config.PriorityOrder {
		if name == preferred {
			continue // Already added
		}

		p, err := o.registry.Get(name)
		if err != nil {
			continue
		}

		if !provider.SupportsCurrency(p, currency) || !provider.SupportsFeature(p, feature) {
			continue
		}

		cb := o.circuitBreakers[name]
		if cb != nil && !cb.Allow() {
			continue // Circuit is open
		}

		candidates = append(candidates, p)
	}

	// Add any providers not in priority order
	for _, p := range o.registry.GetAll() {
		// Check if already added
		found := false
		for _, c := range candidates {
			if c.Name() == p.Name() {
				found = true
				break
			}
		}
		if found {
			continue
		}

		if !provider.SupportsCurrency(p, currency) || !provider.SupportsFeature(p, feature) {
			continue
		}

		cb := o.circuitBreakers[p.Name()]
		if cb != nil && !cb.Allow() {
			continue
		}

		candidates = append(candidates, p)
	}

	return candidates
}

// recordSuccess records a successful operation for a provider
func (o *Orchestrator) recordSuccess(name types.ProviderName) {
	o.mu.RLock()
	cb, exists := o.circuitBreakers[name]
	o.mu.RUnlock()

	if exists {
		cb.RecordSuccess()
	}
}

// recordFailure records a failed operation for a provider
func (o *Orchestrator) recordFailure(name types.ProviderName) {
	o.mu.RLock()
	cb, exists := o.circuitBreakers[name]
	o.mu.RUnlock()

	if exists {
		cb.RecordFailure()
	}
}

// executeWithFailover executes an operation with failover support
func executeWithFailover[T any](ctx context.Context, providers []provider.Provider, o *Orchestrator, fn func(p provider.Provider) (T, error)) (T, error) {
	var zero T

	if len(providers) == 0 {
		return zero, types.ErrNoProvidersAvailable
	}

	var errors []error

	for i, p := range providers {
		// Only try first provider if failover is disabled
		if !o.config.FailoverEnabled && i > 0 {
			break
		}

		result, err := fn(p)
		if err == nil {
			o.recordSuccess(p.Name())
			return result, nil
		}

		o.recordFailure(p.Name())
		errors = append(errors, fmt.Errorf("provider %s: %w", p.Name(), err))
	}

	if len(errors) > 0 {
		return zero, types.NewFailoverError(errors)
	}

	return zero, types.ErrAllProvidersFailed
}
