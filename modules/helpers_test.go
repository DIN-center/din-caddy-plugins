package modules

import (
	"encoding/json"
	"testing"

	dinHttp "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/caddyserver/caddy/v2"
	"github.com/stretchr/testify/assert"
)

func TestCheckForJSONRPCError(t *testing.T) {
	tests := []struct {
		name           string
		responseBody   []byte
		expectedError  *dinHttp.JSONRPCError
		expectNilError bool
	}{
		{
			name:           "Empty response body",
			responseBody:   []byte{},
			expectedError:  nil,
			expectNilError: true,
		},
		{
			name:           "Invalid JSON",
			responseBody:   []byte(`{"invalid": json}`),
			expectedError:  nil,
			expectNilError: true,
		},
		{
			name:           "Valid JSON-RPC success response",
			responseBody:   []byte(`{"jsonrpc":"2.0","result":"0x1234","id":1}`),
			expectedError:  nil,
			expectNilError: true,
		},
		{
			name:           "Valid JSON-RPC success response with numeric result (user example)",
			responseBody:   []byte(`{"jsonrpc":"2.0","result":320179690,"id":1}`),
			expectedError:  nil,
			expectNilError: true,
		},
		{
			name:         "JSON-RPC error response",
			responseBody: []byte(`{"jsonrpc":"2.0","error":{"code":-32601,"message":"Method not found"},"id":1}`),
			expectedError: &dinHttp.JSONRPCError{
				Code:    -32601,
				Message: "Method not found",
			},
			expectNilError: false,
		},
		{
			name:         "JSON-RPC error with data field",
			responseBody: []byte(`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid params","data":"Additional error info"},"id":1}`),
			expectedError: &dinHttp.JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Additional error info",
			},
			expectNilError: false,
		},
		{
			name:         "JSON-RPC error response with complex data",
			responseBody: []byte(`{"jsonrpc":"2.0","error":{"code":-32603,"message":"Internal error","data":{"details":"Server overloaded","retry_after":30}},"id":1}`),
			expectedError: &dinHttp.JSONRPCError{
				Code:    -32603,
				Message: "Internal error",
				Data:    map[string]interface{}{"details": "Server overloaded", "retry_after": float64(30)},
			},
			expectNilError: false,
		},
		{
			name:           "Non-JSON-RPC response",
			responseBody:   []byte(`{"status":"ok","data":"some data"}`),
			expectedError:  nil,
			expectNilError: true,
		},
		{
			name:         "Solana transaction version error (user's specific case)",
			responseBody: []byte(`{"jsonrpc":"2.0","error":{"code":-32015,"message":"Transaction version (0) is not supported by the requesting client. Please try the request again with the following configuration parameter: \"maxSupportedTransactionVersion\": 0"},"id":0}`),
			expectedError: &dinHttp.JSONRPCError{
				Code:    -32015,
				Message: "Transaction version (0) is not supported by the requesting client. Please try the request again with the following configuration parameter: \"maxSupportedTransactionVersion\": 0",
			},
			expectNilError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkForJSONRPCError(tt.responseBody)

			if tt.expectNilError {
				assert.Nil(t, result, "Expected nil error but got: %v", result)
			} else {
				assert.NotNil(t, result, "Expected error but got nil")
				assert.Equal(t, tt.expectedError.Code, result.Code, "Error code mismatch")
				assert.Equal(t, tt.expectedError.Message, result.Message, "Error message mismatch")
				assert.Equal(t, tt.expectedError.Data, result.Data, "Error data mismatch")
			}
		})
	}
}

func TestGetRequestBody(t *testing.T) {
	tests := []struct {
		name          string
		setupReplacer func(repl *caddy.Replacer)
		wantRequest   *dinHttp.JSONRPCRequest
		wantErr       bool
		expectErrStr  string
	}{
		{
			name: "Successful retrieval and unmarshal",
			setupReplacer: func(repl *caddy.Replacer) {
				req := dinHttp.JSONRPCRequest{Method: "test_method", JSONRPC: "2.0"}
				bodyBytes, _ := json.Marshal(req)
				repl.Set(RequestBodyKey, bodyBytes)
			},
			wantRequest: &dinHttp.JSONRPCRequest{
				Method:  "test_method",
				Params:  json.RawMessage("null"),
				ID:      json.RawMessage("null"),
				JSONRPC: "2.0",
			},
			wantErr: false,
		},
		{
			name: "RequestBodyKey present but value is not []byte",
			setupReplacer: func(repl *caddy.Replacer) {
				repl.Set(RequestBodyKey, "not a byte array")
			},
			wantRequest:  nil,
			wantErr:      true,
			expectErrStr: "request body is not a byte array",
		},
		{
			name: "RequestBodyKey present, value is []byte, but JSON unmarshal fails",
			setupReplacer: func(repl *caddy.Replacer) {
				repl.Set(RequestBodyKey, []byte("invalid json"))
			},
			wantRequest:  nil,
			wantErr:      true,
			expectErrStr: "failed to unmarshal request body: invalid character 'i' looking for beginning of value",
		},
		{
			name: "RequestBodyKey not present",
			setupReplacer: func(repl *caddy.Replacer) {
				// Do nothing, key is not set
			},
			wantRequest: nil,
			wantErr:     false, // Function returns nil, nil in this case
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repl := caddy.NewReplacer()
			tt.setupReplacer(repl)

			got, err := getRequestBody(repl)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectErrStr)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantRequest, got)
			}
		})
	}
}

