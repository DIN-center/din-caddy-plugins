// lib/network/helpers.go
package network

import (
	"encoding/json"
	"fmt"
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

// ParseNumericBlockNumber parses a numeric block number (common for Solana)
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
