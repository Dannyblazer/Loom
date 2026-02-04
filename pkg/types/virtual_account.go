package types

import "time"

// VirtualAccountRequest represents a request to create a virtual account
type VirtualAccountRequest struct {
	IdempotencyKey string   `json:"idempotency_key,omitempty"`
	Customer       *CustomerInfo `json:"customer"`
	Reference      string   `json:"reference,omitempty"`
	Currency       Currency `json:"currency,omitempty"`
	Metadata       Metadata `json:"metadata,omitempty"`

	// BVN or other identity verification (required by some providers)
	BVN string `json:"bvn,omitempty"`
	NIN string `json:"nin,omitempty"`

	// Preferred bank for the virtual account
	PreferredBank string `json:"preferred_bank,omitempty"`

	// Provider preference (optional)
	PreferredProvider ProviderName `json:"preferred_provider,omitempty"`

	// Permanent or temporary account
	IsPermanent bool `json:"is_permanent,omitempty"`

	// For temporary accounts - expiry settings
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	// Expected amount for one-time accounts
	ExpectedAmount *Money `json:"expected_amount,omitempty"`
}

// Validate validates the virtual account request
func (r *VirtualAccountRequest) Validate() error {
	if r.Customer == nil || r.Customer.Email == "" {
		return ErrMissingEmail
	}
	return nil
}

// VirtualAccountResponse represents a created virtual account
type VirtualAccountResponse struct {
	ID                string       `json:"id"`
	Reference         string       `json:"reference"`
	AccountNumber     string       `json:"account_number"`
	AccountName       string       `json:"account_name"`
	BankName          string       `json:"bank_name"`
	BankCode          string       `json:"bank_code"`
	Currency          Currency     `json:"currency"`
	Provider          ProviderName `json:"provider"`
	ProviderAccountID string       `json:"provider_account_id"`
	Customer          *CustomerInfo `json:"customer,omitempty"`
	IsActive          bool         `json:"is_active"`
	IsPermanent       bool         `json:"is_permanent"`
	Metadata          Metadata     `json:"metadata,omitempty"`
	CreatedAt         time.Time    `json:"created_at"`
	ExpiresAt         *time.Time   `json:"expires_at,omitempty"`
}

// VirtualAccountTransaction represents a transaction on a virtual account
type VirtualAccountTransaction struct {
	ID              string            `json:"id"`
	Reference       string            `json:"reference"`
	AccountNumber   string            `json:"account_number"`
	Amount          Money             `json:"amount"`
	Status          TransactionStatus `json:"status"`
	SenderName      string            `json:"sender_name,omitempty"`
	SenderBank      string            `json:"sender_bank,omitempty"`
	SenderAccount   string            `json:"sender_account,omitempty"`
	Narration       string            `json:"narration,omitempty"`
	SessionID       string            `json:"session_id,omitempty"`
	Provider        ProviderName      `json:"provider"`
	CreatedAt       time.Time         `json:"created_at"`
}

// ListVirtualAccountsRequest represents a request to list virtual accounts
type ListVirtualAccountsRequest struct {
	Page       int    `json:"page,omitempty"`
	PerPage    int    `json:"per_page,omitempty"`
	CustomerID string `json:"customer_id,omitempty"`
	IsActive   *bool  `json:"is_active,omitempty"`
}

// ListVirtualAccountsResponse represents a list of virtual accounts
type ListVirtualAccountsResponse struct {
	Accounts   []*VirtualAccountResponse `json:"accounts"`
	TotalCount int                       `json:"total_count"`
	Page       int                       `json:"page"`
	PerPage    int                       `json:"per_page"`
}

// DeactivateVirtualAccountRequest represents a request to deactivate a virtual account
type DeactivateVirtualAccountRequest struct {
	AccountID string `json:"account_id"`
	Reference string `json:"reference,omitempty"`
}
