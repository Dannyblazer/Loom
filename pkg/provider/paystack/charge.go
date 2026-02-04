package paystack

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/loom-payments/loom/pkg/types"
)

// paystackInitRequest represents a Paystack transaction initialization request
type paystackInitRequest struct {
	Email       string                 `json:"email"`
	Amount      int64                  `json:"amount"`
	Currency    string                 `json:"currency,omitempty"`
	Reference   string                 `json:"reference,omitempty"`
	CallbackURL string                 `json:"callback_url,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Channels    []string               `json:"channels,omitempty"`
	SplitCode   string                 `json:"split_code,omitempty"`
}

// paystackInitResponse represents a Paystack transaction initialization response
type paystackInitResponse struct {
	AuthorizationURL string `json:"authorization_url"`
	AccessCode       string `json:"access_code"`
	Reference        string `json:"reference"`
}

// paystackVerifyResponse represents a Paystack transaction verification response
type paystackVerifyResponse struct {
	ID              int64                  `json:"id"`
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
	Message         string                 `json:"message"`
}

type paystackCustomer struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	CustomerCode string `json:"customer_code"`
	Phone        string `json:"phone"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
}

type paystackAuthorization struct {
	AuthorizationCode string `json:"authorization_code"`
	Bin               string `json:"bin"`
	Last4             string `json:"last4"`
	ExpMonth          string `json:"exp_month"`
	ExpYear           string `json:"exp_year"`
	Channel           string `json:"channel"`
	CardType          string `json:"card_type"`
	Bank              string `json:"bank"`
	CountryCode       string `json:"country_code"`
	Brand             string `json:"brand"`
	Reusable          bool   `json:"reusable"`
}

// InitializeCharge initializes a charge transaction with Paystack
func (p *Paystack) InitializeCharge(ctx context.Context, req *types.ChargeRequest) (*types.ChargeResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	paystackReq := paystackInitRequest{
		Email:       req.Email,
		Amount:      req.Amount.Amount, // Already in kobo
		Currency:    string(req.Amount.Currency),
		Reference:   req.Reference,
		CallbackURL: req.CallbackURL,
		Metadata:    req.Metadata,
		Channels:    req.Channels,
		SplitCode:   req.SplitCode,
	}

	respBody, err := p.makeRequest(ctx, http.MethodPost, "/transaction/initialize", paystackReq)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize charge: %w", err)
	}

	data, err := parseResponse[paystackInitResponse](respBody)
	if err != nil {
		return nil, err
	}

	return &types.ChargeResponse{
		Reference:         data.Reference,
		Status:            types.StatusPending,
		Amount:            req.Amount,
		AuthorizationURL:  data.AuthorizationURL,
		AccessCode:        data.AccessCode,
		Provider:          types.ProviderPaystack,
		ProviderReference: data.Reference,
		CreatedAt:         time.Now(),
		Metadata:          req.Metadata,
	}, nil
}

// VerifyCharge verifies a charge transaction with Paystack
func (p *Paystack) VerifyCharge(ctx context.Context, reference string) (*types.ChargeResponse, error) {
	if reference == "" {
		return nil, types.ErrMissingReference
	}

	respBody, err := p.makeRequest(ctx, http.MethodGet, "/transaction/verify/"+reference, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to verify charge: %w", err)
	}

	data, err := parseResponse[paystackVerifyResponse](respBody)
	if err != nil {
		return nil, err
	}

	response := &types.ChargeResponse{
		ID:                fmt.Sprintf("%d", data.ID),
		Reference:         data.Reference,
		Status:            mapPaystackStatus(data.Status),
		Amount:            types.Money{Amount: data.Amount, Currency: types.Currency(data.Currency)},
		Provider:          types.ProviderPaystack,
		ProviderReference: data.Reference,
		PaymentMethod:     mapPaystackChannel(data.Channel),
		CreatedAt:         data.CreatedAt,
		PaidAt:            data.PaidAt,
		Metadata:          data.Metadata,
		FailureReason:     data.GatewayResponse,
	}

	if data.Fees > 0 {
		response.Fees = &types.Money{Amount: data.Fees, Currency: types.Currency(data.Currency)}
	}

	// Map customer info
	response.Customer = &types.CustomerInfo{
		ID:        fmt.Sprintf("%d", data.Customer.ID),
		Email:     data.Customer.Email,
		Phone:     data.Customer.Phone,
		FirstName: data.Customer.FirstName,
		LastName:  data.Customer.LastName,
	}

	// Map authorization (card details)
	if data.Authorization != nil {
		response.Card = &types.CardDetails{
			Last4:       data.Authorization.Last4,
			Brand:       data.Authorization.Brand,
			ExpMonth:    data.Authorization.ExpMonth,
			ExpYear:     data.Authorization.ExpYear,
			Bank:        data.Authorization.Bank,
			CountryCode: data.Authorization.CountryCode,
			CardType:    data.Authorization.CardType,
		}
		response.Authorization = &types.Authorization{
			AuthorizationCode: data.Authorization.AuthorizationCode,
			Reusable:          data.Authorization.Reusable,
			CardDetails:       response.Card,
		}
	}

	return response, nil
}

// mapPaystackStatus maps Paystack status to unified status
func mapPaystackStatus(status string) types.TransactionStatus {
	switch status {
	case "success":
		return types.StatusSuccess
	case "failed":
		return types.StatusFailed
	case "abandoned":
		return types.StatusCancelled
	case "pending":
		return types.StatusPending
	default:
		return types.StatusPending
	}
}

// mapPaystackChannel maps Paystack channel to payment method type
func mapPaystackChannel(channel string) types.PaymentMethodType {
	switch channel {
	case "card":
		return types.PaymentMethodCard
	case "bank":
		return types.PaymentMethodBank
	case "ussd":
		return types.PaymentMethodUSSD
	case "bank_transfer":
		return types.PaymentMethodTransfer
	case "mobile_money":
		return types.PaymentMethodMobileMoney
	case "qr":
		return types.PaymentMethodQR
	default:
		return types.PaymentMethodCard
	}
}
