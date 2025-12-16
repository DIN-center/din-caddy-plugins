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

	// Create JSON-RPC payload for chain ID request
	payload := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","params":[],"id":1}`, chainIDMethod))

	var lastErr error
	for attempt := 0; attempt < requestAttempts; attempt++ {
		// Make POST request with payload
		resBytes, statusCode, err := httpClient.Post(httpUrl, headers, payload, authClient)
		if err != nil {
			lastErr = fmt.Errorf("error sending HTTP request: %w", err)
			continue
		}

		// Use the provided parse function to extract chain ID
		chainID, err := parseFunc(resBytes, *statusCode)
		if err != nil {
			lastErr = err
			continue
		}

		// Success!
		return chainID, nil
	}

	return "", fmt.Errorf("failed after %d attempts: %w", requestAttempts, lastErr)
}

// GetLatestBlockNumberViaJSONRPC performs a JSON-RPC latest block number request
// This is shared logic for JSON-RPC based handlers (EVM, Starknet, Solana)
func GetLatestBlockNumberViaJSONRPC(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockNumberMethod string, parseFunc func(json.RawMessage) (int64, error)) (*LatestBlockResult, error) {
	// Create JSON-RPC payload for latest block number request
	payload := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method":"%s","params":[],"id":1}`, blockNumberMethod))

	var lastErr error
	var lastResponseStatus int
	var lastHealthStatus = Unhealthy

	for attempt := 0; attempt < requestAttempts; attempt++ {
		// Make POST request with payload
		resBytes, statusCode, err := httpClient.Post(httpUrl, headers, payload, authClient)
		if statusCode != nil {
			lastResponseStatus = *statusCode
		}

		if err != nil {
			lastErr = fmt.Errorf("error sending HTTP request: %w", err)
			// HTTP connection errors are considered unhealthy (matches original test expectations)
			lastHealthStatus = Unhealthy
			continue
		}

		// Check HTTP status code for rate limiting first
		if lastResponseStatus == 429 {
			lastErr = fmt.Errorf("rate limit error (status code: %d)", lastResponseStatus)
			lastHealthStatus = Warning
			continue
		}

		// Check for other HTTP error status codes
		if lastResponseStatus >= 400 {
			lastErr = fmt.Errorf("error status code: %d", lastResponseStatus)
			// All HTTP error status codes are considered unhealthy (matches original test expectations)
			lastHealthStatus = Unhealthy
			continue
		}

		// Parse JSON-RPC response structure
		var response JSONRPCResponse
		if err := json.Unmarshal(resBytes, &response); err != nil {
			lastErr = fmt.Errorf("failed to parse JSON-RPC response: %w", err)
			lastHealthStatus = Unhealthy
			continue
		}

		// Check for JSON-RPC error
		if response.Error != nil {
			lastErr = fmt.Errorf("JSON-RPC error %d: %s", response.Error.Code, response.Error.Message)
			// Classify JSON-RPC errors based on their nature
			if IsRetryableJSONRPCError(lastErr, lastResponseStatus) {
				lastHealthStatus = Warning
			} else {
				lastHealthStatus = Unhealthy
			}
			continue
		}

		// Use the provided parse function to extract block number
		blockNumber, err := parseFunc(response.Result)
		if err != nil {
			lastErr = fmt.Errorf("failed to parse block number from result: %w", err)
			lastHealthStatus = Unhealthy
			continue
		}

		// Success!
		return &LatestBlockResult{
			BlockNumber:    blockNumber,
			HealthStatus:   Healthy,
			ResponseStatus: lastResponseStatus,
			Metadata:       make(map[string]interface{}),
		}, nil
	}

	// All attempts failed
	return &LatestBlockResult{
		BlockNumber:    0,
		HealthStatus:   lastHealthStatus,
		ResponseStatus: lastResponseStatus,
		Metadata:       make(map[string]interface{}),
	}, fmt.Errorf("failed after %d attempts: %w", requestAttempts, lastErr)
}

