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
	retryablePatterns    []string
	nonRetryablePatterns []string
}

// NewJSONRPCErrorClassifier creates a new error classifier with default patterns
func NewJSONRPCErrorClassifier() *JSONRPCErrorClassifier {
	return &JSONRPCErrorClassifier{
		retryablePatterns: []string{
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
		},
		nonRetryablePatterns: []string{
			"method not found",   // -32601
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
			"gas limit exceeded", // Gas limit exceeded
			"vm execution error", // Virtual machine execution errors
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

	// Common retryable error patterns
	for _, pattern := range c.retryablePatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}

	// Check for retryable JSON-RPC error codes (after non-retryable pattern check)
	// -32000 to -32099: Server errors (implementation defined)
	// Only retry these if they don't match non-retryable patterns above
	if strings.Contains(errMsg, "json-rpc error -320") {
		return true
	}

	// Default to not retrying unknown errors
	return false
}

// Package-level functions for backward compatibility

var defaultClassifier = NewJSONRPCErrorClassifier()

// IsRetryableJSONRPCError checks if a JSON-RPC error is retryable
// This is shared logic for all JSON-RPC based handlers
func IsRetryableJSONRPCError(err error, statusCode int) bool {
	return defaultClassifier.IsRetryable(err, statusCode)
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
