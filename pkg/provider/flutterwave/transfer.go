package flutterwave

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/loom-payments/loom/pkg/types"
)

// flutterwaveTransferRequest represents a Flutterwave transfer request
type flutterwaveTransferRequest struct {
	AccountBank    string                 `json:"account_bank"`
	AccountNumber  string                 `json:"account_number"`
	Amount         float64                `json:"amount"`
	Currency       string                 `json:"currency"`
	Narration      string                 `json:"narration,omitempty"`
	Reference      string                 `json:"reference,omitempty"`
	CallbackURL    string                 `json:"callback_url,omitempty"`
	DebitCurrency  string                 `json:"debit_currency,omitempty"`
	Meta           map[string]interface{} `json:"meta,omitempty"`
	BeneficiaryName string               `json:"beneficiary_name,omitempty"`
}

// flutterwaveTransferResponse represents a Flutterwave transfer response
type flutterwaveTransferResponse struct {
	ID             int64                  `json:"id"`
	AccountNumber  string                 `json:"account_number"`
	BankCode       string                 `json:"bank_code"`
	BankName       string                 `json:"bank_name"`
	FullName       string                 `json:"full_name"`
	CreatedAt      time.Time              `json:"created_at"`
	Currency       string                 `json:"currency"`
	DebitCurrency  string                 `json:"debit_currency"`
	Amount         float64                `json:"amount"`
	Fee            float64                `json:"fee"`
	Status         string                 `json:"status"`
	Reference      string                 `json:"reference"`
	Narration      string                 `json:"narration"`
	CompleteMessage string                `json:"complete_message"`
	RequiresApproval int                  `json:"requires_approval"`
	IsApproved     int                    `json:"is_approved"`
	Meta           map[string]interface{} `json:"meta"`
}

// flutterwaveBankResponse represents a Flutterwave bank response
type flutterwaveBankResponse struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// flutterwaveResolveResponse represents a resolved account response
type flutterwaveResolveResponse struct {
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
}

// InitiateTransfer initiates a transfer
func (f *Flutterwave) InitiateTransfer(ctx context.Context, req *types.TransferRequest) (*types.TransferResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	flwReq := flutterwaveTransferRequest{
		AccountBank:     req.Recipient.BankCode,
		AccountNumber:   req.Recipient.AccountNumber,
		Amount:          req.Amount.ToMajorUnits(),
		Currency:        string(req.Amount.Currency),
		Narration:       req.Reason,
		Reference:       req.Reference,
		Meta:            req.Metadata,
		BeneficiaryName: req.Recipient.AccountName,
	}

	respBody, err := f.makeRequest(ctx, http.MethodPost, "/transfers", flwReq)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate transfer: %w", err)
	}

	data, err := parseResponse[flutterwaveTransferResponse](respBody)
	if err != nil {
		return nil, err
	}

	return mapFlutterwaveTransferResponse(data), nil
}

// VerifyTransfer verifies a transfer by reference
func (f *Flutterwave) VerifyTransfer(ctx context.Context, reference string) (*types.TransferResponse, error) {
	if reference == "" {
		return nil, types.ErrMissingReference
	}

	// Flutterwave uses transfer ID for fetching, but we can query by reference
	respBody, err := f.makeRequest(ctx, http.MethodGet, "/transfers?reference="+reference, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to verify transfer: %w", err)
	}

	// The response is an array, get the first item
	data, err := parseResponse[[]flutterwaveTransferResponse](respBody)
	if err != nil {
		return nil, err
	}

	if len(*data) == 0 {
		return nil, types.ErrTransactionNotFound
	}

	return mapFlutterwaveTransferResponse(&(*data)[0]), nil
}

