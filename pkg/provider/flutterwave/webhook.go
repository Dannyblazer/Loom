package flutterwave

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/loom-payments/loom/pkg/types"
)

// flutterwaveWebhookPayload represents a Flutterwave webhook payload
type flutterwaveWebhookPayload struct {
	Event     string          `json:"event"`
	Data      json.RawMessage `json:"data"`
	EventType string          `json:"event.type,omitempty"`
}

// flutterwaveWebhookChargeData represents charge data in webhook
type flutterwaveWebhookChargeData struct {
	ID                int64                  `json:"id"`
	TxRef             string                 `json:"tx_ref"`
	FlwRef            string                 `json:"flw_ref"`
	DeviceFingerprint string                 `json:"device_fingerprint"`
	Amount            float64                `json:"amount"`
	Currency          string                 `json:"currency"`
	ChargedAmount     float64                `json:"charged_amount"`
	AppFee            float64                `json:"app_fee"`
	MerchantFee       float64                `json:"merchant_fee"`
	ProcessorResponse string                 `json:"processor_response"`
	AuthModel         string                 `json:"auth_model"`
	IP                string                 `json:"ip"`
	Narration         string                 `json:"narration"`
	Status            string                 `json:"status"`
	PaymentType       string                 `json:"payment_type"`
	CreatedAt         time.Time              `json:"created_at"`
	AccountID         int64                  `json:"account_id"`
	Meta              map[string]interface{} `json:"meta"`
	Customer          flutterwaveCustomerResp `json:"customer"`
	Card              *flutterwaveCard       `json:"card"`
}

// flutterwaveWebhookTransferData represents transfer data in webhook
type flutterwaveWebhookTransferData struct {
	ID             int64                  `json:"id"`
	AccountNumber  string                 `json:"account_number"`
	BankCode       string                 `json:"bank_code"`
	BankName       string                 `json:"bank_name"`
	FullName       string                 `json:"full_name"`
	CreatedAt      time.Time              `json:"created_at"`
	Currency       string                 `json:"currency"`
	Amount         float64                `json:"amount"`
	Fee            float64                `json:"fee"`
	Status         string                 `json:"status"`
	Reference      string                 `json:"reference"`
	Narration      string                 `json:"narration"`
	CompleteMessage string                `json:"complete_message"`
	Meta           map[string]interface{} `json:"meta"`
}

// VerifyWebhookSignature verifies a Flutterwave webhook signature using SHA-256
func (f *Flutterwave) VerifyWebhookSignature(payload []byte, signature string) bool {
	secret := f.config.WebhookSecret
	if secret == "" {
		secret = f.config.SecretKey
	}

	// Flutterwave uses SHA-256 for webhook signatures
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// ParseWebhookEvent parses a Flutterwave webhook payload into a normalized event
func (f *Flutterwave) ParseWebhookEvent(payload []byte) (*types.WebhookEvent, error) {
	var raw flutterwaveWebhookPayload
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, types.ErrWebhookParseFailed
	}

	event := &types.WebhookEvent{
		Provider:          types.ProviderFlutterwave,
		ProviderEventType: raw.Event,
		ReceivedAt:        time.Now(),
		RawEvent:          raw,
	}

	// Map event type and parse data based on event type
	switch raw.Event {
	case "charge.completed":
		f.parseChargeWebhook(raw.Data, event)
		if event.Status == types.StatusSuccess {
			event.Type = types.EventChargeSuccess
		} else {
			event.Type = types.EventChargeFailed
		}

	case "transfer.completed":
		f.parseTransferWebhook(raw.Data, event)
		if event.Status == types.StatusSuccess {
			event.Type = types.EventTransferSuccess
		} else {
			event.Type = types.EventTransferFailed
		}

	case "virtualaccount.transfer":
		event.Type = types.EventVirtualAccountCredit
		event.Status = types.StatusSuccess
		f.parseVirtualAccountWebhook(raw.Data, event)

	case "subscription.cancelled":
		event.Type = types.EventSubscriptionDisabled
		event.Status = types.StatusCancelled

	default:
		event.Type = types.EventUnknown
	}

	return event, nil
}

