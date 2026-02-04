package types

import "time"

// TransferRequest represents a unified transfer request
type TransferRequest struct {
	IdempotencyKey string   `json:"idempotency_key,omitempty"`
	Amount         Money    `json:"amount"`
	Reference      string   `json:"reference,omitempty"`
	Reason         string   `json:"reason,omitempty"`
	Metadata       Metadata `json:"metadata,omitempty"`

	// Recipient details
	Recipient *TransferRecipient `json:"recipient"`

	// Provider preference (optional)
	PreferredProvider ProviderName `json:"preferred_provider,omitempty"`
}

// TransferRecipient contains recipient details for a transfer
type TransferRecipient struct {
	// For bank transfers
	AccountNumber string `json:"account_number,omitempty"`
	AccountName   string `json:"account_name,omitempty"`
	BankCode      string `json:"bank_code,omitempty"`
	BankName      string `json:"bank_name,omitempty"`

	// Recipient code (if previously created)
	RecipientCode string `json:"recipient_code,omitempty"`

	// For mobile money
	Phone    string `json:"phone,omitempty"`
	Provider string `json:"provider,omitempty"` // MTN, Vodafone, etc.
}

// Validate validates the transfer request
func (r *TransferRequest) Validate() error {
	if r.Amount.IsZero() {
		return ErrInvalidAmount
	}
	if r.Amount.Currency == "" {
		return ErrMissingCurrency
	}
	if r.Recipient == nil {
		return ErrMissingRecipient
	}
	if r.Recipient.RecipientCode == "" {
		if r.Recipient.AccountNumber == "" {
			return ErrMissingAccountNum
		}
		if r.Recipient.BankCode == "" {
			return ErrMissingBankCode
		}
	}
	return nil
}

// TransferResponse represents a unified transfer response
type TransferResponse struct {
	ID                string            `json:"id"`
	Reference         string            `json:"reference"`
	Status            TransactionStatus `json:"status"`
	Amount            Money             `json:"amount"`
	Fees              *Money            `json:"fees,omitempty"`
	Provider          ProviderName      `json:"provider"`
	ProviderReference string            `json:"provider_reference"`
	Recipient         *TransferRecipient `json:"recipient,omitempty"`
	Reason            string            `json:"reason,omitempty"`
	Metadata          Metadata          `json:"metadata,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	CompletedAt       *time.Time        `json:"completed_at,omitempty"`
	FailureReason     string            `json:"failure_reason,omitempty"`
}

// CreateRecipientRequest represents a request to create a transfer recipient
type CreateRecipientRequest struct {
	Type          string `json:"type"` // nuban, mobile_money, etc.
	Name          string `json:"name"`
	AccountNumber string `json:"account_number"`
	BankCode      string `json:"bank_code"`
	Currency      string `json:"currency,omitempty"`
	Metadata      Metadata `json:"metadata,omitempty"`
}

// RecipientResponse represents a created recipient
type RecipientResponse struct {
	RecipientCode string `json:"recipient_code"`
	Name          string `json:"name"`
	AccountNumber string `json:"account_number"`
	BankCode      string `json:"bank_code"`
	BankName      string `json:"bank_name"`
	Currency      string `json:"currency"`
	IsActive      bool   `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
}

// ListTransfersRequest represents a request to list transfers
type ListTransfersRequest struct {
	Page      int       `json:"page,omitempty"`
	PerPage   int       `json:"per_page,omitempty"`
	From      time.Time `json:"from,omitempty"`
	To        time.Time `json:"to,omitempty"`
	Status    string    `json:"status,omitempty"`
	Recipient string    `json:"recipient,omitempty"`
}

// ListTransfersResponse represents a list of transfers
type ListTransfersResponse struct {
	Transfers  []*TransferResponse `json:"transfers"`
	TotalCount int                 `json:"total_count"`
	Page       int                 `json:"page"`
	PerPage    int                 `json:"per_page"`
}

// ResolveAccountRequest represents a request to resolve a bank account
type ResolveAccountRequest struct {
	AccountNumber string `json:"account_number"`
	BankCode      string `json:"bank_code"`
}

// ListBanksRequest represents a request to list banks
type ListBanksRequest struct {
	Country  string `json:"country,omitempty"`
	Currency string `json:"currency,omitempty"`
	Type     string `json:"type,omitempty"` // nuban, mobile_money
}
