package flutterwave

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/loom-payments/loom/pkg/types"
)

// flutterwaveVARequest represents a Flutterwave virtual account creation request
type flutterwaveVARequest struct {
	Email         string  `json:"email"`
	IsPermanent   bool    `json:"is_permanent"`
	BVN           string  `json:"bvn,omitempty"`
	TxRef         string  `json:"tx_ref,omitempty"`
	PhoneNumber   string  `json:"phonenumber,omitempty"`
	FirstName     string  `json:"firstname,omitempty"`
	LastName      string  `json:"lastname,omitempty"`
	Narration     string  `json:"narration,omitempty"`
	Amount        float64 `json:"amount,omitempty"`
	Frequency     int     `json:"frequency,omitempty"`
}

// flutterwaveVAResponse represents a Flutterwave virtual account response
type flutterwaveVAResponse struct {
	ResponseCode    string    `json:"response_code"`
	ResponseMessage string    `json:"response_message"`
	FlwRef          string    `json:"flw_ref"`
	OrderRef        string    `json:"order_ref"`
	AccountNumber   string    `json:"account_number"`
	AccountStatus   string    `json:"account_status"`
	Frequency       int       `json:"frequency"`
	BankName        string    `json:"bank_name"`
	CreatedAt       time.Time `json:"created_at"`
	ExpiryDate      *time.Time `json:"expiry_date"`
	Note            string    `json:"note"`
	Amount          float64   `json:"amount"`
}

// CreateVirtualAccount creates a virtual account with Flutterwave
func (f *Flutterwave) CreateVirtualAccount(ctx context.Context, req *types.VirtualAccountRequest) (*types.VirtualAccountResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	flwReq := flutterwaveVARequest{
		Email:       req.Customer.Email,
		IsPermanent: req.IsPermanent,
		BVN:         req.BVN,
		TxRef:       req.Reference,
	}

	if req.Customer.FirstName != "" {
		flwReq.FirstName = req.Customer.FirstName
	}
	if req.Customer.LastName != "" {
		flwReq.LastName = req.Customer.LastName
	}
	if req.Customer.Phone != "" {
		flwReq.PhoneNumber = req.Customer.Phone
	}
	if req.ExpectedAmount != nil {
		flwReq.Amount = req.ExpectedAmount.ToMajorUnits()
	}

	respBody, err := f.makeRequest(ctx, http.MethodPost, "/virtual-account-numbers", flwReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create virtual account: %w", err)
	}

	data, err := parseResponse[flutterwaveVAResponse](respBody)
	if err != nil {
		return nil, err
	}

	return mapFlutterwaveVAResponse(data, req), nil
}

// GetVirtualAccount retrieves a virtual account by order reference
func (f *Flutterwave) GetVirtualAccount(ctx context.Context, accountID string) (*types.VirtualAccountResponse, error) {
	if accountID == "" {
		return nil, types.ErrMissingReference
	}

	respBody, err := f.makeRequest(ctx, http.MethodGet, "/virtual-account-numbers/"+accountID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get virtual account: %w", err)
	}

	data, err := parseResponse[flutterwaveVAResponse](respBody)
	if err != nil {
		return nil, err
	}

	return mapFlutterwaveVAResponse(data, nil), nil
}

// DeactivateVirtualAccount deactivates a virtual account
func (f *Flutterwave) DeactivateVirtualAccount(ctx context.Context, accountID string) error {
	if accountID == "" {
		return types.ErrMissingReference
	}

	// Flutterwave uses PUT to update status
	reqBody := map[string]string{
		"status": "inactive",
	}

	_, err := f.makeRequest(ctx, http.MethodPut, "/virtual-account-numbers/"+accountID, reqBody)
	if err != nil {
		return fmt.Errorf("failed to deactivate virtual account: %w", err)
	}

	return nil
}

// mapFlutterwaveVAResponse maps a Flutterwave VA response to unified type
func mapFlutterwaveVAResponse(data *flutterwaveVAResponse, req *types.VirtualAccountRequest) *types.VirtualAccountResponse {
	response := &types.VirtualAccountResponse{
		ID:                data.OrderRef,
		AccountNumber:     data.AccountNumber,
		AccountName:       "", // Flutterwave doesn't return account name
		BankName:          data.BankName,
		Currency:          types.NGN,
		Provider:          types.ProviderFlutterwave,
		ProviderAccountID: data.FlwRef,
		IsActive:          data.AccountStatus == "active",
		IsPermanent:       data.Frequency == 0, // frequency 0 means permanent
		CreatedAt:         data.CreatedAt,
		ExpiresAt:         data.ExpiryDate,
	}

	// Set reference from request or response
	if req != nil && req.Reference != "" {
		response.Reference = req.Reference
	} else {
		response.Reference = data.OrderRef
	}

	// Set customer from request if available
	if req != nil && req.Customer != nil {
		response.Customer = req.Customer
		// Construct account name from customer info
		response.AccountName = req.Customer.FullName()
	}

	// Set metadata from request if available
	if req != nil {
		response.Metadata = req.Metadata
	}

	return response
}