// parseChargeWebhook parses charge-related webhook data
func (f *Flutterwave) parseChargeWebhook(data json.RawMessage, event *types.WebhookEvent) {
	var chargeData flutterwaveWebhookChargeData
	if err := json.Unmarshal(data, &chargeData); err != nil {
		return
	}

	event.ID = chargeData.TxRef
	event.Reference = chargeData.TxRef
	event.Amount = &types.Money{
		Amount:   int64(chargeData.Amount * 100),
		Currency: types.Currency(chargeData.Currency),
	}
	event.CustomerEmail = chargeData.Customer.Email
	event.CustomerID = chargeData.Customer.Email // Flutterwave uses email as identifier
	event.Metadata = chargeData.Meta
	event.Status = mapFlutterwaveStatus(chargeData.Status)

	// Set transaction details
	event.Transaction = &types.WebhookTransactionData{
		Reference:     chargeData.TxRef,
		Amount:        *event.Amount,
		Status:        event.Status,
		PaymentMethod: mapFlutterwavePaymentType(chargeData.PaymentType),
		FailureReason: chargeData.ProcessorResponse,
	}

	if chargeData.Status == "successful" {
		now := chargeData.CreatedAt
		event.Transaction.PaidAt = &now
	}

	if chargeData.Card != nil {
		event.Transaction.Card = &types.CardDetails{
			Last4:       chargeData.Card.Last4Digits,
			Brand:       chargeData.Card.Issuer,
			CardType:    chargeData.Card.Type,
			CountryCode: chargeData.Card.Country,
		}
	}
}

// parseTransferWebhook parses transfer-related webhook data
func (f *Flutterwave) parseTransferWebhook(data json.RawMessage, event *types.WebhookEvent) {
	var transferData flutterwaveWebhookTransferData
	if err := json.Unmarshal(data, &transferData); err != nil {
		return
	}

	event.ID = transferData.Reference
	event.Reference = transferData.Reference
	event.Amount = &types.Money{
		Amount:   int64(transferData.Amount * 100),
		Currency: types.Currency(transferData.Currency),
	}
	event.Metadata = transferData.Meta
	event.Status = mapFlutterwaveTransferStatus(transferData.Status)

	event.Transfer = &types.WebhookTransferData{
		Reference: transferData.Reference,
		Amount:    *event.Amount,
		Status:    event.Status,
		Recipient: &types.TransferRecipient{
			AccountNumber: transferData.AccountNumber,
			AccountName:   transferData.FullName,
			BankCode:      transferData.BankCode,
			BankName:      transferData.BankName,
		},
		FailureReason: transferData.CompleteMessage,
	}
}

// parseVirtualAccountWebhook parses virtual account credit webhook data
func (f *Flutterwave) parseVirtualAccountWebhook(data json.RawMessage, event *types.WebhookEvent) {
	// Virtual account credit has similar structure to charge
	var vaData struct {
		ID            int64   `json:"id"`
		Amount        float64 `json:"amount"`
		Currency      string  `json:"currency"`
		Reference     string  `json:"reference"`
		AccountNumber string  `json:"virtual_account_number"`
		SenderName    string  `json:"sender_name"`
		SenderBank    string  `json:"sender_bank"`
		SenderAccount string  `json:"sender_account_number"`
		SessionID     string  `json:"session_id"`
	}

	if err := json.Unmarshal(data, &vaData); err != nil {
		return
	}

	event.ID = vaData.Reference
	event.Reference = vaData.Reference
	event.Amount = &types.Money{
		Amount:   int64(vaData.Amount * 100),
		Currency: types.Currency(vaData.Currency),
	}

	event.VirtualAccount = &types.WebhookVirtualAccountData{
		AccountNumber: vaData.AccountNumber,
		SenderName:    vaData.SenderName,
		SenderBank:    vaData.SenderBank,
		SenderAccount: vaData.SenderAccount,
		SessionID:     vaData.SessionID,
	}
}
