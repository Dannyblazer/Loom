package paystack

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/loom-payments/loom/pkg/types"
)

// paystackRefundRequest represents a Paystack refund request
type paystackRefundRequest struct {
	Transaction string `json:"transaction"` // Transaction reference or ID
	Amount      int64  `json:"amount,omitempty"`
	Currency    string `json:"currency,omitempty"`
	Reason      string `json:"customer_note,omitempty"`
}

// paystackRefundResponse represents a Paystack refund response
type paystackRefundResponse struct {
	ID                  int64                  `json:"id"`
	Status              string                 `json:"status"`
	TransactionID       int64                  `json:"transaction"`
	Amount              int64                  `json:"amount"`
	Currency            string                 `json:"currency"`
	DeductedAmount      int64                  `json:"deducted_amount"`
	CustomerNote        string                 `json:"customer_note"`
	MerchantNote        string                 `json:"merchant_note"`
	IntegrationID       int64                  `json:"integration"`
	RefundedBy          string                 `json:"refunded_by"`
	ExpectedAt          *time.Time             `json:"expected_at"`
	FullyDeducted       bool                   `json:"fully_deducted"`
	CreatedAt           time.Time              `json:"createdAt"`
	TransactionDetails  map[string]interface{} `json:"transaction_details,omitempty"`
}

// CreateRefund creates a refund for a transaction
func (p *Paystack) CreateRefund(ctx context.Context, req *types.RefundRequest) (*types.RefundResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	paystackReq := paystackRefundRequest{
		Transaction: req.TransactionReference,
		Reason:      req.Reason,
	}

	// Set amount for partial refunds
	if req.Amount != nil && req.Amount.Amount > 0 {
		paystackReq.Amount = req.Amount.Amount
		paystackReq.Currency = string(req.Amount.Currency)
	}

	respBody, err := p.makeRequest(ctx, http.MethodPost, "/refund", paystackReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create refund: %w", err)
	}

	data, err := parseResponse[paystackRefundResponse](respBody)
	if err != nil {
		return nil, err
	}

	return mapPaystackRefundResponse(data, req.TransactionReference), nil
}

// GetRefund retrieves a refund by ID
func (p *Paystack) GetRefund(ctx context.Context, refundID string) (*types.RefundResponse, error) {
	if refundID == "" {
		return nil, types.ErrMissingReference
	}

	respBody, err := p.makeRequest(ctx, http.MethodGet, "/refund/"+refundID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get refund: %w", err)
	}

	data, err := parseResponse[paystackRefundResponse](respBody)
	if err != nil {
		return nil, err
	}

	return mapPaystackRefundResponse(data, ""), nil
}

// mapPaystackRefundResponse maps a Paystack refund response to unified type
func mapPaystackRefundResponse(data *paystackRefundResponse, txRef string) *types.RefundResponse {
	response := &types.RefundResponse{
		ID:                   fmt.Sprintf("%d", data.ID),
		Reference:            fmt.Sprintf("REF_%d", data.ID),
		TransactionReference: txRef,
		Status:               mapPaystackRefundStatus(data.Status),
		Amount:               types.Money{Amount: data.Amount, Currency: types.Currency(data.Currency)},
		Provider:             types.ProviderPaystack,
		ProviderReference:    fmt.Sprintf("%d", data.ID),
		Reason:               data.CustomerNote,
		CreatedAt:            data.CreatedAt,
	}

	if data.Status == "processed" || data.Status == "completed" {
		now := time.Now()
		response.CompletedAt = &now
	}

	return response
}

// mapPaystackRefundStatus maps Paystack refund status to unified status
func mapPaystackRefundStatus(status string) types.TransactionStatus {
	switch status {
	case "pending":
		return types.StatusPending
	case "processed", "completed":
		return types.StatusSuccess
	case "failed":
		return types.StatusFailed
	default:
		return types.StatusPending
	}
}
