// lib/network/helpers.go
package network

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
)

// JSONRPCResponse represents a standard JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error object
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ParseJSONRPCResponse parses a JSON-RPC response and checks for errors
// This is shared logic for all JSON-RPC based handlers (EVM, Starknet, Solana)
func ParseJSONRPCResponse(body []byte, statusCode int) error {
	// First check HTTP status
	if statusCode < 200 || statusCode >= 300 {
		return fmt.Errorf("HTTP error: %d", statusCode)
	}

	// Parse JSON-RPC response
	var response JSONRPCResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse JSON-RPC response: %w", err)
	}

	// Check for JSON-RPC error
	if response.Error != nil {
		return fmt.Errorf("JSON-RPC error %d: %s", response.Error.Code, response.Error.Message)
	}

	return nil
}

// IsRetryableJSONRPCError checks if a JSON-RPC error is retryable
// This is shared logic for all JSON-RPC based handlers
func IsRetryableJSONRPCError(err error, statusCode int) bool {
	// HTTP server errors are always retryable
	if statusCode >= 500 {
		return true
	}

	// Rate limiting is retryable
	if statusCode == 429 {
		return true
	}

	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())

	// Check for retryable JSON-RPC error codes
	// -32000 to -32099: Server errors (implementation defined)
	// These are typically retryable
	if strings.Contains(errMsg, "json-rpc error -320") {
		return true
	}

	// Common retryable error patterns
	retryablePatterns := []string{
		"timeout",
		"connection",
		"network",
		"rate limit",
		"server error",
		"internal error",
		"unavailable",
		"socket hang up",
		"connection reset",
		"temporary failure",
		"too many requests",
		"service unavailable",
		"gateway timeout",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}

	// Non-retryable JSON-RPC errors
	nonRetryablePatterns := []string{
		"method not found",   // -32601
		"invalid params",     // -32602
		"invalid request",    // -32600
		"parse error",        // -32700
		"unauthorized",       // Often permanent
		"forbidden",          // Often permanent
		"insufficient funds", // Transaction specific
		"nonce too low",      // Transaction specific
		"already known",      // Transaction already submitted
	}

	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(errMsg, pattern) {
			return false
		}
	}

	// Default to not retrying unknown errors
	return false
}

// ExtractJSONRPCError extracts the JSON-RPC error from a response body
// Returns nil if no error is found
func ExtractJSONRPCError(body []byte) *JSONRPCError {
	var response JSONRPCResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil
	}
	return response.Error
}

// IsJSONRPCErrorCode checks if an error matches a specific JSON-RPC error code
func IsJSONRPCErrorCode(err error, code int) bool {
	if err == nil {
		return false
	}

	// Try to extract JSON-RPC error from the error message
	errMsg := err.Error()
	expectedMsg := fmt.Sprintf("json-rpc error %d:", code)
	return strings.Contains(strings.ToLower(errMsg), strings.ToLower(expectedMsg))
}

// ParseJSONRPCBlockNumberResponse parses a JSON-RPC response containing a block number
// This is shared logic for JSON-RPC based handlers (EVM, Starknet, Solana)
func ParseJSONRPCBlockNumberResponse(body []byte, statusCode int) (int64, error) {
	// Check HTTP status first
	if statusCode >= 400 {
		if statusCode == 429 {
			return 0, fmt.Errorf("rate limit error (status code: %d)", statusCode)
		}
		return 0, fmt.Errorf("error status code: %d", statusCode)
	}

	// Parse JSON-RPC response
	var response JSONRPCResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return 0, fmt.Errorf("failed to parse JSON-RPC response: %w", err)
	}

	// Check for JSON-RPC error
	if response.Error != nil {
		return 0, fmt.Errorf("JSON-RPC error %d: %s", response.Error.Code, response.Error.Message)
	}

	// Parse the result - this will be network-specific
	// Each handler should implement its own logic for extracting the block number
	return 0, fmt.Errorf("block number parsing must be implemented by specific handler")
}

