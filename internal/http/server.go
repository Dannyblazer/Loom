package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/loom-payments/loom/config"
	"github.com/loom-payments/loom/internal/http/handlers"
	"github.com/loom-payments/loom/internal/http/middleware"
	"github.com/loom-payments/loom/pkg/idempotency"
	"github.com/loom-payments/loom/pkg/loom"
)

// Server represents the HTTP server
type Server struct {
	config     *config.Config
	router     *gin.Engine
	httpServer *http.Server
	client     *loom.Client
	idempStore idempotency.Store
}

// NewServer creates a new HTTP server
func NewServer(cfg *config.Config, client *loom.Client, idempStore idempotency.Store) *Server {
	// Set Gin mode based on environment
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Add recovery middleware
	router.Use(gin.Recovery())

	// Add custom middleware
	router.Use(middleware.RequestID())
	router.Use(middleware.Logger())
	router.Use(middleware.CORS())

	server := &Server{
		config:     cfg,
		router:     router,
		client:     client,
		idempStore: idempStore,
	}

	// Setup routes
	server.setupRoutes()

	return server
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// Health check endpoints
	s.router.GET("/health", s.handleHealth)
	s.router.GET("/health/providers", s.handleProviderHealth)

	// API v1 routes
	v1 := s.router.Group("/api/v1")
	{
		// Apply idempotency middleware to mutating endpoints
		idempMiddleware := middleware.Idempotency(s.idempStore)

		// Charge endpoints
		chargeHandler := handlers.NewChargeHandler(s.client)
		charges := v1.Group("/charges")
		{
			charges.POST("", idempMiddleware, chargeHandler.Initialize)
			charges.GET("/:reference", chargeHandler.Get)
			charges.POST("/:reference/verify", chargeHandler.Verify)
		}

		// Refund endpoints
		refundHandler := handlers.NewRefundHandler(s.client)
		refunds := v1.Group("/refunds")
		{
			refunds.POST("", idempMiddleware, refundHandler.Create)
			refunds.GET("/:id", refundHandler.Get)
		}

		// Transfer endpoints
		transferHandler := handlers.NewTransferHandler(s.client)
		transfers := v1.Group("/transfers")
		{
			transfers.POST("", idempMiddleware, transferHandler.Initiate)
			transfers.GET("/:reference", transferHandler.Get)
		}

		// Bank endpoints
		v1.GET("/banks", transferHandler.ListBanks)
		v1.POST("/banks/resolve", transferHandler.ResolveAccount)

		// Virtual account endpoints
		vaHandler := handlers.NewVirtualAccountHandler(s.client)
		virtualAccounts := v1.Group("/virtual-accounts")
		{
			virtualAccounts.POST("", idempMiddleware, vaHandler.Create)
			virtualAccounts.GET("/:id", vaHandler.Get)
		}

		// Webhook endpoints
		webhookHandler := handlers.NewWebhookHandler(s.client)
		webhooks := v1.Group("/webhooks")
		{
			webhooks.POST("/paystack", webhookHandler.HandlePaystack)
			webhooks.POST("/flutterwave", webhookHandler.HandleFlutterwave)
		}
	}
}

// handleHealth handles the health check endpoint
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":      "healthy",
		"service":     "loom",
		"environment": s.config.Server.Environment,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	})
}

// handleProviderHealth handles the provider health check endpoint
func (s *Server) handleProviderHealth(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	healthResults := s.client.HealthCheck(ctx)
	providerStatus := s.client.GetProviderStatus()

	providers := make(map[string]gin.H)
	allHealthy := true

	for name, err := range healthResults {
		status := "healthy"
		var errMsg string
		if err != nil {
			status = "unhealthy"
			errMsg = err.Error()
			allHealthy = false
		}

		providers[string(name)] = gin.H{
			"status":         status,
			"circuit_state":  providerStatus[name],
			"error":          errMsg,
		}
	}

	httpStatus := http.StatusOK
	if !allHealthy {
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, gin.H{
		"status":    map[bool]string{true: "healthy", false: "degraded"}[allHealthy],
		"providers": providers,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:         ":" + s.config.Server.Port,
		Handler:      s.router,
		ReadTimeout:  s.config.Server.ReadTimeout,
		WriteTimeout: s.config.Server.WriteTimeout,
	}

	fmt.Printf("Starting Loom server on port %s\n", s.config.Server.Port)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// Router returns the underlying Gin router (for testing)
func (s *Server) Router() *gin.Engine {
	return s.router
}
