-- Loom Payment Orchestration Layer - Initial Schema
-- This migration creates the core tables for transaction tracking,
-- idempotency, webhook events, and reconciliation.

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================================================
-- Transactions Table
-- Stores all payment transactions (charges, refunds, transfers)
-- ============================================================================
CREATE TABLE IF NOT EXISTS loom_transactions (
    id BIGSERIAL PRIMARY KEY,
    reference VARCHAR(255) UNIQUE NOT NULL,
    idempotency_key VARCHAR(255) UNIQUE,

    -- Type and status
    type VARCHAR(50) NOT NULL, -- charge, refund, transfer, virtual_account_credit
    status VARCHAR(50) NOT NULL DEFAULT 'pending',

    -- Amount
    amount_minor BIGINT NOT NULL, -- Amount in smallest currency unit (kobo, cents)
    currency VARCHAR(10) NOT NULL DEFAULT 'NGN',
    fees_minor BIGINT DEFAULT 0,

    -- Provider info
    provider VARCHAR(50) NOT NULL,
    provider_reference VARCHAR(255),
    provider_response JSONB,

    -- Customer info
    customer_email VARCHAR(255),
    customer_id VARCHAR(255),

    -- Related transactions (for refunds)
    parent_transaction_id BIGINT REFERENCES loom_transactions(id),

    -- Metadata
    metadata JSONB,
    description TEXT,
    failure_reason TEXT,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

-- Indexes for transactions
CREATE INDEX IF NOT EXISTS idx_loom_transactions_reference ON loom_transactions(reference);
CREATE INDEX IF NOT EXISTS idx_loom_transactions_idempotency ON loom_transactions(idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_loom_transactions_provider ON loom_transactions(provider);
CREATE INDEX IF NOT EXISTS idx_loom_transactions_status ON loom_transactions(status);
CREATE INDEX IF NOT EXISTS idx_loom_transactions_type ON loom_transactions(type);
CREATE INDEX IF NOT EXISTS idx_loom_transactions_created ON loom_transactions(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_loom_transactions_customer ON loom_transactions(customer_email);
CREATE INDEX IF NOT EXISTS idx_loom_transactions_provider_ref ON loom_transactions(provider, provider_reference);

-- ============================================================================
-- Idempotency Keys Table
-- Stores idempotency records to prevent duplicate operations
-- ============================================================================
CREATE TABLE IF NOT EXISTS loom_idempotency_keys (
    key VARCHAR(255) PRIMARY KEY,
    request_hash VARCHAR(64) NOT NULL,
    response JSONB,
    status_code INT NOT NULL DEFAULT 0,
    provider_name VARCHAR(50),
    locked BOOLEAN NOT NULL DEFAULT FALSE,
    locked_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

-- Indexes for idempotency
CREATE INDEX IF NOT EXISTS idx_loom_idempotency_expires ON loom_idempotency_keys(expires_at);
CREATE INDEX IF NOT EXISTS idx_loom_idempotency_locked ON loom_idempotency_keys(locked, locked_until) WHERE locked = TRUE;

-- ============================================================================
-- Webhook Events Table
-- Stores webhook events for audit, replay, and debugging
-- ============================================================================
CREATE TABLE IF NOT EXISTS loom_webhook_events (
    id BIGSERIAL PRIMARY KEY,
    event_id VARCHAR(255),
    provider VARCHAR(50) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    reference VARCHAR(255),
    payload JSONB NOT NULL,
    signature VARCHAR(512),

    -- Processing status
    processed BOOLEAN DEFAULT FALSE,
    processed_at TIMESTAMPTZ,
    processing_error TEXT,
    retry_count INT DEFAULT 0,

    -- Linked transaction
    transaction_id BIGINT REFERENCES loom_transactions(id),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Prevent duplicate events
    UNIQUE(provider, event_id)
);

-- Indexes for webhook events
CREATE INDEX IF NOT EXISTS idx_loom_webhook_provider ON loom_webhook_events(provider);
CREATE INDEX IF NOT EXISTS idx_loom_webhook_event_type ON loom_webhook_events(event_type);
CREATE INDEX IF NOT EXISTS idx_loom_webhook_reference ON loom_webhook_events(reference);
CREATE INDEX IF NOT EXISTS idx_loom_webhook_processed ON loom_webhook_events(processed) WHERE processed = FALSE;
CREATE INDEX IF NOT EXISTS idx_loom_webhook_created ON loom_webhook_events(created_at DESC);

-- ============================================================================
-- Virtual Accounts Table
-- Tracks virtual accounts created across providers
-- ============================================================================
CREATE TABLE IF NOT EXISTS loom_virtual_accounts (
    id BIGSERIAL PRIMARY KEY,
    reference VARCHAR(255) UNIQUE NOT NULL,

    -- Provider info
    provider VARCHAR(50) NOT NULL,
    provider_account_id VARCHAR(255),

    -- Account details
    account_number VARCHAR(50) NOT NULL,
    account_name VARCHAR(255),
    bank_name VARCHAR(100),
    bank_code VARCHAR(20),

    -- Customer info
    customer_id VARCHAR(255),
    customer_email VARCHAR(255),

    -- Status
    is_active BOOLEAN DEFAULT TRUE,
    is_permanent BOOLEAN DEFAULT TRUE,
    currency VARCHAR(10) DEFAULT 'NGN',

    -- Metadata
    metadata JSONB,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    deactivated_at TIMESTAMPTZ
);

-- Indexes for virtual accounts
CREATE INDEX IF NOT EXISTS idx_loom_va_account_number ON loom_virtual_accounts(account_number);
CREATE INDEX IF NOT EXISTS idx_loom_va_customer ON loom_virtual_accounts(customer_id);
CREATE INDEX IF NOT EXISTS idx_loom_va_provider ON loom_virtual_accounts(provider);
CREATE INDEX IF NOT EXISTS idx_loom_va_active ON loom_virtual_accounts(is_active) WHERE is_active = TRUE;

-- ============================================================================
-- Reconciliation Records Table
-- Stores reconciliation reports
-- ============================================================================
CREATE TABLE IF NOT EXISTS loom_reconciliation_records (
    id BIGSERIAL PRIMARY KEY,
    provider VARCHAR(50) NOT NULL,
    reconciliation_date DATE NOT NULL,

    -- Summary
    total_transactions INT NOT NULL DEFAULT 0,
    total_amount_minor BIGINT NOT NULL DEFAULT 0,
    matched_count INT NOT NULL DEFAULT 0,
    mismatched_count INT NOT NULL DEFAULT 0,
    missing_local_count INT NOT NULL DEFAULT 0,
    missing_remote_count INT NOT NULL DEFAULT 0,

    -- Details
    discrepancies JSONB,

    -- Status
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    error_message TEXT,
    completed_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(provider, reconciliation_date)
);

-- ============================================================================
-- Provider Metrics Table
-- Tracks provider health and performance metrics
-- ============================================================================
CREATE TABLE IF NOT EXISTS loom_provider_metrics (
    id BIGSERIAL PRIMARY KEY,
    provider VARCHAR(50) NOT NULL,
    metric_time TIMESTAMPTZ NOT NULL,

    -- Metrics
    success_count INT NOT NULL DEFAULT 0,
    failure_count INT NOT NULL DEFAULT 0,
    avg_latency_ms INT,
    p95_latency_ms INT,
    p99_latency_ms INT,
    circuit_state VARCHAR(20),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for metrics queries
CREATE INDEX IF NOT EXISTS idx_loom_metrics_provider_time ON loom_provider_metrics(provider, metric_time DESC);

-- ============================================================================
-- Trigger for updating updated_at timestamps
-- ============================================================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply trigger to transactions table
DROP TRIGGER IF EXISTS update_loom_transactions_updated_at ON loom_transactions;
CREATE TRIGGER update_loom_transactions_updated_at
    BEFORE UPDATE ON loom_transactions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Apply trigger to virtual accounts table
DROP TRIGGER IF EXISTS update_loom_virtual_accounts_updated_at ON loom_virtual_accounts;
CREATE TRIGGER update_loom_virtual_accounts_updated_at
    BEFORE UPDATE ON loom_virtual_accounts
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- ============================================================================
-- Comments for documentation
-- ============================================================================
COMMENT ON TABLE loom_transactions IS 'Stores all payment transactions processed through Loom';
COMMENT ON TABLE loom_idempotency_keys IS 'Idempotency records to prevent duplicate operations';
COMMENT ON TABLE loom_webhook_events IS 'Webhook events received from payment providers';
COMMENT ON TABLE loom_virtual_accounts IS 'Virtual accounts created across providers';
COMMENT ON TABLE loom_reconciliation_records IS 'Reconciliation reports comparing local and provider records';
COMMENT ON TABLE loom_provider_metrics IS 'Provider health and performance metrics';
