package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// Record stores the result of an idempotent operation
type Record struct {
	Key          string          `json:"key"`
	RequestHash  string          `json:"request_hash"`
	Response     json.RawMessage `json:"response"`
	StatusCode   int             `json:"status_code"`
	ProviderName string          `json:"provider_name,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	ExpiresAt    time.Time       `json:"expires_at"`
	Locked       bool            `json:"locked"`
	LockedUntil  *time.Time      `json:"locked_until,omitempty"`
}

// IsExpired returns true if the record has expired
func (r *Record) IsExpired() bool {
	return time.Now().After(r.ExpiresAt)
}

// IsLocked returns true if the record is currently locked
func (r *Record) IsLocked() bool {
	if !r.Locked {
		return false
	}
	if r.LockedUntil == nil {
		return false
	}
	return time.Now().Before(*r.LockedUntil)
}

// Store defines the interface for idempotency storage
type Store interface {
	// Get retrieves an idempotency record by key
	Get(ctx context.Context, key string) (*Record, error)

	// Set stores an idempotency record
	Set(ctx context.Context, record *Record) error

	// Lock attempts to acquire a lock on an idempotency key
	// Returns the existing record if found, nil if lock acquired
	Lock(ctx context.Context, key string, requestHash string, lockDuration time.Duration) (*Record, error)

	// Unlock releases a lock and stores the final response
	Unlock(ctx context.Context, key string, response json.RawMessage, statusCode int) error

	// Delete removes an idempotency record
	Delete(ctx context.Context, key string) error

	// Cleanup removes expired records
	Cleanup(ctx context.Context) (int64, error)
}

// Config contains idempotency store configuration
type Config struct {
	// TTL is how long idempotency records are retained
	TTL time.Duration

	// LockDuration is how long a lock is held during processing
	LockDuration time.Duration

	// CleanupInterval is how often to run cleanup
	CleanupInterval time.Duration
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		TTL:             24 * time.Hour,
		LockDuration:    30 * time.Second,
		CleanupInterval: 1 * time.Hour,
	}
}

// HashRequest creates a hash of the request for detecting request changes
func HashRequest(request interface{}) (string, error) {
	data, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// Result represents the result of checking idempotency
type Result struct {
	// IsNew indicates this is a new request
	IsNew bool

	// IsDuplicate indicates this is a duplicate of a completed request
	IsDuplicate bool

	// IsConflict indicates the key was used with a different request
	IsConflict bool

	// IsInProgress indicates the request is currently being processed
	IsInProgress bool

	// CachedResponse contains the cached response if duplicate
	CachedResponse json.RawMessage

	// CachedStatusCode contains the cached status code if duplicate
	CachedStatusCode int
}
