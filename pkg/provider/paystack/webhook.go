package paystack

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/loom-payments/loom/pkg/types"
)

// paystackWebhookPayload represents a Paystack webhook payload
type paystackWebhookPayload struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

// paystackWebhookChargeData represents charge data in webhook
type paystackWebhookChargeData struct {
	ID              int64                  `json:"id"`
	Domain          string                 `json:"domain"`
	Status          string                 `json:"status"`
	Reference       string                 `json:"reference"`
	Amount          int64                  `json:"amount"`
	Currency        string                 `json:"currency"`
	Channel         string                 `json:"channel"`
	GatewayResponse string                 `json:"gateway_response"`
	IPAddress       string                 `json:"ip_address"`
	Fees            int64                  `json:"fees"`
	Metadata        map[string]interface{} `json:"metadata"`
	PaidAt          *time.Time             `json:"paid_at"`
	CreatedAt       time.Time              `json:"created_at"`
	Customer        paystackCustomer       `json:"customer"`
	Authorization   *paystackAuthorization `json:"authorization"`
}

// paystackWebhookTransferData represents transfer data in webhook
type paystackWebhookTransferData struct {
	ID            int64                  `json:"id"`
	TransferCode  string                 `json:"transfer_code"`
	Reference     string                 `json:"reference"`
	Status        string                 `json:"status"`
	Amount        int64                  `json:"amount"`
	Currency      string                 `json:"currency"`
	Reason        string                 `json:"reason"`
	Recipient     paystackRecipientData  `json:"recipient"`
	CreatedAt     time.Time              `json:"createdAt"`
	Metadata      map[string]interface{} `json:"metadata"`
	FailureReason string                 `json:"failures"`
}

// paystackWebhookDVAData represents dedicated virtual account webhook data
type paystackWebhookDVAData struct {
	ID                int64                  `json:"id"`
	AccountNumber     string                 `json:"account_number"`
	AccountName       string                 `json:"account_name"`
	BankName          string                 `json:"bank"`
	Amount            int64                  `json:"amount"`
	Currency          string                 `json:"currency"`
	Reference         string                 `json:"reference"`
	SessionID         string                 `json:"session_id"`
	SenderName        string                 `json:"sender_name"`
	SenderBank        string                 `json:"sender_bank"`
	SenderAccountNum  string                 `json:"sender_account_number"`
	Customer          paystackDVACustomer    `json:"customer"`
	Metadata          map[string]interface{} `json:"metadata"`
	CreatedAt         time.Time              `json:"created_at"`
}

// VerifyWebhookSignature verifies a Paystack webhook signature using HMAC-SHA512
func (p *Paystack) VerifyWebhookSignature(payload []byte, signature string) bool {
	secret := p.config.WebhookSecret
	if secret == "" {
		secret = p.config.SecretKey
	}

	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write(payload)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// ParseWebhookEvent parses a Paystack webhook payload into a normalized event
func (p *Paystack) ParseWebhookEvent(payload []byte) (*types.WebhookEvent, error) {
	var raw paystackWebhookPayload
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, types.ErrWebhookParseFailed
	}

	event := &types.WebhookEvent{
		Provider:          types.ProviderPaystack,
		ProviderEventType: raw.Event,
		ReceivedAt:        time.Now(),
		RawEvent:          raw,
	}

	// Map event type and parse data based on event type
	switch raw.Event {
	case "charge.success":
		event.Type = types.EventChargeSuccess
		event.Status = types.StatusSuccess
		p.parseChargeWebhook(raw.Data, event)

	case "charge.failed":
		event.Type = types.EventChargeFailed
		event.Status = types.StatusFailed
		p.parseChargeWebhook(raw.Data, event)

	case "transfer.success":
		event.Type = types.EventTransferSuccess
		event.Status = types.StatusSuccess
		p.parseTransferWebhook(raw.Data, event)

	case "transfer.failed":
		event.Type = types.EventTransferFailed
		event.Status = types.StatusFailed
		p.parseTransferWebhook(raw.Data, event)

	case "transfer.reversed":
		event.Type = types.EventTransferReversed
		event.Status = types.StatusRefunded
		p.parseTransferWebhook(raw.Data, event)

	case "dedicatedaccount.assign.success":
		event.Type = types.EventVirtualAccountCreated
		event.Status = types.StatusSuccess
		p.parseDVAAssignWebhook(raw.Data, event)

	case "dedicatedaccount.assign.failed":
		event.Type = types.EventVirtualAccountCreated
		event.Status = types.StatusFailed

	case "paymentrequest.success":
		event.Type = types.EventVirtualAccountCredit
		event.Status = types.StatusSuccess
		p.parseDVACreditWebhook(raw.Data, event)

	case "refund.processed":
		event.Type = types.EventRefundSuccess
		event.Status = types.StatusSuccess
		p.parseChargeWebhook(raw.Data, event) // Refund data similar to charge

	case "refund.failed":
		event.Type = types.EventRefundFailed
		event.Status = types.StatusFailed

	case "subscription.create":
		event.Type = types.EventSubscriptionCreated
		event.Status = types.StatusSuccess

	case "subscription.disable":
		event.Type = types.EventSubscriptionDisabled
		event.Status = types.StatusCancelled

	case "invoice.payment_failed":
		event.Type = types.EventSubscriptionChargeFailed
		event.Status = types.StatusFailed

	default:
		event.Type = types.EventUnknown
	}

	return event, nil
}

