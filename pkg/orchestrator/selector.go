package orchestrator

import (
	"math/rand"
	"sync"
	"time"

	"github.com/loom-payments/loom/pkg/provider"
	"github.com/loom-payments/loom/pkg/types"
)

// ProviderSelector defines the interface for provider selection strategies
type ProviderSelector interface {
	// Select returns providers in order of preference
	Select(providers []provider.Provider, req interface{}) []provider.Provider
}

// PrioritySelector selects providers based on a configured priority order
type PrioritySelector struct {
	priority []types.ProviderName
}

// NewPrioritySelector creates a new priority-based selector
func NewPrioritySelector(priority []types.ProviderName) *PrioritySelector {
	return &PrioritySelector{priority: priority}
}

// Select returns providers sorted by priority
func (s *PrioritySelector) Select(providers []provider.Provider, req interface{}) []provider.Provider {
	// Create a map for O(1) lookup
	providerMap := make(map[types.ProviderName]provider.Provider)
	for _, p := range providers {
		providerMap[p.Name()] = p
	}

	// Build result in priority order
	result := make([]provider.Provider, 0, len(providers))

	// Add providers in priority order
	for _, name := range s.priority {
		if p, exists := providerMap[name]; exists {
			result = append(result, p)
			delete(providerMap, name)
		}
	}

	// Add remaining providers not in priority list
	for _, p := range providerMap {
		result = append(result, p)
	}

	return result
}

// RoundRobinSelector distributes requests across providers evenly
type RoundRobinSelector struct {
	mu      sync.Mutex
	current int
}

// NewRoundRobinSelector creates a new round-robin selector
func NewRoundRobinSelector() *RoundRobinSelector {
	return &RoundRobinSelector{}
}

// Select returns providers with the next provider first
func (s *RoundRobinSelector) Select(providers []provider.Provider, req interface{}) []provider.Provider {
	if len(providers) == 0 {
		return providers
	}

	s.mu.Lock()
	start := s.current
	s.current = (s.current + 1) % len(providers)
	s.mu.Unlock()

	// Rotate the slice so the selected provider is first
	result := make([]provider.Provider, len(providers))
	for i := 0; i < len(providers); i++ {
		result[i] = providers[(start+i)%len(providers)]
	}

	return result
}

// RandomSelector randomly selects provider order
type RandomSelector struct {
	rng *rand.Rand
	mu  sync.Mutex
}

// NewRandomSelector creates a new random selector
func NewRandomSelector() *RandomSelector {
	return &RandomSelector{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Select returns providers in random order
func (s *RandomSelector) Select(providers []provider.Provider, req interface{}) []provider.Provider {
	if len(providers) <= 1 {
		return providers
	}

	// Make a copy to avoid modifying the original
	result := make([]provider.Provider, len(providers))
	copy(result, providers)

	s.mu.Lock()
	s.rng.Shuffle(len(result), func(i, j int) {
		result[i], result[j] = result[j], result[i]
	})
	s.mu.Unlock()

	return result
}

// WeightedSelector selects providers based on configured weights
type WeightedSelector struct {
	weights map[types.ProviderName]int
	rng     *rand.Rand
	mu      sync.Mutex
}

// NewWeightedSelector creates a new weighted selector
func NewWeightedSelector(weights map[types.ProviderName]int) *WeightedSelector {
	return &WeightedSelector{
		weights: weights,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Select returns providers ordered by weighted random selection
func (s *WeightedSelector) Select(providers []provider.Provider, req interface{}) []provider.Provider {
	if len(providers) <= 1 {
		return providers
	}

	// Calculate total weight
	totalWeight := 0
	providerWeights := make([]struct {
		provider provider.Provider
		weight   int
	}, len(providers))

	for i, p := range providers {
		weight := s.weights[p.Name()]
		if weight <= 0 {
			weight = 1 // Default weight
		}
		providerWeights[i] = struct {
			provider provider.Provider
			weight   int
		}{p, weight}
		totalWeight += weight
	}

	// Build result using weighted selection
	result := make([]provider.Provider, 0, len(providers))
	remaining := make([]struct {
		provider provider.Provider
		weight   int
	}, len(providerWeights))
	copy(remaining, providerWeights)

	s.mu.Lock()
	for len(remaining) > 0 {
		// Select based on weight
		r := s.rng.Intn(totalWeight)
		cumulative := 0
		selected := 0

		for i, pw := range remaining {
			cumulative += pw.weight
			if r < cumulative {
				selected = i
				break
			}
		}

		result = append(result, remaining[selected].provider)
		totalWeight -= remaining[selected].weight

		// Remove selected provider
		remaining = append(remaining[:selected], remaining[selected+1:]...)
	}
	s.mu.Unlock()

	return result
}

// CostBasedSelector selects providers based on transaction cost
type CostBasedSelector struct {
	costFn func(provider.Provider, interface{}) float64
}

// NewCostBasedSelector creates a cost-based selector
func NewCostBasedSelector(costFn func(provider.Provider, interface{}) float64) *CostBasedSelector {
	return &CostBasedSelector{costFn: costFn}
}

// Select returns providers sorted by cost (lowest first)
func (s *CostBasedSelector) Select(providers []provider.Provider, req interface{}) []provider.Provider {
	if len(providers) <= 1 {
		return providers
	}

	// Calculate costs
	type providerCost struct {
		provider provider.Provider
		cost     float64
	}

	costs := make([]providerCost, len(providers))
	for i, p := range providers {
		costs[i] = providerCost{p, s.costFn(p, req)}
	}

	// Sort by cost
	for i := 0; i < len(costs)-1; i++ {
		for j := i + 1; j < len(costs); j++ {
			if costs[j].cost < costs[i].cost {
				costs[i], costs[j] = costs[j], costs[i]
			}
		}
	}

	result := make([]provider.Provider, len(costs))
	for i, pc := range costs {
		result[i] = pc.provider
	}

	return result
}
