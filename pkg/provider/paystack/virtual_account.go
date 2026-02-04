package paystack

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/loom-payments/loom/pkg/types"
)

// paystackDVARequest represents a dedicated virtual account creation request
type paystackDVARequest struct {
	Customer       string   `json:"customer"` // Customer email or code
	PreferredBank  string   `json:"preferred_bank,omitempty"`
	SplitCode      string   `json:"split_code,omitempty"`
	SubAccount     string   `json:"subaccount,omitempty"`
	Phone          string   `json:"phone,omitempty"`
	FirstName      string   `json:"first_name,omitempty"`
	LastName       string   `json:"last_name,omitempty"`
	BVN            string   `json:"bvn,omitempty"`
}

// paystackDVAResponse represents a dedicated virtual account response
type paystackDVAResponse struct {
	Bank          paystackDVABank     `json:"bank"`
	AccountName   string              `json:"account_name"`
	AccountNumber string              `json:"account_number"`
	Assigned      bool                `json:"assigned"`
	Currency      string              `json:"currency"`
	Metadata      interface{}         `json:"metadata"`
	Active        bool                `json:"active"`
	ID            int64               `json:"id"`
	CreatedAt     time.Time           `json:"createdAt"`
	UpdatedAt     time.Time           `json:"updatedAt"`
	Customer      paystackDVACustomer `json:"customer"`
	Assignment    paystackAssignment  `json:"assignment"`
}

type paystackDVABank struct {
	Name string `json:"name"`
	ID   int64  `json:"id"`
	Slug string `json:"slug"`
}

type paystackDVACustomer struct {
	ID           int64  `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Email        string `json:"email"`
	CustomerCode string `json:"customer_code"`
	Phone        string `json:"phone"`
}

type paystackAssignment struct {
	Integration  int64  `json:"integration"`
	AssigneeID   int64  `json:"assignee_id"`
	AssigneeType string `json:"assignee_type"`
	Expired      bool   `json:"expired"`
	AccountType  string `json:"account_type"`
}

// CreateVirtualAccount creates a dedicated virtual account with Paystack
func (p *Paystack) CreateVirtualAccount(ctx context.Context, req *types.VirtualAccountRequest) (*types.VirtualAccountResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// First, ensure customer exists or create one
	customerIdentifier := req.Customer.Email

	paystackReq := paystackDVARequest{
		Customer:      customerIdentifier,
		PreferredBank: req.PreferredBank,
		BVN:           req.BVN,
	}

	if req.Customer.FirstName != "" {
		paystackReq.FirstName = req.Customer.FirstName
	}
	if req.Customer.LastName != "" {
		paystackReq.LastName = req.Customer.LastName
	}
	if req.Customer.Phone != "" {
		paystackReq.Phone = req.Customer.Phone
	}

	respBody, err := p.makeRequest(ctx, http.MethodPost, "/dedicated_account/assign", paystackReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create virtual account: %w", err)
	}

	data, err := parseResponse[paystackDVAResponse](respBody)
	if err != nil {
		return nil, err
	}

	return mapPaystackDVAResponse(data, req), nil
}

// GetVirtualAccount retrieves a virtual account by ID
func (p *Paystack) GetVirtualAccount(ctx context.Context, accountID string) (*types.VirtualAccountResponse, error) {
	if accountID == "" {
		return nil, types.ErrMissingReference
	}

	respBody, err := p.makeRequest(ctx, http.MethodGet, "/dedicated_account/"+accountID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get virtual account: %w", err)
	}

	data, err := parseResponse[paystackDVAResponse](respBody)
	if err != nil {
		return nil, err
	}

	return mapPaystackDVAResponse(data, nil), nil
}

// DeactivateVirtualAccount deactivates a virtual account
func (p *Paystack) DeactivateVirtualAccount(ctx context.Context, accountID string) error {
	if accountID == "" {
		return types.ErrMissingReference
	}

	_, err := p.makeRequest(ctx, http.MethodDelete, "/dedicated_account/"+accountID, nil)
	if err != nil {
		return fmt.Errorf("failed to deactivate virtual account: %w", err)
	}

	return nil
}

// mapPaystackDVAResponse maps a Paystack DVA response to unified type
func mapPaystackDVAResponse(data *paystackDVAResponse, req *types.VirtualAccountRequest) *types.VirtualAccountResponse {
	response := &types.VirtualAccountResponse{
		ID:                fmt.Sprintf("%d", data.ID),
		AccountNumber:     data.AccountNumber,
		AccountName:       data.AccountName,
		BankName:          data.Bank.Name,
		BankCode:          data.Bank.Slug,
		Currency:          types.Currency(data.Currency),
		Provider:          types.ProviderPaystack,
		ProviderAccountID: fmt.Sprintf("%d", data.ID),
		IsActive:          data.Active && data.Assigned,
		IsPermanent:       true, // Paystack DVAs are permanent
		CreatedAt:         data.CreatedAt,
	}

	// Map customer info
	response.Customer = &types.CustomerInfo{
		ID:        fmt.Sprintf("%d", data.Customer.ID),
		Email:     data.Customer.Email,
		Phone:     data.Customer.Phone,
		FirstName: data.Customer.FirstName,
		LastName:  data.Customer.LastName,
	}

	// Set reference from request if available
	if req != nil && req.Reference != "" {
		response.Reference = req.Reference
	} else {
		response.Reference = fmt.Sprintf("DVA_%d", data.ID)
	}

	// Set metadata from request if available
	if req != nil {
		response.Metadata = req.Metadata
	}

	return response
}