// parseChargeWebhook parses charge-related webhook data
func (p *Paystack) parseChargeWebhook(data json.RawMessage, event *types.WebhookEvent) {
	var chargeData paystackWebhookChargeData
	if err := json.Unmarshal(data, &chargeData); err != nil {
		return
	}

	event.ID = chargeData.Reference
	event.Reference = chargeData.Reference
	event.Amount = &types.Money{
		Amount:   chargeData.Amount,
		Currency: types.Currency(chargeData.Currency),
	}
	event.CustomerEmail = chargeData.Customer.Email
	event.CustomerID = chargeData.Customer.CustomerCode
	event.Metadata = chargeData.Metadata

	// Set transaction details
	event.Transaction = &types.WebhookTransactionData{
		Reference:     chargeData.Reference,
		Amount:        *event.Amount,
		Status:        mapPaystackStatus(chargeData.Status),
		PaidAt:        chargeData.PaidAt,
		PaymentMethod: mapPaystackChannel(chargeData.Channel),
		FailureReason: chargeData.GatewayResponse,
	}

	if chargeData.Authorization != nil {
		event.Transaction.Card = &types.CardDetails{
			Last4:       chargeData.Authorization.Last4,
			Brand:       chargeData.Authorization.Brand,
			ExpMonth:    chargeData.Authorization.ExpMonth,
			ExpYear:     chargeData.Authorization.ExpYear,
			Bank:        chargeData.Authorization.Bank,
			CountryCode: chargeData.Authorization.CountryCode,
		}
		event.Transaction.Authorization = &types.Authorization{
			AuthorizationCode: chargeData.Authorization.AuthorizationCode,
			Reusable:          chargeData.Authorization.Reusable,
			CardDetails:       event.Transaction.Card,
		}
	}
}

// parseTransferWebhook parses transfer-related webhook data
func (p *Paystack) parseTransferWebhook(data json.RawMessage, event *types.WebhookEvent) {
	var transferData paystackWebhookTransferData
	if err := json.Unmarshal(data, &transferData); err != nil {
		return
	}

	event.ID = transferData.Reference
	event.Reference = transferData.Reference
	event.Amount = &types.Money{
		Amount:   transferData.Amount,
		Currency: types.Currency(transferData.Currency),
	}
	event.Metadata = transferData.Metadata

	event.Transfer = &types.WebhookTransferData{
		Reference: transferData.Reference,
		Amount:    *event.Amount,
		Status:    mapPaystackTransferStatus(transferData.Status),
		Recipient: &types.TransferRecipient{
			RecipientCode: transferData.Recipient.RecipientCode,
			AccountNumber: transferData.Recipient.Details.AccountNumber,
			AccountName:   transferData.Recipient.Details.AccountName,
			BankCode:      transferData.Recipient.Details.BankCode,
			BankName:      transferData.Recipient.Details.BankName,
		},
		FailureReason: transferData.FailureReason,
	}
}

// parseDVACreditWebhook parses DVA credit webhook data
func (p *Paystack) parseDVACreditWebhook(data json.RawMessage, event *types.WebhookEvent) {
	var dvaData paystackWebhookDVAData
	if err := json.Unmarshal(data, &dvaData); err != nil {
		return
	}

	event.ID = dvaData.Reference
	event.Reference = dvaData.Reference
	event.Amount = &types.Money{
		Amount:   dvaData.Amount,
		Currency: types.Currency(dvaData.Currency),
	}
	event.CustomerEmail = dvaData.Customer.Email
	event.Metadata = dvaData.Metadata

	event.VirtualAccount = &types.WebhookVirtualAccountData{
		AccountNumber: dvaData.AccountNumber,
		AccountName:   dvaData.AccountName,
		BankName:      dvaData.BankName,
		SenderName:    dvaData.SenderName,
		SenderBank:    dvaData.SenderBank,
		SenderAccount: dvaData.SenderAccountNum,
		SessionID:     dvaData.SessionID,
	}
}

// parseDVAAssignWebhook parses DVA assignment webhook data
func (p *Paystack) parseDVAAssignWebhook(data json.RawMessage, event *types.WebhookEvent) {
	var dvaData paystackDVAResponse
	if err := json.Unmarshal(data, &dvaData); err != nil {
		return
	}

	event.ID = dvaData.AccountNumber
	event.Reference = dvaData.AccountNumber
	event.CustomerEmail = dvaData.Customer.Email

	event.VirtualAccount = &types.WebhookVirtualAccountData{
		AccountNumber: dvaData.AccountNumber,
		AccountName:   dvaData.AccountName,
		BankName:      dvaData.Bank.Name,
	}
}
