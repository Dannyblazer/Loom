package paystack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/loom-payments/loom/pkg/provider"
	"github.com/loom-payments/loom/pkg/types"
)

const (
	DefaultBaseURL = "https://api.paystack.co"
	DefaultTimeout = 30 * time.Second
)

// Paystack implements the Provider interface for Paystack payment gateway
type Paystack struct {
	config     *provider.Config
	httpClient *http.Client
}

// New creates a new Paystack provider instance
func New(config *provider.Config) *Paystack {
	if config.BaseURL == "" {
		config.BaseURL = DefaultBaseURL
	}
	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}
	if config.RetryConfig == nil {
		config.RetryConfig = provider.DefaultRetryConfig()
	}

	return &Paystack{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Name returns the provider name
func (p *Paystack) Name() types.ProviderName {
	return types.ProviderPaystack
}

// SupportedCurrencies returns currencies supported by Paystack
func (p *Paystack) SupportedCurrencies() []types.Currency {
	return []types.Currency{
		types.NGN,
		types.GHS,
		types.USD,
		types.ZAR,
		types.KES,
	}
}

// SupportedFeatures returns features supported by Paystack
func (p *Paystack) SupportedFeatures() []provider.Feature {
	return []provider.Feature{
		provider.FeatureCharges,
		provider.FeatureRefunds,
		provider.FeaturePartialRefunds,
		provider.FeatureTransfers,
		provider.FeatureBulkTransfers,
		provider.FeatureVirtualAccounts,
		provider.FeatureRecurring,
		provider.FeatureSplitPayments,
		provider.FeatureSubscriptions,
	}
}

// HealthCheck performs a health check on the Paystack API
func (p *Paystack) HealthCheck(ctx context.Context) error {
	_, err := p.makeRequest(ctx, http.MethodGet, "/bank", nil)
	if err != nil {
		return fmt.Errorf("paystack health check failed: %w", err)
	}
	return nil
}

// makeRequest makes an HTTP request to the Paystack API
func (p *Paystack) makeRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, p.config.BaseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.config.SecretKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for error responses
	if resp.StatusCode >= 400 {
		var errResp paystackErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Message != "" {
			return nil, types.NewProviderError(types.ProviderPaystack, errResp.Code, errResp.Message, nil)
		}
		return nil, types.NewProviderError(types.ProviderPaystack, "", fmt.Sprintf("request failed with status %d", resp.StatusCode), nil)
	}

	return respBody, nil
}

// paystackResponse represents the standard Paystack API response structure
type paystackResponse struct {
	Status  bool            `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// paystackErrorResponse represents a Paystack error response
type paystackErrorResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// parseResponse parses a Paystack response and extracts the data
func parseResponse[T any](respBody []byte) (*T, error) {
	var resp paystackResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !resp.Status {
		return nil, types.NewProviderError(types.ProviderPaystack, "", resp.Message, nil)
	}

	var data T
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("failed to parse response data: %w", err)
	}

	return &data, nil
}
