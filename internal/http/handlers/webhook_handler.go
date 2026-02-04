package handlers

import (
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/loom-payments/loom/pkg/loom"
	"github.com/loom-payments/loom/pkg/types"
)

// WebhookHandler handles webhook HTTP requests
type WebhookHandler struct {
	client   *loom.Client
	handlers map[types.WebhookEventType]types.WebhookHandler
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(client *loom.Client) *WebhookHandler {
	return &WebhookHandler{
		client:   client,
		handlers: make(map[types.WebhookEventType]types.WebhookHandler),
	}
}

// RegisterHandler registers a handler for a specific event type
func (h *WebhookHandler) RegisterHandler(eventType types.WebhookEventType, handler types.WebhookHandler) {
	h.handlers[eventType] = handler
}

// HandlePaystack handles POST /webhooks/paystack
func (h *WebhookHandler) HandlePaystack(c *gin.Context) {
	h.handleWebhook(c, types.ProviderPaystack, "x-paystack-signature")
}

// HandleFlutterwave handles POST /webhooks/flutterwave
func (h *WebhookHandler) HandleFlutterwave(c *gin.Context) {
	h.handleWebhook(c, types.ProviderFlutterwave, "verif-hash")
}

// handleWebhook is the common webhook handling logic
func (h *WebhookHandler) handleWebhook(c *gin.Context, provider types.ProviderName, signatureHeader string) {
	// Read the raw body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Failed to read request body",
		})
		return
	}

	// Get signature from header
	signature := c.GetHeader(signatureHeader)

	// Verify and parse the webhook
	event, err := h.client.Webhooks.VerifyAndParse(provider, body, signature)
	if err != nil {
		if err == types.ErrInvalidSignature {
			log.Printf("Invalid webhook signature from %s", provider)
			c.JSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Invalid signature",
			})
			return
		}

		log.Printf("Failed to parse webhook from %s: %v", provider, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Failed to parse webhook",
		})
		return
	}

	// Log the event
	log.Printf("Received webhook event: type=%s provider=%s reference=%s", event.Type, event.Provider, event.Reference)

	// Call registered handler if exists
	if handler, exists := h.handlers[event.Type]; exists {
		if err := handler.HandleEvent(event); err != nil {
			log.Printf("Error handling webhook event %s: %v", event.Type, err)
			// Still return 200 to prevent retries for business logic errors
		}
	}

	// Always return 200 to acknowledge receipt
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Webhook received",
		"event": gin.H{
			"type":      event.Type,
			"reference": event.Reference,
		},
	})
}

// DefaultWebhookHandlers returns default handlers that log events
func DefaultWebhookHandlers() map[types.WebhookEventType]types.WebhookHandler {
	handlers := make(map[types.WebhookEventType]types.WebhookHandler)

	// Log handler for all event types
	logHandler := types.WebhookHandlerFunc(func(event *types.WebhookEvent) error {
		log.Printf("Webhook Event: type=%s provider=%s reference=%s status=%s",
			event.Type, event.Provider, event.Reference, event.Status)

		if event.Amount != nil {
			log.Printf("  Amount: %d %s", event.Amount.Amount, event.Amount.Currency)
		}
		if event.CustomerEmail != "" {
			log.Printf("  Customer: %s", event.CustomerEmail)
		}
		return nil
	})

	// Register for common events
	handlers[types.EventChargeSuccess] = logHandler
	handlers[types.EventChargeFailed] = logHandler
	handlers[types.EventTransferSuccess] = logHandler
	handlers[types.EventTransferFailed] = logHandler
	handlers[types.EventRefundSuccess] = logHandler
	handlers[types.EventVirtualAccountCredit] = logHandler

	return handlers
}
