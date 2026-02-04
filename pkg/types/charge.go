package types

import "time"

// ChargeRequest represents a unified charge request
type ChargeRequest struct {
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	Amount         Money          `json:"amount"`
	Email          string         `json:"email"`
	Description    string         `json:"description,omitempty"`
	Reference      string         `json:"reference,omitempty"`
	CallbackURL    string         `json:"callback_url,omitempty"`
	Metadata       Metadata       `json:"metadata,omitempty"`
	Customer       *CustomerInfo  `json:"customer,omitempty"`
	PaymentMethod  *PaymentMethod `json:"payment_method,omitempty"`

	// Provider preference (optional - orchestrator decides if empty)
	PreferredProvider ProviderName `json:"preferred_provider,omitempty"`

	// Split payment configuration
	SplitCode string `json:"split_code,omitempty"`

	// Channels to allow (card, bank, ussd, transfer, etc.)
	Channels []string `json:"channels,omitempty"`
}

// Validate validates the charge request
func (r *ChargeRequest) Validate() error {
	if r.Amount.IsZero() {
		return ErrInvalidAmount
	}
	if r.Email == "" {
		return ErrMissingEmail
	}
	if r.Amount.Currency == "" {
		return ErrMissingCurrency
	}
	return nil
}

// ChargeResponse represents a unified charge response
type ChargeResponse struct {
	ID                string            `json:"id"`
	Reference         string            `json:"reference"`
	Status            TransactionStatus `json:"status"`
	Amount            Money             `json:"amount"`
	Fees              *Money            `json:"fees,omitempty"`
	AuthorizationURL  string            `json:"authorization_url,omitempty"`
	AccessCode        string            `json:"access_code,omitempty"`
	Provider          ProviderName      `json:"provider"`
	ProviderReference string            `json:"provider_reference"`
	PaymentMethod     PaymentMethodType `json:"payment_method,omitempty"`
	Customer          *CustomerInfo     `json:"customer,omitempty"`
	Metadata          Metadata          `json:"metadata,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	PaidAt            *time.Time        `json:"paid_at,omitempty"`
	FailureReason     string            `json:"failure_reason,omitempty"`

	// Card details (if paid with card)
	Card *CardDetails `json:"card,omitempty"`

	// Authorization for recurring charges
	Authorization *Authorization `json:"authorization,omitempty"`
}

// CardDetails contains card information from a charge
type CardDetails struct {
	Last4       string `json:"last4"`
	Brand       string `json:"brand"`
	ExpMonth    string `json:"exp_month"`
	ExpYear     string `json:"exp_year"`
	Bank        string `json:"bank,omitempty"`
	CountryCode string `json:"country_code,omitempty"`
	CardType    string `json:"card_type,omitempty"`
}

// Authorization contains reusable authorization details for recurring charges
type Authorization struct {
	AuthorizationCode string `json:"authorization_code"`
	Reusable          bool   `json:"reusable"`
	CardDetails       *CardDetails `json:"card,omitempty"`
}

// ChargeVerifyRequest represents a request to verify a charge
type ChargeVerifyRequest struct {
	Reference string `json:"reference"`
}

// ListChargesRequest represents a request to list charges
type ListChargesRequest struct {
	Page     int       `json:"page,omitempty"`
	PerPage  int       `json:"per_page,omitempty"`
	From     time.Time `json:"from,omitempty"`
	To       time.Time `json:"to,omitempty"`
	Status   string    `json:"status,omitempty"`
	Customer string    `json:"customer,omitempty"`
}

// ListChargesResponse represents a list of charges
type ListChargesResponse struct {
	Charges    []*ChargeResponse `json:"charges"`
	TotalCount int               `json:"total_count"`
	Page       int               `json:"page"`
	PerPage    int               `json:"per_page"`
}
