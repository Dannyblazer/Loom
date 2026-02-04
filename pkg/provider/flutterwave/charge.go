package flutterwave

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/loom-payments/loom/pkg/types"
)

// flutterwavePaymentRequest represents a Flutterwave payment initialization request
type flutterwavePaymentRequest struct {
	TxRef          string                 `json:"tx_ref"`
	Amount         float64                `json:"amount"`
	Currency       string                 `json:"currency"`
	RedirectURL    string                 `json:"redirect_url,omitempty"`
	PaymentOptions string                 `json:"payment_options,omitempty"`
	Meta           map[string]interface{} `json:"meta,omitempty"`
	Customer       flutterwaveCustomer    `json:"customer"`
	Customizations *flutterwaveCustom     `json:"customizations,omitempty"`
}

type flutterwaveCustomer struct {
	Email       string `json:"email"`
	PhoneNumber string `json:"phonenumber,omitempty"`
	Name        string `json:"name,omitempty"`
}

type flutterwaveCustom struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Logo        string `json:"logo,omitempty"`
}

// flutterwavePaymentResponse represents a Flutterwave payment initialization response
type flutterwavePaymentResponse struct {
	Link string `json:"link"`
}

// flutterwaveVerifyResponse represents a Flutterwave transaction verification response
type flutterwaveVerifyResponse struct {
	ID              int64                  `json:"id"`
	TxRef           string                 `json:"tx_ref"`
	FlwRef          string                 `json:"flw_ref"`
	DeviceFingerprint string              `json:"device_fingerprint"`
	Amount          float64                `json:"amount"`
	Currency        string                 `json:"currency"`
	ChargedAmount   float64                `json:"charged_amount"`
	AppFee          float64                `json:"app_fee"`
	MerchantFee     float64                `json:"merchant_fee"`
	ProcessorResponse string              `json:"processor_response"`
	AuthModel       string                 `json:"auth_model"`
	IP              string                 `json:"ip"`
	Narration       string                 `json:"narration"`
	Status          string                 `json:"status"`
	PaymentType     string                 `json:"payment_type"`
	CreatedAt       time.Time              `json:"created_at"`
	AccountID       int64                  `json:"account_id"`
	Meta            map[string]interface{} `json:"meta"`
	Customer        flutterwaveCustomerResp `json:"customer"`
	Card            *flutterwaveCard       `json:"card"`
}

type flutterwaveCustomerResp struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	PhoneNumber string `json:"phone_number"`
	Email       string `json:"email"`
	CreatedAt   time.Time `json:"created_at"`
}

type flutterwaveCard struct {
	First6Digits string `json:"first_6digits"`
	Last4Digits  string `json:"last_4digits"`
	Issuer       string `json:"issuer"`
	Country      string `json:"country"`
	Type         string `json:"type"`
	Expiry       string `json:"expiry"`
}

// InitializeCharge initializes a charge transaction with Flutterwave
func (f *Flutterwave) InitializeCharge(ctx context.Context, req *types.ChargeRequest) (*types.ChargeResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	flwReq := flutterwavePaymentRequest{
		TxRef:       req.Reference,
		Amount:      req.Amount.ToMajorUnits(), // Flutterwave uses major units
		Currency:    string(req.Amount.Currency),
		RedirectURL: req.CallbackURL,
		Meta:        req.Metadata,
		Customer: flutterwaveCustomer{
			Email: req.Email,
		},
	}

	if req.Customer != nil {
		flwReq.Customer.Name = req.Customer.FullName()
		flwReq.Customer.PhoneNumber = req.Customer.Phone
	}

	if req.Description != "" {
		flwReq.Customizations = &flutterwaveCustom{
			Description: req.Description,
		}
	}

	// Map channels to payment options
	if len(req.Channels) > 0 {
		flwReq.PaymentOptions = mapChannelsToPaymentOptions(req.Channels)
	}

	respBody, err := f.makeRequest(ctx, http.MethodPost, "/payments", flwReq)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize charge: %w", err)
	}

	data, err := parseResponse[flutterwavePaymentResponse](respBody)
	if err != nil {
		return nil, err
	}

	return &types.ChargeResponse{
		Reference:         req.Reference,
		Status:            types.StatusPending,
		Amount:            req.Amount,
		AuthorizationURL:  data.Link,
		Provider:          types.ProviderFlutterwave,
		ProviderReference: req.Reference,
		CreatedAt:         time.Now(),
		Metadata:          req.Metadata,
	}, nil
}