// CreateRecipient creates a transfer recipient
// Flutterwave doesn't have a separate recipient creation - it's done inline with transfers
func (f *Flutterwave) CreateRecipient(ctx context.Context, req *types.CreateRecipientRequest) (*types.RecipientResponse, error) {
	// Verify the account first to get the account name
	accountInfo, err := f.ResolveAccount(ctx, req.AccountNumber, req.BankCode)
	if err != nil {
		return nil, err
	}

	// Return a pseudo-recipient (Flutterwave doesn't persist recipients)
	return &types.RecipientResponse{
		RecipientCode: fmt.Sprintf("FLW_%s_%s", req.BankCode, req.AccountNumber),
		Name:          accountInfo.AccountName,
		AccountNumber: req.AccountNumber,
		BankCode:      req.BankCode,
		Currency:      req.Currency,
		IsActive:      true,
		CreatedAt:     time.Now(),
	}, nil
}

// ListBanks returns the list of banks for a country
func (f *Flutterwave) ListBanks(ctx context.Context, country string) ([]types.Bank, error) {
	if country == "" {
		country = "NG"
	}

	respBody, err := f.makeRequest(ctx, http.MethodGet, "/banks/"+country, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list banks: %w", err)
	}

	data, err := parseResponse[[]flutterwaveBankResponse](respBody)
	if err != nil {
		return nil, err
	}

	banks := make([]types.Bank, 0, len(*data))
	for _, b := range *data {
		banks = append(banks, types.Bank{
			ID:       fmt.Sprintf("%d", b.ID),
			Name:     b.Name,
			Code:     b.Code,
			Country:  country,
			IsActive: true,
		})
	}

	return banks, nil
}

// ResolveAccount resolves a bank account number to get the account name
func (f *Flutterwave) ResolveAccount(ctx context.Context, accountNumber, bankCode string) (*types.AccountInfo, error) {
	if accountNumber == "" {
		return nil, types.ErrMissingAccountNum
	}
	if bankCode == "" {
		return nil, types.ErrMissingBankCode
	}

	reqBody := map[string]string{
		"account_number": accountNumber,
		"account_bank":   bankCode,
	}

	respBody, err := f.makeRequest(ctx, http.MethodPost, "/accounts/resolve", reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve account: %w", err)
	}

	data, err := parseResponse[flutterwaveResolveResponse](respBody)
	if err != nil {
		return nil, err
	}

	return &types.AccountInfo{
		AccountNumber: data.AccountNumber,
		AccountName:   data.AccountName,
		BankCode:      bankCode,
	}, nil
}

// mapFlutterwaveTransferResponse maps a Flutterwave transfer response to unified type
func mapFlutterwaveTransferResponse(data *flutterwaveTransferResponse) *types.TransferResponse {
	response := &types.TransferResponse{
		ID:                fmt.Sprintf("%d", data.ID),
		Reference:         data.Reference,
		Status:            mapFlutterwaveTransferStatus(data.Status),
		Amount:            types.FromMajorUnits(data.Amount, types.Currency(data.Currency)),
		Provider:          types.ProviderFlutterwave,
		ProviderReference: fmt.Sprintf("%d", data.ID),
		Reason:            data.Narration,
		Metadata:          data.Meta,
		CreatedAt:         data.CreatedAt,
		FailureReason:     data.CompleteMessage,
	}

	if data.Fee > 0 {
		response.Fees = &types.Money{
			Amount:   int64(data.Fee * 100),
			Currency: types.Currency(data.Currency),
		}
	}

	response.Recipient = &types.TransferRecipient{
		AccountNumber: data.AccountNumber,
		AccountName:   data.FullName,
		BankCode:      data.BankCode,
		BankName:      data.BankName,
	}

	if data.Status == "SUCCESSFUL" {
		response.CompletedAt = &data.CreatedAt
	}

	return response
}

// mapFlutterwaveTransferStatus maps Flutterwave transfer status to unified status
func mapFlutterwaveTransferStatus(status string) types.TransactionStatus {
	switch status {
	case "SUCCESSFUL":
		return types.StatusSuccess
	case "FAILED":
		return types.StatusFailed
	case "PENDING", "NEW":
		return types.StatusPending
	default:
		return types.StatusPending
	}
}
