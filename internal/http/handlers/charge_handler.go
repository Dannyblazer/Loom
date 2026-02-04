package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/loom-payments/loom/pkg/loom"
	"github.com/loom-payments/loom/pkg/types"
)

// ChargeHandler handles charge-related HTTP requests
type ChargeHandler struct {
	client *loom.Client
}

// NewChargeHandler creates a new charge handler
func NewChargeHandler(client *loom.Client) *ChargeHandler {
	return &ChargeHandler{client: client}
}

// InitializeChargeRequest represents the charge initialization request
type InitializeChargeRequest struct {
	Amount         float64                `json:"amount" binding:"required,gt=0"`
	Currency       string                 `json:"currency" binding:"required"`
	Email          string                 `json:"email" binding:"required,email"`
	Description    string                 `json:"description,omitempty"`
	Reference      string                 `json:"reference,omitempty"`
	CallbackURL    string                 `json:"callback_url,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Channels       []string               `json:"channels,omitempty"`
	Provider       string                 `json:"provider,omitempty"`
}

// Initialize handles POST /charges
func (h *ChargeHandler) Initialize(c *gin.Context) {
	var req InitializeChargeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request",
			"error":   err.Error(),
		})
		return
	}

	chargeReq := &types.ChargeRequest{
		Amount:      types.FromMajorUnits(req.Amount, types.Currency(req.Currency)),
		Email:       req.Email,
		Description: req.Description,
		Reference:   req.Reference,
		CallbackURL: req.CallbackURL,
		Metadata:    req.Metadata,
		Channels:    req.Channels,
	}

	if req.Provider != "" {
		chargeReq.PreferredProvider = types.ProviderName(req.Provider)
	}

	// Get idempotency key if present
	if key, exists := c.Get("idempotency_key"); exists {
		chargeReq.IdempotencyKey = key.(string)
	}

	resp, err := h.client.Charges.Initialize(c.Request.Context(), chargeReq)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Charge initialized successfully",
		"data":    resp,
	})
}

// Get handles GET /charges/:reference
func (h *ChargeHandler) Get(c *gin.Context) {
	reference := c.Param("reference")
	providerName := c.Query("provider")

	if reference == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Reference is required",
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

	resp, err := h.client.Charges.Verify(c.Request.Context(), types.ProviderName(providerName), reference)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   resp,
	})
}

// Verify handles POST /charges/:reference/verify
func (h *ChargeHandler) Verify(c *gin.Context) {
	reference := c.Param("reference")

	var req struct {
		Provider string `json:"provider" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Provider is required",
			"error":   err.Error(),
		})
		return
	}

	resp, err := h.client.Charges.Verify(c.Request.Context(), types.ProviderName(req.Provider), reference)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   resp,
	})
}

// handleError handles errors and returns appropriate HTTP responses
func handleError(c *gin.Context, err error) {
	// Check for specific error types
	switch {
	case err == types.ErrProviderNotFound:
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Provider not found",
			"error":   err.Error(),
		})
	case err == types.ErrTransactionNotFound:
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Transaction not found",
			"error":   err.Error(),
		})
	case err == types.ErrNoProvidersAvailable:
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "error",
			"message": "No payment providers available",
			"error":   err.Error(),
		})
	case err == types.ErrInvalidAmount || err == types.ErrMissingEmail || err == types.ErrMissingCurrency:
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request",
			"error":   err.Error(),
		})
	default:
		// Check if it's a provider error
		if providerErr, ok := err.(*types.ProviderError); ok {
			c.JSON(http.StatusBadGateway, gin.H{
				"status":   "error",
				"message":  "Provider error",
				"provider": providerErr.Provider,
				"error":    providerErr.Message,
			})
			return
		}

		// Generic error
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Internal server error",
			"error":   err.Error(),
		})
	}
}
