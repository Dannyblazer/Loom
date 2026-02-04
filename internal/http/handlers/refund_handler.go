package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/loom-payments/loom/pkg/loom"
	"github.com/loom-payments/loom/pkg/types"
)

// RefundHandler handles refund-related HTTP requests
type RefundHandler struct {
	client *loom.Client
}

// NewRefundHandler creates a new refund handler
func NewRefundHandler(client *loom.Client) *RefundHandler {
	return &RefundHandler{client: client}
}

// CreateRefundRequest represents the refund creation request
type CreateRefundRequest struct {
	TransactionReference string                 `json:"transaction_reference" binding:"required"`
	Amount               *float64               `json:"amount,omitempty"`
	Currency             string                 `json:"currency,omitempty"`
	Reason               string                 `json:"reason,omitempty"`
	Metadata             map[string]interface{} `json:"metadata,omitempty"`
	Provider             string                 `json:"provider,omitempty"`
}

// Create handles POST /refunds
func (h *RefundHandler) Create(c *gin.Context) {
	var req CreateRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request",
			"error":   err.Error(),
		})
		return
	}

	refundReq := &types.RefundRequest{
		TransactionReference: req.TransactionReference,
		Reason:               req.Reason,
		Metadata:             req.Metadata,
	}

	// Set partial refund amount if provided
	if req.Amount != nil && *req.Amount > 0 {
		currency := types.NGN
		if req.Currency != "" {
			currency = types.Currency(req.Currency)
		}
		money := types.FromMajorUnits(*req.Amount, currency)
		refundReq.Amount = &money
	}

	if req.Provider != "" {
		refundReq.PreferredProvider = types.ProviderName(req.Provider)
	}

	if key, exists := c.Get("idempotency_key"); exists {
		refundReq.IdempotencyKey = key.(string)
	}

	resp, err := h.client.Refunds.Create(c.Request.Context(), refundReq)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Refund created successfully",
		"data":    resp,
	})
}

// Get handles GET /refunds/:id
func (h *RefundHandler) Get(c *gin.Context) {
	refundID := c.Param("id")
	providerName := c.Query("provider")

	if refundID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Refund ID is required",
		})
		return
	}

	if providerName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Provider query parameter is required",
		})
		return
	}

	resp, err := h.client.Refunds.Get(c.Request.Context(), types.ProviderName(providerName), refundID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   resp,
	})
}
