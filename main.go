package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/loom-payments/loom/config"
	"github.com/loom-payments/loom/internal/http"
	"github.com/loom-payments/loom/pkg/idempotency"
	"github.com/loom-payments/loom/pkg/loom"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Starting Loom Payment Orchestration Layer")
	log.Printf("Environment: %s", cfg.Server.Environment)

	// Create Loom client
	client, err := loom.NewClientFromConfig(cfg)
	if err != nil {
		log.Fatalf("Failed to create Loom client: %v", err)
	}

	// Setup idempotency store
	var idempStore idempotency.Store
	if cfg.Database.URL != "" {
		// Use PostgreSQL store
		db, err := setupDatabase(cfg)
		if err != nil {
			log.Fatalf("Failed to setup database: %v", err)
		}

		idempStore, err = idempotency.NewPostgresStore(db, &idempotency.Config{
			TTL:             cfg.Idempotency.TTL,
			LockDuration:    cfg.Idempotency.LockDuration,
			CleanupInterval: cfg.Idempotency.CleanupInterval,
		})
		if err != nil {
			log.Fatalf("Failed to create idempotency store: %v", err)
		}
		log.Printf("Using PostgreSQL idempotency store")
	} else {
		// Use in-memory store
		idempStore = idempotency.NewMemoryStore(&idempotency.Config{
			TTL:             cfg.Idempotency.TTL,
			LockDuration:    cfg.Idempotency.LockDuration,
			CleanupInterval: cfg.Idempotency.CleanupInterval,
		})
		log.Printf("Using in-memory idempotency store (not recommended for production)")
	}

	// Create and start HTTP server
	server := http.NewServer(cfg, client, idempStore)

	// Start server in goroutine
	go func() {
		if err := server.Start(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Print startup info
	log.Printf("Server listening on port %s", cfg.Server.Port)
	log.Printf("Provider priority: %v", cfg.Orchestration.ProviderPriority)
	log.Printf("Failover enabled: %v", cfg.Orchestration.FailoverEnabled)

	// Check provider health on startup
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	healthResults := client.HealthCheck(ctx)
	for provider, err := range healthResults {
		if err != nil {
			log.Printf("Provider %s: UNHEALTHY (%v)", provider, err)
		} else {
			log.Printf("Provider %s: HEALTHY", provider)
		}
	}

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func setupDatabase(cfg *config.Config) (*gorm.DB, error) {
	// Configure GORM logger
	gormLogger := logger.Default.LogMode(logger.Silent)
	if cfg.IsDevelopment() {
		gormLogger = logger.Default.LogMode(logger.Info)
	}

	db, err := gorm.Open(postgres.Open(cfg.Database.URL), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.Database.ConnMaxIdleTime)

	return db, nil
}