// VerifyCharge verifies a charge transaction with Flutterwave
func (f *Flutterwave) VerifyCharge(ctx context.Context, reference string) (*types.ChargeResponse, error) {
	if reference == "" {
		return nil, types.ErrMissingReference
	}

	// Flutterwave requires transaction ID for verification, but we can use tx_ref
	respBody, err := f.makeRequest(ctx, http.MethodGet, "/transactions/verify_by_reference?tx_ref="+reference, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to verify charge: %w", err)
	}

	data, err := parseResponse[flutterwaveVerifyResponse](respBody)
	if err != nil {
		return nil, err
	}

	response := &types.ChargeResponse{
		ID:                fmt.Sprintf("%d", data.ID),
		Reference:         data.TxRef,
		Status:            mapFlutterwaveStatus(data.Status),
		Amount:            types.FromMajorUnits(data.Amount, types.Currency(data.Currency)),
		Provider:          types.ProviderFlutterwave,
		ProviderReference: data.FlwRef,
		PaymentMethod:     mapFlutterwavePaymentType(data.PaymentType),
		CreatedAt:         data.CreatedAt,
		Metadata:          data.Meta,
		FailureReason:     data.ProcessorResponse,
	}

	if data.AppFee > 0 {
		response.Fees = &types.Money{
			Amount:   int64(data.AppFee * 100),
			Currency: types.Currency(data.Currency),
		}
	}

	// Map customer info
	response.Customer = &types.CustomerInfo{
		ID:    fmt.Sprintf("%d", data.Customer.ID),
		Email: data.Customer.Email,
		Phone: data.Customer.PhoneNumber,
	}

	// Parse name if available
	if data.Customer.Name != "" {
		response.Customer.FirstName = data.Customer.Name
	}

	// Map card details
	if data.Card != nil && data.Card.Last4Digits != "" {
		response.Card = &types.CardDetails{
			Last4:       data.Card.Last4Digits,
			Brand:       data.Card.Issuer,
			CardType:    data.Card.Type,
			CountryCode: data.Card.Country,
		}
		// Parse expiry if available
		if data.Card.Expiry != "" {
			// Expiry format is usually MM/YY
			if len(data.Card.Expiry) >= 5 {
				response.Card.ExpMonth = data.Card.Expiry[:2]
				response.Card.ExpYear = data.Card.Expiry[3:]
			}
		}
	}

	return response, nil
}

// mapFlutterwaveStatus maps Flutterwave status to unified status
func mapFlutterwaveStatus(status string) types.TransactionStatus {
	switch status {
	case "successful":
		return types.StatusSuccess
	case "failed":
		return types.StatusFailed
	case "pending":
		return types.StatusPending
	case "cancelled":
		return types.StatusCancelled
	default:
		return types.StatusPending
	}
}

// mapFlutterwavePaymentType maps Flutterwave payment type to payment method type
func mapFlutterwavePaymentType(paymentType string) types.PaymentMethodType {
	switch paymentType {
	case "card":
		return types.PaymentMethodCard
	case "bank_transfer", "account":
		return types.PaymentMethodBank
	case "ussd":
		return types.PaymentMethodUSSD
	case "mobilemoney", "mobilemoneygh", "mobilemoneyuganda", "mobilemoneyrwanda":
		return types.PaymentMethodMobileMoney
	case "qr":
		return types.PaymentMethodQR
	default:
		return types.PaymentMethodCard
	}
}

// mapChannelsToPaymentOptions converts channel array to Flutterwave payment_options string
func mapChannelsToPaymentOptions(channels []string) string {
	if len(channels) == 0 {
		return ""
	}
	result := ""
	for i, ch := range channels {
		if i > 0 {
			result += ", "
		}
		result += ch
	}
	return result
}
