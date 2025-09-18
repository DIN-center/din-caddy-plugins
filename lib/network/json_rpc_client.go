// lib/network/json_rpc_client.go
package network

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
)

// ============================================================================
// JSON-RPC Client Operations
// ============================================================================

// GetChainIDViaJSONRPC performs a JSON-RPC chain ID request
// This is shared logic for JSON-RPC based handlers (EVM, Starknet, Solana)
func GetChainIDViaJSONRPC(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, chainIDMethod string, parseFunc func([]byte, int) (string, error)) (string, error) {
	// Single attempt - let middleware handle retries
	// Create JSON-RPC payload for chain ID request
	payload := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","params":[],"id":1}`, chainIDMethod))

	// Make POST request with payload
	resBytes, statusCode, err := httpClient.Post(httpUrl, headers, payload, authClient)
	if err != nil {
		return "", fmt.Errorf("error sending HTTP request: %w", err)
	}

	// Use the provided parse function to extract chain ID
	chainID, err := parseFunc(resBytes, *statusCode)
	if err != nil {
		return "", err
	}

	return chainID, nil
}

// GetLatestBlockNumberViaJSONRPC performs a JSON-RPC latest block number request
// This is shared logic for JSON-RPC based handlers (EVM, Starknet, Solana)
func GetLatestBlockNumberViaJSONRPC(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockNumberMethod string, parseFunc func(json.RawMessage) (int64, error)) (*LatestBlockResult, error) {
	// Single attempt - let middleware handle retries
	// Create JSON-RPC payload for latest block number request
	payload := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","params":[],"id":1}`, blockNumberMethod))

	// Make POST request with payload
	resBytes, statusCode, err := httpClient.Post(httpUrl, headers, payload, authClient)
	var responseStatus int
	if statusCode != nil {
		responseStatus = *statusCode
	}

	if err != nil {
		// HTTP connection errors are considered unhealthy
		return &LatestBlockResult{
			BlockNumber:    0,
			HealthStatus:   Unhealthy,
			ResponseStatus: responseStatus,
			Metadata:       make(map[string]interface{}),
		}, fmt.Errorf("error sending HTTP request: %w", err)
	}

	// Check HTTP status code for rate limiting first
	if responseStatus == 429 {
		return &LatestBlockResult{
			BlockNumber:    0,
			HealthStatus:   Warning,
			ResponseStatus: responseStatus,
			Metadata:       make(map[string]interface{}),
		}, fmt.Errorf("rate limit error (status code: %d)", responseStatus)
	}

	// Check for server errors (5xx)
	if responseStatus >= 500 {
		return &LatestBlockResult{
			BlockNumber:    0,
			HealthStatus:   Warning,
			ResponseStatus: responseStatus,
			Metadata:       make(map[string]interface{}),
		}, fmt.Errorf("server error (status code: %d)", responseStatus)
	}

	// Check for other HTTP error status codes
	if responseStatus >= 400 {
		return &LatestBlockResult{
			BlockNumber:    0,
			HealthStatus:   Unhealthy,
			ResponseStatus: responseStatus,
			Metadata:       make(map[string]interface{}),
		}, fmt.Errorf("error status code: %d", responseStatus)
	}

	// Parse JSON-RPC response structure
	var response JSONRPCResponse
	if err := json.Unmarshal(resBytes, &response); err != nil {
		return &LatestBlockResult{
			BlockNumber:    0,
			HealthStatus:   Unhealthy,
			ResponseStatus: responseStatus,
			Metadata:       make(map[string]interface{}),
		}, fmt.Errorf("failed to parse JSON-RPC response: %w", err)
	}

	// Check for JSON-RPC error
	if response.Error != nil {
		var healthStatus HealthStatus
		// Classify JSON-RPC errors based on their nature
		if IsRetryableJSONRPCError(fmt.Errorf("JSON-RPC error %d: %s", response.Error.Code, response.Error.Message), responseStatus) {
			healthStatus = Warning
		} else {
			healthStatus = Unhealthy
		}
		return &LatestBlockResult{
			BlockNumber:    0,
			HealthStatus:   healthStatus,
			ResponseStatus: responseStatus,
			Metadata:       make(map[string]interface{}),
		}, fmt.Errorf("JSON-RPC error %d: %s", response.Error.Code, response.Error.Message)
	}

	// Use the provided parse function to extract block number
	blockNumber, err := parseFunc(response.Result)
	if err != nil {
		return &LatestBlockResult{
			BlockNumber:    0,
			HealthStatus:   Unhealthy,
			ResponseStatus: responseStatus,
			Metadata:       make(map[string]interface{}),
		}, fmt.Errorf("failed to parse block number from result: %w", err)
	}

	// Success!
	return &LatestBlockResult{
		BlockNumber:    blockNumber,
		HealthStatus:   Healthy,
		ResponseStatus: responseStatus,
		Metadata:       make(map[string]interface{}),
	}, nil
}

