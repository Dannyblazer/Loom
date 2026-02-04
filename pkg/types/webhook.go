package types

import "time"

// WebhookEventType represents normalized webhook event types
type WebhookEventType string

const (
	// Charge events
	EventChargeSuccess   WebhookEventType = "charge.success"
	EventChargeFailed    WebhookEventType = "charge.failed"
	EventChargePending   WebhookEventType = "charge.pending"
	EventChargeDisputed  WebhookEventType = "charge.disputed"

	// Refund events
	EventRefundSuccess WebhookEventType = "refund.success"
	EventRefundFailed  WebhookEventType = "refund.failed"
	EventRefundPending WebhookEventType = "refund.pending"

	// Transfer events
	EventTransferSuccess WebhookEventType = "transfer.success"
	EventTransferFailed  WebhookEventType = "transfer.failed"
	EventTransferPending WebhookEventType = "transfer.pending"
	EventTransferReversed WebhookEventType = "transfer.reversed"

	// Virtual account events
	EventVirtualAccountCredit  WebhookEventType = "virtual_account.credit"
	EventVirtualAccountCreated WebhookEventType = "virtual_account.created"
	EventVirtualAccountExpired WebhookEventType = "virtual_account.expired"

	// Subscription events
	EventSubscriptionCreated   WebhookEventType = "subscription.created"
	EventSubscriptionDisabled  WebhookEventType = "subscription.disabled"
	EventSubscriptionChargeSuccess WebhookEventType = "subscription.charge.success"
	EventSubscriptionChargeFailed  WebhookEventType = "subscription.charge.failed"

	// Unknown event
	EventUnknown WebhookEventType = "unknown"
)

// WebhookEvent represents a normalized webhook event
type WebhookEvent struct {
	ID            string            `json:"id"`
	Type          WebhookEventType  `json:"type"`
	Provider      ProviderName      `json:"provider"`
	Reference     string            `json:"reference"`
	Amount        *Money            `json:"amount,omitempty"`
	Status        TransactionStatus `json:"status"`
	CustomerEmail string            `json:"customer_email,omitempty"`
	CustomerID    string            `json:"customer_id,omitempty"`
	Metadata      Metadata          `json:"metadata,omitempty"`
	ReceivedAt    time.Time         `json:"received_at"`

	// Original provider event type
	ProviderEventType string `json:"provider_event_type,omitempty"`

	// Raw event payload from provider
	RawEvent interface{} `json:"raw_event,omitempty"`

	// Transaction details if available
	Transaction *WebhookTransactionData `json:"transaction,omitempty"`

	// Virtual account details for VA credits
	VirtualAccount *WebhookVirtualAccountData `json:"virtual_account,omitempty"`

	// Transfer details
	Transfer *WebhookTransferData `json:"transfer,omitempty"`
}

// WebhookTransactionData contains transaction data from webhooks
type WebhookTransactionData struct {
	Reference         string            `json:"reference"`
	Amount            Money             `json:"amount"`
	Status            TransactionStatus `json:"status"`
	PaidAt            *time.Time        `json:"paid_at,omitempty"`
	PaymentMethod     PaymentMethodType `json:"payment_method,omitempty"`
	Card              *CardDetails      `json:"card,omitempty"`
	Authorization     *Authorization    `json:"authorization,omitempty"`
	FailureReason     string            `json:"failure_reason,omitempty"`
}

// WebhookVirtualAccountData contains virtual account data from webhooks
type WebhookVirtualAccountData struct {
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
	BankName      string `json:"bank_name"`
	SenderName    string `json:"sender_name,omitempty"`
	SenderBank    string `json:"sender_bank,omitempty"`
	SenderAccount string `json:"sender_account,omitempty"`
	SessionID     string `json:"session_id,omitempty"`
}

// WebhookTransferData contains transfer data from webhooks
type WebhookTransferData struct {
	Reference     string             `json:"reference"`
	Amount        Money              `json:"amount"`
	Status        TransactionStatus  `json:"status"`
	Recipient     *TransferRecipient `json:"recipient,omitempty"`
	FailureReason string             `json:"failure_reason,omitempty"`
}

// WebhookHandler defines the interface for handling webhook events
type WebhookHandler interface {
	HandleEvent(event *WebhookEvent) error
}

// WebhookHandlerFunc is a function type that implements WebhookHandler
type WebhookHandlerFunc func(event *WebhookEvent) error

func (f WebhookHandlerFunc) HandleEvent(event *WebhookEvent) error {
	return f(event)
}

// WebhookConfig contains webhook configuration
type WebhookConfig struct {
	// Secret for verifying webhook signatures
	Secret string

	// Handlers for different event types
	Handlers map[WebhookEventType]WebhookHandler

	// Default handler for unhandled events
	DefaultHandler WebhookHandler

	// Whether to store raw events for replay
	StoreEvents bool

	// Event TTL for stored events
	EventTTL time.Duration
}