func TestGetRequestMethod(t *testing.T) {
	tests := []struct {
		name          string
		setupReplacer func(repl *caddy.Replacer)
		wantMethod    string
		wantErr       bool
		expectErrStr  string
	}{
		{
			name: "Successful retrieval of method string",
			setupReplacer: func(repl *caddy.Replacer) {
				repl.Set(RequestMethodKey, "eth_call")
			},
			wantMethod: "eth_call",
			wantErr:    false,
		},
		{
			name: "RequestMethodKey present but value is not string",
			setupReplacer: func(repl *caddy.Replacer) {
				repl.Set(RequestMethodKey, 123)
			},
			wantMethod:   "",
			wantErr:      true,
			expectErrStr: "request method is not a string",
		},
		{
			name: "RequestMethodKey not present",
			setupReplacer: func(repl *caddy.Replacer) {
				// Do nothing, key is not set
			},
			wantMethod:   "",
			wantErr:      true,
			expectErrStr: "request method not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repl := caddy.NewReplacer()
			if tt.setupReplacer != nil {
				tt.setupReplacer(repl)
			}

			gotMethod, err := getRequestMethod(repl)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectErrStr != "" {
					assert.Contains(t, err.Error(), tt.expectErrStr)
				}
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantMethod, gotMethod)
		})
	}
}

func TestIsJSONRPCErrorRetryable(t *testing.T) {
	tests := []struct {
		name           string
		jsonRPCError   *dinHttp.JSONRPCError
		expectedResult bool
	}{
		{
			name:           "Nil error",
			jsonRPCError:   nil,
			expectedResult: false,
		},
		{
			name: "Parse error (not retryable)",
			jsonRPCError: &dinHttp.JSONRPCError{
				Code:    -32700,
				Message: "Parse error",
			},
			expectedResult: false,
		},
		{
			name: "Invalid request (not retryable)",
			jsonRPCError: &dinHttp.JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
			},
			expectedResult: false,
		},
		{
			name: "Method not found (not retryable)",
			jsonRPCError: &dinHttp.JSONRPCError{
				Code:    -32601,
				Message: "Method not found",
			},
			expectedResult: false,
		},
		{
			name: "Invalid params (not retryable)",
			jsonRPCError: &dinHttp.JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
			},
			expectedResult: false,
		},
		{
			name: "Internal error (retryable)",
			jsonRPCError: &dinHttp.JSONRPCError{
				Code:    -32603,
				Message: "Internal error",
			},
			expectedResult: true,
		},
		{
			name: "Server error -32000 (retryable)",
			jsonRPCError: &dinHttp.JSONRPCError{
				Code:    -32000,
				Message: "Server error",
			},
			expectedResult: true,
		},
		{
			name: "Rate limit error -32005 (retryable)",
			jsonRPCError: &dinHttp.JSONRPCError{
				Code:    -32005,
				Message: "Limit exceeded",
			},
			expectedResult: true,
		},
		{
			name: "Server error -32050 (retryable)",
			jsonRPCError: &dinHttp.JSONRPCError{
				Code:    -32050,
				Message: "Server overloaded",
			},
			expectedResult: true,
		},
		{
			name: "Server error -32099 (retryable)",
			jsonRPCError: &dinHttp.JSONRPCError{
				Code:    -32099,
				Message: "Server error",
			},
			expectedResult: true,
		},
		{
			name: "Custom error with timeout message (retryable)",
			jsonRPCError: &dinHttp.JSONRPCError{
				Code:    -40000,
				Message: "Request timeout occurred",
			},
			expectedResult: true,
		},
		{
			name: "Custom error with connection message (retryable)",
			jsonRPCError: &dinHttp.JSONRPCError{
				Code:    -40001,
				Message: "Connection failed",
			},
			expectedResult: true,
		},
		{
			name: "Custom error with network message (retryable)",
			jsonRPCError: &dinHttp.JSONRPCError{
				Code:    -40002,
				Message: "Network unavailable",
			},
			expectedResult: true,
		},
		{
			name: "Custom error with rate limit message (retryable)",
			jsonRPCError: &dinHttp.JSONRPCError{
				Code:    -40003,
				Message: "Too many requests",
			},
			expectedResult: true,
		},
		{
			name: "Custom error with server error message (retryable)",
			jsonRPCError: &dinHttp.JSONRPCError{
				Code:    -40004,
				Message: "Internal server error",
			},
			expectedResult: true,
		},
		{
			name: "Custom error with method not found message (not retryable)",
			jsonRPCError: &dinHttp.JSONRPCError{
				Code:    -40005,
				Message: "Method not found",
			},
			expectedResult: false,
		},
		{
			name: "Custom error with invalid params message (not retryable)",
			jsonRPCError: &dinHttp.JSONRPCError{
				Code:    -40006,
				Message: "Invalid params provided",
			},
			expectedResult: false,
		},
		{
			name: "Custom error with unauthorized message (not retryable)",
			jsonRPCError: &dinHttp.JSONRPCError{
				Code:    -40007,
				Message: "Unauthorized access",
			},
			expectedResult: false,
		},
		{
			name: "Custom error with forbidden message (not retryable)",
			jsonRPCError: &dinHttp.JSONRPCError{
				Code:    -40008,
				Message: "Forbidden request",
			},
			expectedResult: false,
		},
		{
			name: "Unknown error code with no message (retryable by default)",
			jsonRPCError: &dinHttp.JSONRPCError{
				Code:    -50000,
				Message: "",
			},
			expectedResult: true,
		},
		{
			name: "Unknown error code with generic message (retryable by default)",
			jsonRPCError: &dinHttp.JSONRPCError{
				Code:    -50001,
				Message: "Something went wrong",
			},
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isJSONRPCErrorRetryable(tt.jsonRPCError)
			assert.Equal(t, tt.expectedResult, result, "Expected %v but got %v for error: %+v", tt.expectedResult, result, tt.jsonRPCError)
		})
	}
}
