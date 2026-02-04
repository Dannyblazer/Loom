package idempotency

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/loom-payments/loom/pkg/types"
)

// IdempotencyKeyModel is the GORM model for idempotency keys
type IdempotencyKeyModel struct {
	Key          string          `gorm:"primaryKey;type:varchar(255)"`
	RequestHash  string          `gorm:"type:varchar(64);not null"`
	Response     json.RawMessage `gorm:"type:jsonb"`
	StatusCode   int             `gorm:"not null;default:0"`
	ProviderName string          `gorm:"type:varchar(50)"`
	Locked       bool            `gorm:"not null;default:false"`
	LockedUntil  *time.Time      `gorm:"index"`
	CreatedAt    time.Time       `gorm:"autoCreateTime"`
	ExpiresAt    time.Time       `gorm:"index;not null"`
}

// TableName returns the table name for GORM
func (IdempotencyKeyModel) TableName() string {
	return "loom_idempotency_keys"
}

// PostgresStore implements Store using PostgreSQL
type PostgresStore struct {
	db     *gorm.DB
	config *Config
}

// NewPostgresStore creates a new PostgreSQL-backed idempotency store
func NewPostgresStore(db *gorm.DB, config *Config) (*PostgresStore, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Auto-migrate the table
	if err := db.AutoMigrate(&IdempotencyKeyModel{}); err != nil {
		return nil, err
	}

	store := &PostgresStore{
		db:     db,
		config: config,
	}

	// Start background cleanup
	go store.startCleanup()

	return store, nil
}

// Get retrieves an idempotency record by key
func (s *PostgresStore) Get(ctx context.Context, key string) (*Record, error) {
	var model IdempotencyKeyModel
	err := s.db.WithContext(ctx).
		Where("key = ? AND expires_at > ?", key, time.Now()).
		First(&model).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return modelToRecord(&model), nil
}

// Set stores an idempotency record
func (s *PostgresStore) Set(ctx context.Context, record *Record) error {
	model := recordToModel(record)

	if model.ExpiresAt.IsZero() {
		model.ExpiresAt = time.Now().Add(s.config.TTL)
	}

	// Use upsert to handle concurrent requests
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"response", "status_code", "locked", "locked_until",
		}),
	}).Create(&model).Error
}

// Lock attempts to acquire a lock on an idempotency key
func (s *PostgresStore) Lock(ctx context.Context, key string, requestHash string, lockDuration time.Duration) (*Record, error) {
	var result *Record

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing IdempotencyKeyModel

		// Try to get existing record with lock
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("key = ? AND expires_at > ?", key, time.Now()).
			First(&existing).Error

		if err == nil {
			// Record exists
			if existing.RequestHash != requestHash {
				return types.ErrIdempotencyConflict
			}

			// If completed, return the response
			if !existing.Locked && existing.Response != nil {
				result = modelToRecord(&existing)
				return nil
			}

			// If still locked, return to indicate in-progress
			if existing.Locked && existing.LockedUntil != nil && time.Now().Before(*existing.LockedUntil) {
				result = modelToRecord(&existing)
				return nil
			}

			// Lock expired, refresh it
			lockUntil := time.Now().Add(lockDuration)
			existing.Locked = true
			existing.LockedUntil = &lockUntil
			return tx.Save(&existing).Error
		}

		if err != gorm.ErrRecordNotFound {
			return err
		}

		// Create new locked record
		now := time.Now()
		lockUntil := now.Add(lockDuration)

		newRecord := IdempotencyKeyModel{
			Key:         key,
			RequestHash: requestHash,
			Locked:      true,
			LockedUntil: &lockUntil,
			CreatedAt:   now,
			ExpiresAt:   now.Add(s.config.TTL),
		}

		return tx.Create(&newRecord).Error
	})

	return result, err
}

// Unlock releases a lock and stores the final response
func (s *PostgresStore) Unlock(ctx context.Context, key string, response json.RawMessage, statusCode int) error {
	return s.db.WithContext(ctx).Model(&IdempotencyKeyModel{}).
		Where("key = ?", key).
		Updates(map[string]interface{}{
			"response":     response,
			"status_code":  statusCode,
			"locked":       false,
			"locked_until": nil,
		}).Error
}

// Delete removes an idempotency record
func (s *PostgresStore) Delete(ctx context.Context, key string) error {
	return s.db.WithContext(ctx).Delete(&IdempotencyKeyModel{}, "key = ?", key).Error
}

// Cleanup removes expired records
func (s *PostgresStore) Cleanup(ctx context.Context) (int64, error) {
	result := s.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&IdempotencyKeyModel{})

	return result.RowsAffected, result.Error
}

// startCleanup periodically cleans up expired records
func (s *PostgresStore) startCleanup() {
	ticker := time.NewTicker(s.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		s.Cleanup(context.Background())
	}
}

// modelToRecord converts a GORM model to a Record
func modelToRecord(m *IdempotencyKeyModel) *Record {
	return &Record{
		Key:          m.Key,
		RequestHash:  m.RequestHash,
		Response:     m.Response,
		StatusCode:   m.StatusCode,
		ProviderName: m.ProviderName,
		CreatedAt:    m.CreatedAt,
		ExpiresAt:    m.ExpiresAt,
		Locked:       m.Locked,
		LockedUntil:  m.LockedUntil,
	}
}

// recordToModel converts a Record to a GORM model
func recordToModel(r *Record) *IdempotencyKeyModel {
	return &IdempotencyKeyModel{
		Key:          r.Key,
		RequestHash:  r.RequestHash,
		Response:     r.Response,
		StatusCode:   r.StatusCode,
		ProviderName: r.ProviderName,
		Locked:       r.Locked,
		LockedUntil:  r.LockedUntil,
		CreatedAt:    r.CreatedAt,
		ExpiresAt:    r.ExpiresAt,
	}
}
