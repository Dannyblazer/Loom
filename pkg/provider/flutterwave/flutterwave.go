package flutterwave

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
	DefaultBaseURL = "https://api.flutterwave.com/v3"
	DefaultTimeout = 30 * time.Second
)

// Flutterwave implements the Provider interface for Flutterwave payment gateway
type Flutterwave struct {
	config        *provider.Config
	encryptionKey string
	httpClient    *http.Client
}

// FlutterwaveConfig extends provider.Config with Flutterwave-specific settings
type FlutterwaveConfig struct {
	*provider.Config
	EncryptionKey string // For card encryption
}

// New creates a new Flutterwave provider instance
func New(config *FlutterwaveConfig) *Flutterwave {
	if config.BaseURL == "" {
		config.BaseURL = DefaultBaseURL
	}
	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}
	if config.RetryConfig == nil {
		config.RetryConfig = provider.DefaultRetryConfig()
	}

	return &Flutterwave{
		config:        config.Config,
		encryptionKey: config.EncryptionKey,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Name returns the provider name
func (f *Flutterwave) Name() types.ProviderName {
	return types.ProviderFlutterwave
}

// SupportedCurrencies returns currencies supported by Flutterwave
func (f *Flutterwave) SupportedCurrencies() []types.Currency {
	return []types.Currency{
		types.NGN,
		types.GHS,
		types.USD,
		types.ZAR,
		types.KES,
		types.GBP,
		types.EUR,
	}
}

// SupportedFeatures returns features supported by Flutterwave
func (f *Flutterwave) SupportedFeatures() []provider.Feature {
	return []provider.Feature{
		provider.FeatureCharges,
		provider.FeatureRefunds,
		provider.FeaturePartialRefunds,
		provider.FeatureTransfers,
		provider.FeatureBulkTransfers,
		provider.FeatureVirtualAccounts,
		provider.FeatureSplitPayments,
		provider.FeatureSubscriptions,
	}
}

// HealthCheck performs a health check on the Flutterwave API
func (f *Flutterwave) HealthCheck(ctx context.Context) error {
	_, err := f.makeRequest(ctx, http.MethodGet, "/banks/NG", nil)
	if err != nil {
		return fmt.Errorf("flutterwave health check failed: %w", err)
	}
	return nil
}

// makeRequest makes an HTTP request to the Flutterwave API
func (f *Flutterwave) makeRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, f.config.BaseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+f.config.SecretKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := f.httpClient.Do(req)
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
		var errResp flutterwaveResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Message != "" {
			return nil, types.NewProviderError(types.ProviderFlutterwave, "", errResp.Message, nil)
		}
		return nil, types.NewProviderError(types.ProviderFlutterwave, "", fmt.Sprintf("request failed with status %d", resp.StatusCode), nil)
	}

	return respBody, nil
}

// flutterwaveResponse represents the standard Flutterwave API response structure
type flutterwaveResponse struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// parseResponse parses a Flutterwave response and extracts the data
func parseResponse[T any](respBody []byte) (*T, error) {
	var resp flutterwaveResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.Status != "success" {
		return nil, types.NewProviderError(types.ProviderFlutterwave, "", resp.Message, nil)
	}

	var data T
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("failed to parse response data: %w", err)
	}

	return &data, nil
}
