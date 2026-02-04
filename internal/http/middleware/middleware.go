package middleware

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/loom-payments/loom/pkg/idempotency"
)

// RequestID adds a unique request ID to each request
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// Logger logs HTTP requests
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		// Log format: timestamp method path status latency
		if query != "" {
			path = path + "?" + query
		}

		// Simple logging - in production, use structured logging
		gin.DefaultWriter.Write([]byte(
			time.Now().Format(time.RFC3339) + " " +
				c.Request.Method + " " +
				path + " " +
				http.StatusText(status) + " " +
				latency.String() + "\n",
		))
	}
}

// CORS adds CORS headers
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Request-ID, Idempotency-Key")
		c.Header("Access-Control-Expose-Headers", "X-Request-ID")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// Idempotency handles idempotency for POST/PUT requests
func Idempotency(store idempotency.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only apply to POST/PUT requests
		if c.Request.Method != http.MethodPost && c.Request.Method != http.MethodPut {
			c.Next()
			return
		}

		idempotencyKey := c.GetHeader("Idempotency-Key")
		if idempotencyKey == "" {
			// No idempotency key provided, proceed normally
			c.Next()
			return
		}

		ctx := c.Request.Context()

		// Try to acquire lock or get existing response
		record, err := store.Lock(ctx, idempotencyKey, "", 30*time.Second)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "idempotency_conflict",
				"message": "This idempotency key was used with a different request",
			})
			c.Abort()
			return
		}

		// If we got a record back, return the cached response
		if record != nil {
			if record.IsLocked() {
				c.JSON(http.StatusConflict, gin.H{
					"error":   "request_in_progress",
					"message": "A request with this idempotency key is already being processed",
				})
				c.Abort()
				return
			}

			// Return cached response
			c.Header("Idempotent-Replayed", "true")
			c.Data(record.StatusCode, "application/json", record.Response)
			c.Abort()
			return
		}

		// Store the idempotency key in context for later
		c.Set("idempotency_key", idempotencyKey)

		// Create a response writer wrapper to capture the response
		writer := &responseCapture{
			ResponseWriter: c.Writer,
			body:           make([]byte, 0),
		}
		c.Writer = writer

		c.Next()

		// After request completes, store the response
		if !c.IsAborted() {
			response, _ := json.Marshal(gin.H{
				"captured": true,
				"body":     string(writer.body),
			})
			store.Unlock(ctx, idempotencyKey, response, writer.Status())
		}
	}
}

// responseCapture captures response body
type responseCapture struct {
	gin.ResponseWriter
	body []byte
}

func (r *responseCapture) Write(b []byte) (int, error) {
	r.body = append(r.body, b...)
	return r.ResponseWriter.Write(b)
}

// RateLimit implements simple rate limiting (placeholder for production implementation)
func RateLimit(requestsPerMinute int) gin.HandlerFunc {
	// In production, use a proper rate limiter like golang.org/x/time/rate
	// or a distributed rate limiter with Redis
	return func(c *gin.Context) {
		c.Next()
	}
}

// Auth implements API key authentication (placeholder)
func Auth(apiKeys map[string]bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			apiKey = c.Query("api_key")
		}

		if apiKey == "" || !apiKeys[apiKey] {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Invalid or missing API key",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
