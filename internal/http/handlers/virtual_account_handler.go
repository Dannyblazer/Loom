package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/loom-payments/loom/pkg/loom"
	"github.com/loom-payments/loom/pkg/types"
)

// VirtualAccountHandler handles virtual account HTTP requests
type VirtualAccountHandler struct {
	client *loom.Client
}

// NewVirtualAccountHandler creates a new virtual account handler
func NewVirtualAccountHandler(client *loom.Client) *VirtualAccountHandler {
	return &VirtualAccountHandler{client: client}
}

// CreateVirtualAccountRequest represents the virtual account creation request
type CreateVirtualAccountRequest struct {
	Email         string                 `json:"email" binding:"required,email"`
	FirstName     string                 `json:"first_name,omitempty"`
	LastName      string                 `json:"last_name,omitempty"`
	Phone         string                 `json:"phone,omitempty"`
	Reference     string                 `json:"reference,omitempty"`
	BVN           string                 `json:"bvn,omitempty"`
	PreferredBank string                 `json:"preferred_bank,omitempty"`
	IsPermanent   bool                   `json:"is_permanent"`
	Currency      string                 `json:"currency,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Provider      string                 `json:"provider,omitempty"`
}

// Create handles POST /virtual-accounts
func (h *VirtualAccountHandler) Create(c *gin.Context) {
	var req CreateVirtualAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request",
			"error":   err.Error(),
		})
		return
	}

	vaReq := &types.VirtualAccountRequest{
		Customer: &types.CustomerInfo{
			Email:     req.Email,
			FirstName: req.FirstName,
			LastName:  req.LastName,
			Phone:     req.Phone,
		},
		Reference:     req.Reference,
		BVN:           req.BVN,
		PreferredBank: req.PreferredBank,
		IsPermanent:   req.IsPermanent,
		Metadata:      req.Metadata,
	}

	if req.Currency != "" {
		vaReq.Currency = types.Currency(req.Currency)
	}

	if req.Provider != "" {
		vaReq.PreferredProvider = types.ProviderName(req.Provider)
	}

	if key, exists := c.Get("idempotency_key"); exists {
		vaReq.IdempotencyKey = key.(string)
	}

	resp, err := h.client.VirtualAccounts.Create(c.Request.Context(), vaReq)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Virtual account created successfully",
		"data":    resp,
	})
}

// Get handles GET /virtual-accounts/:id
func (h *VirtualAccountHandler) Get(c *gin.Context) {
	accountID := c.Param("id")
	providerName := c.Query("provider")

	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Account ID is required",
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

	resp, err := h.client.VirtualAccounts.Get(c.Request.Context(), types.ProviderName(providerName), accountID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   resp,
	})
}
