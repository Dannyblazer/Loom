package types

import "errors"

// Validation errors
var (
	ErrInvalidAmount     = errors.New("invalid amount: must be greater than zero")
	ErrMissingEmail      = errors.New("email is required")
	ErrMissingCurrency   = errors.New("currency is required")
	ErrMissingReference  = errors.New("reference is required")
	ErrMissingBankCode   = errors.New("bank code is required")
	ErrMissingAccountNum = errors.New("account number is required")
	ErrMissingRecipient  = errors.New("recipient is required")
	ErrInvalidBVN        = errors.New("invalid BVN: must be 11 digits")
)

// Provider errors
var (
	ErrProviderNotFound     = errors.New("provider not found")
	ErrProviderUnavailable  = errors.New("provider temporarily unavailable")
	ErrAllProvidersFailed   = errors.New("all providers failed")
	ErrNoProvidersAvailable = errors.New("no providers available")
	ErrUnsupportedCurrency  = errors.New("currency not supported by provider")
	ErrUnsupportedFeature   = errors.New("feature not supported by provider")
)

// Transaction errors
var (
	ErrTransactionNotFound   = errors.New("transaction not found")
	ErrTransactionFailed     = errors.New("transaction failed")
	ErrDuplicateTransaction  = errors.New("duplicate transaction")
	ErrRefundExceedsAmount   = errors.New("refund amount exceeds original transaction")
	ErrTransactionNotRefundable = errors.New("transaction cannot be refunded")
)

// Webhook errors
var (
	ErrInvalidSignature    = errors.New("invalid webhook signature")
	ErrWebhookParseFailed  = errors.New("failed to parse webhook payload")
	ErrUnknownEventType    = errors.New("unknown webhook event type")
)

// Idempotency errors
var (
	ErrIdempotencyConflict = errors.New("idempotency key already used with different request")
)

// ProviderError wraps errors from payment providers with additional context
type ProviderError struct {
	Provider    ProviderName
	Code        string
	Message     string
	RawResponse interface{}
	Err         error
}

func (e *ProviderError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "provider error"
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

// NewProviderError creates a new provider error
func NewProviderError(provider ProviderName, code, message string, err error) *ProviderError {
	return &ProviderError{
		Provider: provider,
		Code:     code,
		Message:  message,
		Err:      err,
	}
}

// FailoverError contains errors from multiple providers during failover
type FailoverError struct {
	Errors []error
}

func (e *FailoverError) Error() string {
	if len(e.Errors) == 0 {
		return "failover failed with no errors recorded"
	}
	return "all providers failed: " + e.Errors[len(e.Errors)-1].Error()
}

// NewFailoverError creates a new failover error
func NewFailoverError(errors []error) *FailoverError {
	return &FailoverError{Errors: errors}
}
