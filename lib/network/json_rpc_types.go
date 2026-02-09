// lib/network/json_rpc_types.go
package network

import (
	"encoding/json"
	"fmt"
	"strings"
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

// JSONRPCErrorClassifier provides error classification logic for JSON-RPC errors
type JSONRPCErrorClassifier struct {
	nonRetryablePatterns                 []string
	retryableOnDifferentProviderPatterns []string
}

// NewJSONRPCErrorClassifier creates a new error classifier with default patterns
func NewJSONRPCErrorClassifier() *JSONRPCErrorClassifier {
	return &JSONRPCErrorClassifier{
		nonRetryablePatterns: []string{
			"method not found",   // -32601 (also in retryableOnDifferentProviderPatterns)
			"invalid params",     // -32602
			"invalid request",    // -32600
			"parse error",        // -32700
			"unauthorized",       // Often permanent
			"forbidden",          // Often permanent
			"insufficient funds", // Transaction specific
			"nonce too low",      // Transaction specific
			"already known",      // Transaction already submitted
			"gas",                // Gas-related errors (insufficient gas, gas estimation failed, etc.)
			"revert",             // EVM execution reverts
			"vm execution error", // Virtual machine execution errors
		},
		retryableOnDifferentProviderPatterns: []string{
			"method not found", // -32601: provider may not support this method, but another might
		},
	}
}

// IsRetryable checks if a JSON-RPC error is retryable
func (c *JSONRPCErrorClassifier) IsRetryable(err error, statusCode int) bool {
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

	// First check for explicitly non-retryable patterns
	// These should never be retried regardless of error code
	for _, pattern := range c.nonRetryablePatterns {
		if strings.Contains(errMsg, pattern) {
			return false
		}
	}

	return true
}

// IsRetryableOnDifferentProvider returns true if this error should be retried
// on a different provider (but NOT the same one). This covers cases like -32601
// where one provider may not support a method but another might.
//
// Matching is done by error code (-32601) rather than message text, since different
// providers use different wording (e.g., "Method not found", "unsupported method",
// "the method X does not exist/is not available").
func (c *JSONRPCErrorClassifier) IsRetryableOnDifferentProvider(err error, statusCode int) bool {
	// HTTP server errors should be retried on the same provider (standard retry)
	if statusCode >= 500 {
		return false
	}

	// Rate limiting should be retried on the same provider (standard retry)
	if statusCode == 429 {
		return false
	}

	if err == nil {
		return false
	}

	// Match on JSON-RPC error code -32601 (method not found) regardless of message text
	if IsJSONRPCErrorCode(err, -32601) {
		return true
	}

	// Also match text patterns for cases where error code isn't in the standard format
	errMsg := strings.ToLower(err.Error())
	for _, pattern := range c.retryableOnDifferentProviderPatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}

	return false
}

// Package-level functions for backward compatibility

var defaultClassifier = NewJSONRPCErrorClassifier()

// IsRetryableJSONRPCError checks if a JSON-RPC error is retryable
// This is shared logic for all JSON-RPC based handlers
func IsRetryableJSONRPCError(err error, statusCode int) bool {
	return defaultClassifier.IsRetryable(err, statusCode)
}

// IsRetryableOnDifferentProviderJSONRPCError checks if a JSON-RPC error should be retried
// on a different provider. This is shared logic for all JSON-RPC based handlers.
func IsRetryableOnDifferentProviderJSONRPCError(err error, statusCode int) bool {
	return defaultClassifier.IsRetryableOnDifferentProvider(err, statusCode)
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

// ExtractJSONRPCError extracts the JSON-RPC error from a response body
// Returns nil if no error is found
func ExtractJSONRPCError(body []byte) *JSONRPCError {
	var response JSONRPCResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil
	}
	return response.Error
}
