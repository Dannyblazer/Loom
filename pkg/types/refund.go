package types

import "time"

// RefundRequest represents a unified refund request
type RefundRequest struct {
	IdempotencyKey       string   `json:"idempotency_key,omitempty"`
	TransactionReference string   `json:"transaction_reference"`
	Amount               *Money   `json:"amount,omitempty"` // nil for full refund
	Reason               string   `json:"reason,omitempty"`
	Metadata             Metadata `json:"metadata,omitempty"`

	// Provider preference (optional - uses original transaction's provider if empty)
	PreferredProvider ProviderName `json:"preferred_provider,omitempty"`
}

// Validate validates the refund request
func (r *RefundRequest) Validate() error {
	if r.TransactionReference == "" {
		return ErrMissingReference
	}
	if r.Amount != nil && r.Amount.Amount < 0 {
		return ErrInvalidAmount
	}
	return nil
}

// IsPartialRefund returns true if this is a partial refund
func (r *RefundRequest) IsPartialRefund() bool {
	return r.Amount != nil && r.Amount.Amount > 0
}

// RefundResponse represents a unified refund response
type RefundResponse struct {
	ID                   string            `json:"id"`
	Reference            string            `json:"reference"`
	TransactionReference string            `json:"transaction_reference"`
	Status               TransactionStatus `json:"status"`
	Amount               Money             `json:"amount"`
	Provider             ProviderName      `json:"provider"`
	ProviderReference    string            `json:"provider_reference"`
	Reason               string            `json:"reason,omitempty"`
	Metadata             Metadata          `json:"metadata,omitempty"`
	CreatedAt            time.Time         `json:"created_at"`
	CompletedAt          *time.Time        `json:"completed_at,omitempty"`
	FailureReason        string            `json:"failure_reason,omitempty"`
}

// ListRefundsRequest represents a request to list refunds
type ListRefundsRequest struct {
	Page                 int       `json:"page,omitempty"`
	PerPage              int       `json:"per_page,omitempty"`
	TransactionReference string    `json:"transaction_reference,omitempty"`
	From                 time.Time `json:"from,omitempty"`
	To                   time.Time `json:"to,omitempty"`
}

// ListRefundsResponse represents a list of refunds
type ListRefundsResponse struct {
	Refunds    []*RefundResponse `json:"refunds"`
	TotalCount int               `json:"total_count"`
	Page       int               `json:"page"`
	PerPage    int               `json:"per_page"`
}