// PerformArchiveCheckViaJSONRPC performs a JSON-RPC archive mode check request
// This is shared logic for JSON-RPC based handlers (EVM, Starknet, Solana)
func PerformArchiveCheckViaJSONRPC(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, archiveMethod string, blockHeight string, createPayloadFunc func(string, string) ([]byte, error), parseResponseFunc func([]byte) error) error {
	// Single attempt - let middleware handle retries
	// Use the provided function to create archive payload
	payload, err := createPayloadFunc(archiveMethod, blockHeight)
	if err != nil {
		return fmt.Errorf("failed to create archive payload: %w", err)
	}

	// Make POST request with payload
	resBytes, statusCode, err := httpClient.Post(httpUrl, headers, payload, authClient)
	if err != nil {
		return fmt.Errorf("error sending HTTP request: %w", err)
	}

	var responseStatus int
	if statusCode != nil {
		responseStatus = *statusCode
	}

	// Check for specific HTTP error status codes
	if responseStatus >= 400 {
		if responseStatus == 503 || responseStatus == 502 {
			return fmt.Errorf("network unavailable (status code: %d)", responseStatus)
		} else {
			return fmt.Errorf("error status code: %d", responseStatus)
		}
	}

	// Use the provided parse function to validate archive response
	err = parseResponseFunc(resBytes)
	if err != nil {
		return fmt.Errorf("archive mode check failed: %w", err)
	}

	return nil
}

// PerformGetBlockByNumberViaJSONRPC performs a JSON-RPC get block by number request
// This is shared logic for JSON-RPC based handlers (EVM, Starknet, Solana)
func PerformGetBlockByNumberViaJSONRPC(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockNumber int64, getSupportedMethodsFunc func() []string, createBlockRequestFunc func(string, int64, bool) ([]byte, error), parseBlockResponseFunc func([]byte) (interface{}, error)) (interface{}, error) {
	// Single attempt - let middleware handle retries
	// Find the appropriate get block by number method
	supportedMethods := getSupportedMethodsFunc()
	if len(supportedMethods) == 0 {
		return nil, fmt.Errorf("no supported block methods available")
	}

	var getBlockMethod string
	for _, method := range supportedMethods {
		methodLower := strings.ToLower(method)
		if strings.Contains(methodLower, "getblockbynumber") ||
			strings.Contains(methodLower, "get_block_by_number") {
			getBlockMethod = method
			break
		}
	}

	if getBlockMethod == "" {
		return nil, fmt.Errorf("no getBlockByNumber method found")
	}

	// Use the provided function to create block request payload
	payload, err := createBlockRequestFunc(getBlockMethod, blockNumber, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create block request: %w", err)
	}

	// Make POST request with payload
	resBytes, statusCode, err := httpClient.Post(httpUrl, headers, payload, authClient)
	if err != nil {
		return nil, fmt.Errorf("error sending HTTP request: %w", err)
	}

	// Check HTTP status code
	if statusCode != nil && *statusCode != 200 {
		return nil, fmt.Errorf("error status code: %d", *statusCode)
	}

	// Use the provided parse function to parse block response
	blockData, err := parseBlockResponseFunc(resBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse block response: %w", err)
	}

	return blockData, nil
}