// ParseHexBlockNumber parses a hex-encoded block number (common for EVM chains)
func ParseHexBlockNumber(result json.RawMessage) (int64, error) {
	var hexStr string
	if err := json.Unmarshal(result, &hexStr); err != nil {
		return 0, fmt.Errorf("failed to unmarshal block number: %w", err)
	}

	if hexStr == "" || len(hexStr) < 2 || hexStr[:2] != "0x" {
		return 0, fmt.Errorf("invalid hex block number: %s", hexStr)
	}

	blockNumber, err := strconv.ParseInt(hexStr[2:], 16, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse hex block number: %w", err)
	}

	return blockNumber, nil
}

// ParseNumericBlockNumber parses a numeric block number (common for Solana and Starknet)
func ParseNumericBlockNumber(result json.RawMessage) (int64, error) {
	var num float64
	if err := json.Unmarshal(result, &num); err != nil {
		return 0, fmt.Errorf("failed to unmarshal numeric block number: %w", err)
	}

	return int64(num), nil
}

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
	var lastHealthStatus HealthStatus = Unhealthy

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
			Extra:          make(map[string]interface{}),
		}, nil
	}

	// All attempts failed
	return &LatestBlockResult{
		BlockNumber:    0,
		HealthStatus:   lastHealthStatus,
		ResponseStatus: lastResponseStatus,
		Extra:          make(map[string]interface{}),
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

// CreateJSONRPCRequestContext creates a JSON-RPC specific request context
// This contains the JSON-RPC parsing logic moved from modules/helpers.go
func CreateJSONRPCRequestContext(payload []byte, method string) (map[string]interface{}, error) {
	context := make(map[string]interface{})

	// Extract params from payload for accurate JSONRPCRequest
	var params json.RawMessage
	var tempReq struct {
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(payload, &tempReq); err == nil && len(tempReq.Params) > 0 {
		params = tempReq.Params
	} else {
		// Default to empty array for requests without params
		params = json.RawMessage(`[]`)
	}

	// Store RPC-specific context
	context["jsonrpc"] = "2.0"
	context["method"] = method
	context["id"] = json.RawMessage(`1`)
	context["params"] = params

	return context, nil
}

// ParseJSONRPCPayload extracts method and params from a JSON-RPC payload
func ParseJSONRPCPayload(payload []byte) (method string, params json.RawMessage, err error) {
	var tempReq struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}

	if err := json.Unmarshal(payload, &tempReq); err != nil {
		return "", nil, fmt.Errorf("failed to parse JSON-RPC payload: %w", err)
	}

	return tempReq.Method, tempReq.Params, nil
}

// ConfigureJSONRPCRequestPath configures the request path for JSON-RPC providers
// This is shared logic for all JSON-RPC handlers (EVM, Starknet, Solana)
func ConfigureJSONRPCRequestPath(req *http.Request, providerPath string) {
	if providerPath != "" {
		req.URL.RawPath = providerPath
		req.URL.Path, _ = url.PathUnescape(req.URL.RawPath)
	} else {
		// For JSON-RPC providers without configured paths, clear RawPath
		req.URL.RawPath = ""
		// Path already set, no changes needed
	}
}

// ConfigureRESTRequestPath configures the request path for REST API providers
// This is shared logic for all REST handlers (Beacon Chain, Bitcoin Esplora)
func ConfigureRESTRequestPath(req *http.Request, providerPath string, networkName string) {
	currentPath := req.URL.Path
	
	// Strip the network prefix from the path
	// e.g., "/eth-beacon-mainnet/eth/v1/beacon/genesis" -> "/eth/v1/beacon/genesis"
	pathSegments := strings.Split(strings.TrimPrefix(currentPath, "/"), "/")
	if len(pathSegments) > 1 && pathSegments[0] == networkName {
		// Remove the first segment (network name) and rebuild path
		strippedPath := "/" + strings.Join(pathSegments[1:], "/")
		currentPath = strippedPath
	}
	
	// Combine provider base path with processed request path
	if providerPath != "" && providerPath != "/" {
		combinedPath := strings.TrimSuffix(providerPath, "/") + currentPath
		req.URL.Path = combinedPath
	} else {
		req.URL.Path = currentPath
	}
	req.URL.RawPath = ""
}