// PerformArchiveCheckViaJSONRPC performs a JSON-RPC archive mode check request
// This is shared logic for JSON-RPC based handlers (EVM, Starknet, Solana)
func PerformArchiveCheckViaJSONRPC(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, archiveMethod string, blockHeight string, createPayloadFunc func(string, string) ([]byte, error), parseResponseFunc func([]byte) error) error {
	var lastErr error
	var lastResponseStatus int

	for attempt := 0; attempt < requestAttempts; attempt++ {
		// Use the provided function to create archive payload
		payload, err := createPayloadFunc(archiveMethod, blockHeight)
		if err != nil {
			lastErr = fmt.Errorf("failed to create archive payload: %w", err)
			continue
		}

		// Make POST request with payload
		resBytes, statusCode, err := httpClient.Post(httpUrl, headers, payload, authClient)
		if statusCode != nil {
			lastResponseStatus = *statusCode
		}

		if err != nil {
			lastErr = fmt.Errorf("error sending HTTP request: %w", err)
			continue
		}

		// Check for specific HTTP error status codes
		if lastResponseStatus >= 400 {
			if lastResponseStatus == 503 || lastResponseStatus == 502 {
				lastErr = fmt.Errorf("network unavailable (status code: %d)", lastResponseStatus)
			} else {
				lastErr = fmt.Errorf("error status code: %d", lastResponseStatus)
			}
			continue
		}

		// Use the provided parse function to validate archive response
		err = parseResponseFunc(resBytes)
		if err != nil {
			lastErr = fmt.Errorf("archive mode check failed: %w", err)
			continue
		}

		// Success!
		return nil
	}

	// All attempts failed
	return fmt.Errorf("failed after %d attempts: %w", requestAttempts, lastErr)
}

// PerformTraceBlockByNumberCheckViaJSONRPC performs a debug_traceBlockByNumber check via JSON-RPC
// This is EVM-specific and verifies trace/debug capabilities for MetaMask compliance
func PerformTraceBlockByNumberCheckViaJSONRPC(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockHeight string) error {
	var lastErr error
	var lastResponseStatus int

	// Create payload once outside the retry loop - it's deterministic
	payload, err := createTraceBlockByNumberPayload(blockHeight)
	if err != nil {
		return fmt.Errorf("failed to create trace payload: %w", err)
	}

	for attempt := 0; attempt < requestAttempts; attempt++ {
		// Make POST request with payload
		resBytes, statusCode, err := httpClient.Post(httpUrl, headers, payload, authClient)
		if statusCode != nil {
			lastResponseStatus = *statusCode
		}

		if err != nil {
			lastErr = fmt.Errorf("error sending HTTP request: %w", err)
			continue
		}

		// Check for specific HTTP error status codes
		if lastResponseStatus >= 400 {
			if lastResponseStatus == 503 || lastResponseStatus == 502 {
				lastErr = fmt.Errorf("network unavailable (status code: %d)", lastResponseStatus)
			} else {
				lastErr = fmt.Errorf("error status code: %d", lastResponseStatus)
			}
			continue
		}

		// Validate the trace response
		err = parseTraceBlockByNumberResponse(resBytes)
		if err != nil {
			lastErr = fmt.Errorf("trace block check failed: %w", err)
			continue
		}

		// Success!
		return nil
	}

	// All attempts failed
	return fmt.Errorf("failed after %d attempts: %w", requestAttempts, lastErr)
}

// PerformGetBlockByNumberViaJSONRPC performs a JSON-RPC get block by number request
// This is shared logic for JSON-RPC based handlers (EVM, Starknet, Solana)
func PerformGetBlockByNumberViaJSONRPC(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockNumber int64, getSupportedMethodsFunc func() []string, createBlockRequestFunc func(string, int64, bool) ([]byte, error), parseBlockResponseFunc func([]byte) (interface{}, error)) (interface{}, error) {
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

	var lastErr error
	for attempt := 0; attempt < requestAttempts; attempt++ {
		// Use the provided function to create block request payload
		payload, err := createBlockRequestFunc(getBlockMethod, blockNumber, false)
		if err != nil {
			lastErr = fmt.Errorf("failed to create block request: %w", err)
			continue
		}

		// Make POST request with payload
		resBytes, statusCode, err := httpClient.Post(httpUrl, headers, payload, authClient)
		if err != nil {
			lastErr = fmt.Errorf("error sending HTTP request: %w", err)
			continue
		}

		// Check HTTP status code
		if statusCode != nil && *statusCode != 200 {
			lastErr = fmt.Errorf("error status code: %d", *statusCode)
			continue
		}

		// Use the provided parse function to parse block response
		blockData, err := parseBlockResponseFunc(resBytes)
		if err != nil {
			lastErr = fmt.Errorf("failed to parse block response: %w", err)
			continue
		}

		// Success!
		return blockData, nil
	}

	// All attempts failed
	return nil, fmt.Errorf("failed after %d attempts: %w", requestAttempts, lastErr)
}
