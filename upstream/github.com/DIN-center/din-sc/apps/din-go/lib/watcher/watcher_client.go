package watcher

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
)

type WatcherAPIClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewClient creates a new instance of the Watcher API client.
func NewClient(baseURL, apiKey string) *WatcherAPIClient {
	return &WatcherAPIClient{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		HTTPClient: &http.Client{},
	}
}

func (c *WatcherAPIClient) doRequest(method, endpoint string, params interface{}) ([]byte, error) {
	// Build URL with query parameters
	apiURL, err := url.JoinPath(c.BaseURL, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to construct API URL: %w", err)
	}
	if qs := buildQuery(params); qs != "" {
		apiURL = fmt.Sprintf("%s?%s", apiURL, qs)
	}

	// Create HTTP request
	req, err := http.NewRequest(method, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	req.Header.Set("X-API-KEY", c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	// Make HTTP request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle non-200 responses
	if resp.StatusCode != http.StatusOK {
		var errResp = &ErrorResponse{}
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("status code: %d, message: %s", resp.StatusCode, errResp.Message)
	}

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return respBody, nil
}

func (c *WatcherAPIClient) GetCheck(params CheckQueryParams) Result[CheckResponse] {
	// Validate parameters
	if err := params.Validate(); err != nil {
		return Err[CheckResponse](errors.Wrap(err, "Error happened while validating params"))
	}

	// Make the API call
	respBody, err := c.doRequest("GET", CheckResourcePath, params)
	if err != nil {
		return Err[CheckResponse](errors.Wrap(err, "Failed to send the GET /check request"))
	}

	var response CheckResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return Err[CheckResponse](errors.Wrap(err, "Failed to parse the response"))
	}

	return Ok(response)
}

func (c *WatcherAPIClient) GetLatency(params LatencyQueryParams) Result[LatencyResponse] {
	// Validate parameters
	if err := params.Validate(); err != nil {
		return Err[LatencyResponse](errors.Wrap(err, "Error happened while validating params"))
	}

	// Make the API call
	respBody, err := c.doRequest("GET", LatencyResourcePath, params)
	if err != nil {
		return Err[LatencyResponse](errors.Wrap(err, "Failed to send the GET /latency request"))
	}

	var response LatencyResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return Err[LatencyResponse](errors.Wrap(err, "Failed to parse the response"))
	}

	return Ok(response)
}
