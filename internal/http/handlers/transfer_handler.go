package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/loom-payments/loom/pkg/loom"
	"github.com/loom-payments/loom/pkg/types"
)

// TransferHandler handles transfer-related HTTP requests
type TransferHandler struct {
	client *loom.Client
}

// NewTransferHandler creates a new transfer handler
func NewTransferHandler(client *loom.Client) *TransferHandler {
	return &TransferHandler{client: client}
}

// InitiateTransferRequest represents the transfer initiation request
type InitiateTransferRequest struct {
	Amount        float64                `json:"amount" binding:"required,gt=0"`
	Currency      string                 `json:"currency" binding:"required"`
	Reason        string                 `json:"reason,omitempty"`
	Reference     string                 `json:"reference,omitempty"`
	AccountNumber string                 `json:"account_number" binding:"required"`
	AccountName   string                 `json:"account_name,omitempty"`
	BankCode      string                 `json:"bank_code" binding:"required"`
	BankName      string                 `json:"bank_name,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Provider      string                 `json:"provider,omitempty"`
}

// Initiate handles POST /transfers
func (h *TransferHandler) Initiate(c *gin.Context) {
	var req InitiateTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request",
			"error":   err.Error(),
		})
		return
	}

	transferReq := &types.TransferRequest{
		Amount:    types.FromMajorUnits(req.Amount, types.Currency(req.Currency)),
		Reason:    req.Reason,
		Reference: req.Reference,
		Metadata:  req.Metadata,
		Recipient: &types.TransferRecipient{
			AccountNumber: req.AccountNumber,
			AccountName:   req.AccountName,
			BankCode:      req.BankCode,
			BankName:      req.BankName,
		},
	}

	if req.Provider != "" {
		transferReq.PreferredProvider = types.ProviderName(req.Provider)
	}

	if key, exists := c.Get("idempotency_key"); exists {
		transferReq.IdempotencyKey = key.(string)
	}

	resp, err := h.client.Transfers.Initiate(c.Request.Context(), transferReq)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Transfer initiated successfully",
		"data":    resp,
	})
}

// Get handles GET /transfers/:reference
func (h *TransferHandler) Get(c *gin.Context) {
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

	resp, err := h.client.Transfers.Verify(c.Request.Context(), types.ProviderName(providerName), reference)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   resp,
	})
}

// ListBanks handles GET /banks
func (h *TransferHandler) ListBanks(c *gin.Context) {
	country := c.DefaultQuery("country", "NG")
	providerName := c.Query("provider")

	var provider types.ProviderName
	if providerName != "" {
		provider = types.ProviderName(providerName)
	}

	banks, err := h.client.Transfers.ListBanks(c.Request.Context(), provider, country)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   banks,
	})
}

// ResolveAccountRequest represents the account resolution request
type ResolveAccountRequest struct {
	AccountNumber string `json:"account_number" binding:"required"`
	BankCode      string `json:"bank_code" binding:"required"`
	Provider      string `json:"provider,omitempty"`
}

// ResolveAccount handles POST /banks/resolve
func (h *TransferHandler) ResolveAccount(c *gin.Context) {
	var req ResolveAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request",
			"error":   err.Error(),
		})
		return
	}

	var provider types.ProviderName
	if req.Provider != "" {
		provider = types.ProviderName(req.Provider)
	}

	accountInfo, err := h.client.Transfers.ResolveAccount(c.Request.Context(), provider, req.AccountNumber, req.BankCode)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   accountInfo,
	})
}
