package provider

import (
	"fmt"
	"sync"

	"github.com/loom-payments/loom/pkg/types"
)

// Registry manages registered payment providers
type Registry struct {
	providers map[types.ProviderName]Provider
	mu        sync.RWMutex
}

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[types.ProviderName]Provider),
	}
}

// Register adds a provider to the registry
func (r *Registry) Register(provider Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := provider.Name()
	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider %s is already registered", name)
	}

	r.providers[name] = provider
	return nil
}

// Unregister removes a provider from the registry
func (r *Registry) Unregister(name types.ProviderName) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, name)
}

// Get retrieves a provider by name
func (r *Registry) Get(name types.ProviderName) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not found: %w", name, types.ErrProviderNotFound)
	}
	return provider, nil
}

// GetAll returns all registered providers
func (r *Registry) GetAll() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}
	return providers
}

// GetNames returns the names of all registered providers
func (r *Registry) GetNames() []types.ProviderName {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]types.ProviderName, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// Has checks if a provider is registered
func (r *Registry) Has(name types.ProviderName) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.providers[name]
	return exists
}

// Count returns the number of registered providers
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.providers)
}

// GetByCurrency returns providers that support a given currency
func (r *Registry) GetByCurrency(currency types.Currency) []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var providers []Provider
	for _, p := range r.providers {
		if SupportsCurrency(p, currency) {
			providers = append(providers, p)
		}
	}
	return providers
}

// GetByFeature returns providers that support a given feature
func (r *Registry) GetByFeature(feature Feature) []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var providers []Provider
	for _, p := range r.providers {
		if SupportsFeature(p, feature) {
			providers = append(providers, p)
		}
	}
	return providers
}

// FindBest finds the best provider for a given request based on criteria
func (r *Registry) FindBest(currency types.Currency, feature Feature, preferred types.ProviderName) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// If preferred provider is specified and available, use it
	if preferred != "" {
		if p, exists := r.providers[preferred]; exists {
			if SupportsCurrency(p, currency) && SupportsFeature(p, feature) {
				return p, nil
			}
		}
	}

	// Find any provider that supports the currency and feature
	for _, p := range r.providers {
		if SupportsCurrency(p, currency) && SupportsFeature(p, feature) {
			return p, nil
		}
	}

	return nil, types.ErrNoProvidersAvailable
}
