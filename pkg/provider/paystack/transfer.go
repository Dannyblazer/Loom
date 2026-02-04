package paystack

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/loom-payments/loom/pkg/types"
)

// paystackTransferRequest represents a Paystack transfer request
type paystackTransferRequest struct {
	Source    string                 `json:"source"` // "balance"
	Amount    int64                  `json:"amount"`
	Currency  string                 `json:"currency,omitempty"`
	Recipient string                 `json:"recipient"` // recipient code
	Reason    string                 `json:"reason,omitempty"`
	Reference string                 `json:"reference,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// paystackTransferResponse represents a Paystack transfer response
type paystackTransferResponse struct {
	ID            int64                  `json:"id"`
	TransferCode  string                 `json:"transfer_code"`
	Reference     string                 `json:"reference"`
	Status        string                 `json:"status"`
	Amount        int64                  `json:"amount"`
	Currency      string                 `json:"currency"`
	Source        string                 `json:"source"`
	Reason        string                 `json:"reason"`
	Recipient     paystackRecipientData  `json:"recipient"`
	CreatedAt     time.Time              `json:"createdAt"`
	UpdatedAt     time.Time              `json:"updatedAt"`
	Metadata      map[string]interface{} `json:"metadata"`
	FailureReason string                 `json:"failures,omitempty"`
}

type paystackRecipientData struct {
	ID            int64                 `json:"id"`
	RecipientCode string                `json:"recipient_code"`
	Name          string                `json:"name"`
	Type          string                `json:"type"`
	Details       paystackAccountDetails `json:"details"`
}

type paystackAccountDetails struct {
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
	BankCode      string `json:"bank_code"`
	BankName      string `json:"bank_name"`
}

// paystackCreateRecipientRequest represents a request to create a transfer recipient
type paystackCreateRecipientRequest struct {
	Type          string                 `json:"type"` // nuban, mobile_money, etc.
	Name          string                 `json:"name"`
	AccountNumber string                 `json:"account_number"`
	BankCode      string                 `json:"bank_code"`
	Currency      string                 `json:"currency,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// paystackCreateRecipientResponse represents a Paystack recipient creation response
type paystackCreateRecipientResponse struct {
	ID            int64                 `json:"id"`
	RecipientCode string                `json:"recipient_code"`
	Name          string                `json:"name"`
	Type          string                `json:"type"`
	Currency      string                `json:"currency"`
	Details       paystackAccountDetails `json:"details"`
	IsDeleted     bool                  `json:"is_deleted"`
	CreatedAt     time.Time             `json:"createdAt"`
}

// paystackBankResponse represents a Paystack bank list response
type paystackBankResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Code      string `json:"code"`
	Country   string `json:"country"`
	Currency  string `json:"currency"`
	Type      string `json:"type"`
	IsDeleted bool   `json:"is_deleted"`
	Active    bool   `json:"active"`
}

// paystackResolveAccountResponse represents a resolved account response
type paystackResolveAccountResponse struct {
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
	BankID        int64  `json:"bank_id"`
}

// InitiateTransfer initiates a transfer to a recipient
func (p *Paystack) InitiateTransfer(ctx context.Context, req *types.TransferRequest) (*types.TransferResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Get or create recipient code
	recipientCode := req.Recipient.RecipientCode
	if recipientCode == "" {
		// Create recipient first
		recipientReq := &types.CreateRecipientRequest{
			Type:          "nuban",
			Name:          req.Recipient.AccountName,
			AccountNumber: req.Recipient.AccountNumber,
			BankCode:      req.Recipient.BankCode,
		}
		recipient, err := p.CreateRecipient(ctx, recipientReq)
		if err != nil {
			return nil, fmt.Errorf("failed to create recipient: %w", err)
		}
		recipientCode = recipient.RecipientCode
	}

	paystackReq := paystackTransferRequest{
		Source:    "balance",
		Amount:    req.Amount.Amount,
		Currency:  string(req.Amount.Currency),
		Recipient: recipientCode,
		Reason:    req.Reason,
		Reference: req.Reference,
		Metadata:  req.Metadata,
	}

	respBody, err := p.makeRequest(ctx, http.MethodPost, "/transfer", paystackReq)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate transfer: %w", err)
	}

	data, err := parseResponse[paystackTransferResponse](respBody)
	if err != nil {
		return nil, err
	}

	return mapPaystackTransferResponse(data), nil
}

// VerifyTransfer verifies a transfer by reference
func (p *Paystack) VerifyTransfer(ctx context.Context, reference string) (*types.TransferResponse, error) {
	if reference == "" {
		return nil, types.ErrMissingReference
	}

	respBody, err := p.makeRequest(ctx, http.MethodGet, "/transfer/verify/"+reference, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to verify transfer: %w", err)
	}

	data, err := parseResponse[paystackTransferResponse](respBody)
	if err != nil {
		return nil, err
	}

	return mapPaystackTransferResponse(data), nil
}

