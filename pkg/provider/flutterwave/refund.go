package flutterwave

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/loom-payments/loom/pkg/types"
)

// flutterwaveRefundRequest represents a Flutterwave refund request
type flutterwaveRefundRequest struct {
	Amount float64 `json:"amount,omitempty"`
}

// flutterwaveRefundResponse represents a Flutterwave refund response
type flutterwaveRefundResponse struct {
	ID              int64     `json:"id"`
	AccountID       int64     `json:"account_id"`
	TxID            int64     `json:"tx_id"`
	FlwRef          string    `json:"flw_ref"`
	WalletID        int64     `json:"wallet_id"`
	AmountRefunded  float64   `json:"amount_refunded"`
	Status          string    `json:"status"`
	Destination     string    `json:"destination"`
	Meta            interface{} `json:"meta"`
	CreatedAt       time.Time `json:"created_at"`
}

// CreateRefund creates a refund for a transaction
func (f *Flutterwave) CreateRefund(ctx context.Context, req *types.RefundRequest) (*types.RefundResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Flutterwave requires transaction ID, so we need to verify first to get it
	// In practice, you'd store the Flutterwave transaction ID when the charge was made
	path := fmt.Sprintf("/transactions/%s/refund", req.TransactionReference)

	var flwReq *flutterwaveRefundRequest
	if req.Amount != nil && req.Amount.Amount > 0 {
		flwReq = &flutterwaveRefundRequest{
			Amount: req.Amount.ToMajorUnits(),
		}
	}

	respBody, err := f.makeRequest(ctx, http.MethodPost, path, flwReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create refund: %w", err)
	}

	data, err := parseResponse[flutterwaveRefundResponse](respBody)
	if err != nil {
		return nil, err
	}

	return mapFlutterwaveRefundResponse(data, req.TransactionReference), nil
}

// GetRefund retrieves a refund by ID
func (f *Flutterwave) GetRefund(ctx context.Context, refundID string) (*types.RefundResponse, error) {
	if refundID == "" {
		return nil, types.ErrMissingReference
	}

	// Flutterwave doesn't have a direct get refund endpoint
	// You'd typically query transactions and filter
	return nil, types.NewProviderError(types.ProviderFlutterwave, "NOT_IMPLEMENTED", "get refund not directly supported by Flutterwave", nil)
}

// mapFlutterwaveRefundResponse maps a Flutterwave refund response to unified type
func mapFlutterwaveRefundResponse(data *flutterwaveRefundResponse, txRef string) *types.RefundResponse {
	response := &types.RefundResponse{
		ID:                   fmt.Sprintf("%d", data.ID),
		Reference:            fmt.Sprintf("REF_%d", data.ID),
		TransactionReference: txRef,
		Status:               mapFlutterwaveRefundStatus(data.Status),
		Amount:               types.FromMajorUnits(data.AmountRefunded, types.NGN), // Currency from original tx
		Provider:             types.ProviderFlutterwave,
		ProviderReference:    data.FlwRef,
		CreatedAt:            data.CreatedAt,
	}

	if data.Status == "completed" {
		response.CompletedAt = &data.CreatedAt
	}

	return response
}

// mapFlutterwaveRefundStatus maps Flutterwave refund status to unified status
func mapFlutterwaveRefundStatus(status string) types.TransactionStatus {
	switch status {
	case "completed":
		return types.StatusSuccess
	case "failed":
		return types.StatusFailed
	case "pending":
		return types.StatusPending
	default:
		return types.StatusPending
	}
}
