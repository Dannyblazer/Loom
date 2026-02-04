package idempotency

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/loom-payments/loom/pkg/types"
)

// MemoryStore implements Store using an in-memory map
// Suitable for testing and single-instance deployments
type MemoryStore struct {
	records map[string]*Record
	config  *Config
	mu      sync.RWMutex
}

// NewMemoryStore creates a new in-memory idempotency store
func NewMemoryStore(config *Config) *MemoryStore {
	if config == nil {
		config = DefaultConfig()
	}

	store := &MemoryStore{
		records: make(map[string]*Record),
		config:  config,
	}

	// Start cleanup goroutine
	go store.startCleanup()

	return store
}

// Get retrieves an idempotency record by key
func (s *MemoryStore) Get(ctx context.Context, key string) (*Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.records[key]
	if !exists {
		return nil, nil
	}

	if record.IsExpired() {
		return nil, nil
	}

	return record, nil
}

// Set stores an idempotency record
func (s *MemoryStore) Set(ctx context.Context, record *Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for existing record with different request hash
	if existing, exists := s.records[record.Key]; exists {
		if !existing.IsExpired() && existing.RequestHash != record.RequestHash {
			return types.ErrIdempotencyConflict
		}
	}

	if record.ExpiresAt.IsZero() {
		record.ExpiresAt = time.Now().Add(s.config.TTL)
	}

	s.records[record.Key] = record
	return nil
}

// Lock attempts to acquire a lock on an idempotency key
func (s *MemoryStore) Lock(ctx context.Context, key string, requestHash string, lockDuration time.Duration) (*Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for existing record
	if existing, exists := s.records[key]; exists && !existing.IsExpired() {
		// Check if request hash matches
		if existing.RequestHash != requestHash {
			return nil, types.ErrIdempotencyConflict
		}

		// If record is complete, return it
		if !existing.Locked && existing.Response != nil {
			return existing, nil
		}

		// If still locked, return it to indicate in-progress
		if existing.IsLocked() {
			return existing, nil
		}
	}

	// Create new locked record
	now := time.Now()
	lockUntil := now.Add(lockDuration)

	record := &Record{
		Key:         key,
		RequestHash: requestHash,
		CreatedAt:   now,
		ExpiresAt:   now.Add(s.config.TTL),
		Locked:      true,
		LockedUntil: &lockUntil,
	}

	s.records[key] = record
	return nil, nil // nil record means lock acquired
}

// Unlock releases a lock and stores the final response
func (s *MemoryStore) Unlock(ctx context.Context, key string, response json.RawMessage, statusCode int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.records[key]
	if !exists {
		return nil // Already cleaned up, nothing to do
	}

	record.Response = response
	record.StatusCode = statusCode
	record.Locked = false
	record.LockedUntil = nil

	return nil
}

// Delete removes an idempotency record
func (s *MemoryStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, key)
	return nil
}

// Cleanup removes expired records
func (s *MemoryStore) Cleanup(ctx context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var count int64
	for key, record := range s.records {
		if record.IsExpired() {
			delete(s.records, key)
			count++
		}
	}

	return count, nil
}

// startCleanup periodically cleans up expired records
func (s *MemoryStore) startCleanup() {
	ticker := time.NewTicker(s.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		s.Cleanup(context.Background())
	}
}

// Count returns the number of records (for testing)
func (s *MemoryStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}