// CreateRecipient creates a transfer recipient
func (p *Paystack) CreateRecipient(ctx context.Context, req *types.CreateRecipientRequest) (*types.RecipientResponse, error) {
	if req.AccountNumber == "" {
		return nil, types.ErrMissingAccountNum
	}
	if req.BankCode == "" {
		return nil, types.ErrMissingBankCode
	}

	paystackReq := paystackCreateRecipientRequest{
		Type:          req.Type,
		Name:          req.Name,
		AccountNumber: req.AccountNumber,
		BankCode:      req.BankCode,
		Currency:      req.Currency,
		Metadata:      req.Metadata,
	}

	if paystackReq.Type == "" {
		paystackReq.Type = "nuban"
	}

	respBody, err := p.makeRequest(ctx, http.MethodPost, "/transferrecipient", paystackReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create recipient: %w", err)
	}

	data, err := parseResponse[paystackCreateRecipientResponse](respBody)
	if err != nil {
		return nil, err
	}

	return &types.RecipientResponse{
		RecipientCode: data.RecipientCode,
		Name:          data.Name,
		AccountNumber: data.Details.AccountNumber,
		BankCode:      data.Details.BankCode,
		BankName:      data.Details.BankName,
		Currency:      data.Currency,
		IsActive:      !data.IsDeleted,
		CreatedAt:     data.CreatedAt,
	}, nil
}

// ListBanks returns the list of banks for a country
func (p *Paystack) ListBanks(ctx context.Context, country string) ([]types.Bank, error) {
	path := "/bank"
	if country != "" {
		path = fmt.Sprintf("/bank?country=%s", country)
	}

	respBody, err := p.makeRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list banks: %w", err)
	}

	data, err := parseResponse[[]paystackBankResponse](respBody)
	if err != nil {
		return nil, err
	}

	banks := make([]types.Bank, 0, len(*data))
	for _, b := range *data {
		if !b.Active || b.IsDeleted {
			continue
		}
		banks = append(banks, types.Bank{
			ID:       fmt.Sprintf("%d", b.ID),
			Name:     b.Name,
			Code:     b.Code,
			Country:  b.Country,
			Currency: b.Currency,
			IsActive: b.Active,
		})
	}

	return banks, nil
}

// ResolveAccount resolves a bank account number to get the account name
func (p *Paystack) ResolveAccount(ctx context.Context, accountNumber, bankCode string) (*types.AccountInfo, error) {
	if accountNumber == "" {
		return nil, types.ErrMissingAccountNum
	}
	if bankCode == "" {
		return nil, types.ErrMissingBankCode
	}

	path := fmt.Sprintf("/bank/resolve?account_number=%s&bank_code=%s", accountNumber, bankCode)

	respBody, err := p.makeRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve account: %w", err)
	}

	data, err := parseResponse[paystackResolveAccountResponse](respBody)
	if err != nil {
		return nil, err
	}

	return &types.AccountInfo{
		AccountNumber: data.AccountNumber,
		AccountName:   data.AccountName,
		BankCode:      bankCode,
	}, nil
}

// mapPaystackTransferResponse maps a Paystack transfer response to unified type
func mapPaystackTransferResponse(data *paystackTransferResponse) *types.TransferResponse {
	response := &types.TransferResponse{
		ID:                fmt.Sprintf("%d", data.ID),
		Reference:         data.Reference,
		Status:            mapPaystackTransferStatus(data.Status),
		Amount:            types.Money{Amount: data.Amount, Currency: types.Currency(data.Currency)},
		Provider:          types.ProviderPaystack,
		ProviderReference: data.TransferCode,
		Reason:            data.Reason,
		Metadata:          data.Metadata,
		CreatedAt:         data.CreatedAt,
		FailureReason:     data.FailureReason,
	}

	response.Recipient = &types.TransferRecipient{
		RecipientCode: data.Recipient.RecipientCode,
		AccountNumber: data.Recipient.Details.AccountNumber,
		AccountName:   data.Recipient.Details.AccountName,
		BankCode:      data.Recipient.Details.BankCode,
		BankName:      data.Recipient.Details.BankName,
	}

	if data.Status == "success" {
		response.CompletedAt = &data.UpdatedAt
	}

	return response
}

// mapPaystackTransferStatus maps Paystack transfer status to unified status
func mapPaystackTransferStatus(status string) types.TransactionStatus {
	switch status {
	case "success":
		return types.StatusSuccess
	case "failed":
		return types.StatusFailed
	case "pending", "otp", "processing":
		return types.StatusPending
	case "reversed":
		return types.StatusRefunded
	default:
		return types.StatusPending
	}
}
