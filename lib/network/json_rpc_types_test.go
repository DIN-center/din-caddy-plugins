package network

import (
	"fmt"
	"testing"
)

func TestIsRetryableJSONRPCError_GasAndRevertHandling(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		statusCode int
		expected   bool
	}{
		// Gas-related errors should NOT be retryable even with -32000 error code
		{
			name:       "insufficient gas with -32000",
			err:        fmt.Errorf("json-rpc error -32000: insufficient gas"),
			statusCode: 200,
			expected:   false, // Should not retry gas errors
		},
		{
			name:       "out of gas error",
			err:        fmt.Errorf("out of gas"),
			statusCode: 200,
			expected:   false,
		},
		{
			name:       "gas limit exceeded",
			err:        fmt.Errorf("gas limit exceeded"),
			statusCode: 200,
			expected:   false,
		},
		{
			name:       "intrinsic gas too low",
			err:        fmt.Errorf("intrinsic gas too low"),
			statusCode: 200,
			expected:   false,
		},
		// Revert-related errors should NOT be retryable
		{
			name:       "execution reverted with -32000",
			err:        fmt.Errorf("json-rpc error -32000: execution reverted"),
			statusCode: 200,
			expected:   false, // Should not retry revert errors
		},
		{
			name:       "vm execution error",
			err:        fmt.Errorf("vm execution error"),
			statusCode: 200,
			expected:   false,
		},
		{
			name:       "simple revert",
			err:        fmt.Errorf("revert"),
			statusCode: 200,
			expected:   false,
		},
		// Other -32000 errors should be retryable
		{
			name:       "generic -32000 server error",
			err:        fmt.Errorf("json-rpc error -32000: server busy"),
			statusCode: 200,
			expected:   true, // Should retry non-gas/non-revert -32000 errors
		},
		{
			name:       "network timeout -32001",
			err:        fmt.Errorf("json-rpc error -32001: network timeout"),
			statusCode: 200,
			expected:   true,
		},
		// Retryable patterns should still work
		{
			name:       "connection error",
			err:        fmt.Errorf("connection timeout"),
			statusCode: 200,
			expected:   true,
		},
		{
			name:       "rate limit error",
			err:        fmt.Errorf("rate limit exceeded"),
			statusCode: 200,
			expected:   true,
		},
		// HTTP errors should still be retryable
		{
			name:       "HTTP 500 error",
			err:        fmt.Errorf("internal server error"),
			statusCode: 500,
			expected:   true,
		},
		{
			name:       "HTTP 429 rate limit",
			err:        fmt.Errorf("too many requests"),
			statusCode: 429,
			expected:   true,
		},
		// Traditional non-retryable errors should still not be retryable
		{
			name:       "method not found",
			err:        fmt.Errorf("method not found"),
			statusCode: 200,
			expected:   false,
		},
		{
			name:       "insufficient funds",
			err:        fmt.Errorf("insufficient funds"),
			statusCode: 200,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableJSONRPCError(tt.err, tt.statusCode)
			if result != tt.expected {
				t.Errorf("IsRetryableJSONRPCError(%v, %d) = %v, expected %v",
					tt.err, tt.statusCode, result, tt.expected)
			}
		})
	}
}

func TestIsRetryableOnDifferentProviderJSONRPCError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		statusCode int
		expected   bool
	}{
		// Method not found should be retryable on a different provider
		{
			name:       "method not found with -32601",
			err:        fmt.Errorf("JSON-RPC error -32601: method not found"),
			statusCode: 200,
			expected:   true,
		},
		{
			name:       "method not found plain",
			err:        fmt.Errorf("method not found"),
			statusCode: 200,
			expected:   true,
		},
		{
			name:       "the method does not exist is not supported",
			err:        fmt.Errorf("the method debug_traceBlockByNumber is not supported"),
			statusCode: 200,
			expected:   false, // does not contain exact "method not found" pattern
		},
		// HTTP 500 takes precedence — should use standard retry, not different-provider
		{
			name:       "method not found with HTTP 500",
			err:        fmt.Errorf("method not found"),
			statusCode: 500,
			expected:   false,
		},
		// HTTP 429 takes precedence — should use standard retry
		{
			name:       "method not found with HTTP 429",
			err:        fmt.Errorf("method not found"),
			statusCode: 429,
			expected:   false,
		},
		// Other non-retryable errors should NOT trigger different-provider retry
		{
			name:       "invalid params",
			err:        fmt.Errorf("invalid params"),
			statusCode: 200,
			expected:   false,
		},
		{
			name:       "revert",
			err:        fmt.Errorf("execution reverted"),
			statusCode: 200,
			expected:   false,
		},
		{
			name:       "insufficient funds",
			err:        fmt.Errorf("insufficient funds"),
			statusCode: 200,
			expected:   false,
		},
		// Generic server errors should NOT trigger different-provider retry
		{
			name:       "generic server error",
			err:        fmt.Errorf("json-rpc error -32000: server busy"),
			statusCode: 200,
			expected:   false,
		},
		// Nil error
		{
			name:       "nil error",
			err:        nil,
			statusCode: 200,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableOnDifferentProviderJSONRPCError(tt.err, tt.statusCode)
			if result != tt.expected {
				t.Errorf("IsRetryableOnDifferentProviderJSONRPCError(%v, %d) = %v, expected %v",
					tt.err, tt.statusCode, result, tt.expected)
			}
		})
	}
}
