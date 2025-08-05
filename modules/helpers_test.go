package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
)

func TestCheckForJSONRPCError(t *testing.T) {
	tests := []struct {
		name           string
		responseBody   []byte
		expectedError  *din_http.JSONRPCError
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
			expectedError: &din_http.JSONRPCError{
				Code:    -32601,
				Message: "Method not found",
			},
			expectNilError: false,
		},
		{
			name:         "JSON-RPC error with data field",
			responseBody: []byte(`{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid params","data":"Additional error info"},"id":1}`),
			expectedError: &din_http.JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    "Additional error info",
			},
			expectNilError: false,
		},
		{
			name:         "JSON-RPC error response with complex data",
			responseBody: []byte(`{"jsonrpc":"2.0","error":{"code":-32603,"message":"Internal error","data":{"details":"Server overloaded","retry_after":30}},"id":1}`),
			expectedError: &din_http.JSONRPCError{
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
			expectedError: &din_http.JSONRPCError{
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
		wantRequest   *din_http.JSONRPCRequest
		wantErr       bool
		expectErrStr  string
	}{
		{
			name: "Successful retrieval and unmarshal",
			setupReplacer: func(repl *caddy.Replacer) {
				req := din_http.JSONRPCRequest{Method: "test_method", JSONRPC: "2.0"}
				bodyBytes, _ := json.Marshal(req)
				repl.Set(RequestBodyKey, bodyBytes)
			},
			wantRequest: &din_http.JSONRPCRequest{
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
		jsonRPCError   *din_http.JSONRPCError
		expectedResult bool
	}{
		{
			name:           "Nil error",
			jsonRPCError:   nil,
			expectedResult: false,
		},
		{
			name: "Parse error (not retryable)",
			jsonRPCError: &din_http.JSONRPCError{
				Code:    -32700,
				Message: "Parse error",
			},
			expectedResult: false,
		},
		{
			name: "Invalid request (not retryable)",
			jsonRPCError: &din_http.JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
			},
			expectedResult: false,
		},
		{
			name: "Method not found (not retryable)",
			jsonRPCError: &din_http.JSONRPCError{
				Code:    -32601,
				Message: "Method not found",
			},
			expectedResult: false,
		},
		{
			name: "Invalid params (not retryable)",
			jsonRPCError: &din_http.JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
			},
			expectedResult: false,
		},
		{
			name: "Internal error (retryable)",
			jsonRPCError: &din_http.JSONRPCError{
				Code:    -32603,
				Message: "Internal error",
			},
			expectedResult: true,
		},
		{
			name: "Server error -32000 (retryable)",
			jsonRPCError: &din_http.JSONRPCError{
				Code:    -32000,
				Message: "Server error",
			},
			expectedResult: true,
		},
		{
			name: "Rate limit error -32005 (retryable)",
			jsonRPCError: &din_http.JSONRPCError{
				Code:    -32005,
				Message: "Limit exceeded",
			},
			expectedResult: true,
		},
		{
			name: "Server error -32050 (retryable)",
			jsonRPCError: &din_http.JSONRPCError{
				Code:    -32050,
				Message: "Server overloaded",
			},
			expectedResult: true,
		},
		{
			name: "Server error -32099 (retryable)",
			jsonRPCError: &din_http.JSONRPCError{
				Code:    -32099,
				Message: "Server error",
			},
			expectedResult: true,
		},
		{
			name: "Custom error with timeout message (retryable)",
			jsonRPCError: &din_http.JSONRPCError{
				Code:    -40000,
				Message: "Request timeout occurred",
			},
			expectedResult: true,
		},
		{
			name: "Custom error with connection message (retryable)",
			jsonRPCError: &din_http.JSONRPCError{
				Code:    -40001,
				Message: "Connection failed",
			},
			expectedResult: true,
		},
		{
			name: "Custom error with network message (retryable)",
			jsonRPCError: &din_http.JSONRPCError{
				Code:    -40002,
				Message: "Network unavailable",
			},
			expectedResult: true,
		},
		{
			name: "Custom error with rate limit message (retryable)",
			jsonRPCError: &din_http.JSONRPCError{
				Code:    -40003,
				Message: "Too many requests",
			},
			expectedResult: true,
		},
		{
			name: "Custom error with server error message (retryable)",
			jsonRPCError: &din_http.JSONRPCError{
				Code:    -40004,
				Message: "Internal server error",
			},
			expectedResult: true,
		},
		{
			name: "Custom error with method not found message (not retryable)",
			jsonRPCError: &din_http.JSONRPCError{
				Code:    -40005,
				Message: "Method not found",
			},
			expectedResult: false,
		},
		{
			name: "Custom error with invalid params message (not retryable)",
			jsonRPCError: &din_http.JSONRPCError{
				Code:    -40006,
				Message: "Invalid params provided",
			},
			expectedResult: false,
		},
		{
			name: "Custom error with unauthorized message (not retryable)",
			jsonRPCError: &din_http.JSONRPCError{
				Code:    -40007,
				Message: "Unauthorized access",
			},
			expectedResult: false,
		},
		{
			name: "Custom error with forbidden message (not retryable)",
			jsonRPCError: &din_http.JSONRPCError{
				Code:    -40008,
				Message: "Forbidden request",
			},
			expectedResult: false,
		},
		{
			name: "Unknown error code with no message (retryable by default)",
			jsonRPCError: &din_http.JSONRPCError{
				Code:    -50000,
				Message: "",
			},
			expectedResult: true,
		},
		{
			name: "Unknown error code with generic message (retryable by default)",
			jsonRPCError: &din_http.JSONRPCError{
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

func TestCheckRequestContext(t *testing.T) {
	tests := []struct {
		name        string
		setupCtx    func() context.Context
		networkPath string
		attempt     int
		expectError bool
		expectMsg   string
	}{
		{
			name: "Active context should return nil",
			setupCtx: func() context.Context {
				return context.Background()
			},
			networkPath: "ethereum-mainnet",
			attempt:     0,
			expectError: false,
			expectMsg:   "",
		},
		{
			name: "Cancelled context should return client cancellation error",
			setupCtx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Cancel immediately
				return ctx
			},
			networkPath: "ethereum-mainnet",
			attempt:     1,
			expectError: true,
			expectMsg:   "request cancelled by client",
		},
		{
			name: "Deadline exceeded context should return timeout error",
			setupCtx: func() context.Context {
				ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				defer cancel()
				return ctx
			},
			networkPath: "polygon-mainnet",
			attempt:     2,
			expectError: true,
			expectMsg:   "request deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create observed logger
			observedZapCore, observedLogs := observer.New(zap.DebugLevel)
			observedLogger := zap.New(observedZapCore)
			loggerClient := &logger.LoggerClient{Logger: observedLogger}

			// Create test middleware

			// Create request with test context
			req := httptest.NewRequest("POST", "/"+tt.networkPath, nil)
			req = req.WithContext(tt.setupCtx())

			// Call the function
			err := checkRequestContext(loggerClient, req, tt.networkPath, tt.attempt)

			// Verify results
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectMsg)

				// Verify debug log was created
				logs := observedLogs.All()
				assert.NotEmpty(t, logs)

				// Find the relevant log entry
				found := false
				for _, log := range logs {
					if log.Level == zapcore.DebugLevel {
						fields := log.ContextMap()
						if fields["network"] == tt.networkPath && fields["attempt"] == int64(tt.attempt+1) {
							found = true
							break
						}
					}
				}
				assert.True(t, found, "Expected debug log not found")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHandleContextCancellation(t *testing.T) {
	tests := []struct {
		name               string
		errorMsg           string
		networkPath        string
		attempt            int
		setupReplacer      func(repl *caddy.Replacer)
		expectedStatusCode int
		expectedLogLevel   zapcore.Level
		expectedLogMsg     string
		expectedResponse   string
	}{
		{
			name:        "Client cancellation should return 408",
			errorMsg:    "request cancelled by client",
			networkPath: "ethereum-mainnet",
			attempt:     1,
			setupReplacer: func(repl *caddy.Replacer) {
				repl.Set(RequestBodyKey, []byte(`{"jsonrpc":"2.0","method":"eth_getBalance","id":1}`))
				repl.Set(RequestProviderKey, "https://eth-mainnet.g.alchemy.com/v2/test")
			},
			expectedStatusCode: http.StatusRequestTimeout,
			expectedLogLevel:   zapcore.ErrorLevel,
			expectedLogMsg:     "Request attempt failed",
			expectedResponse:   `{"error": "Request cancelled by client", "code": 408}`,
		},
		{
			name:        "Deadline exceeded should return 504",
			errorMsg:    "request deadline exceeded",
			networkPath: "polygon-mainnet",
			attempt:     2,
			setupReplacer: func(repl *caddy.Replacer) {
				repl.Set(RequestBodyKey, []byte(`{"jsonrpc":"2.0","method":"eth_blockNumber","id":2}`))
				repl.Set(RequestProviderKey, "https://polygon-rpc.com")
			},
			expectedStatusCode: http.StatusGatewayTimeout,
			expectedLogLevel:   zapcore.ErrorLevel,
			expectedLogMsg:     "Request attempt failed",
			expectedResponse:   `{"error": "Request timeout exceeded", "code": 504}`,
		},
		{
			name:        "Other context error should return 503",
			errorMsg:    "context error: some other error",
			networkPath: "arbitrum-mainnet",
			attempt:     0,
			setupReplacer: func(repl *caddy.Replacer) {
				repl.Set(RequestBodyKey, []byte(`{"jsonrpc":"2.0","method":"eth_call","id":3}`))
				// No provider set to test "No provider selected" case
			},
			expectedStatusCode: http.StatusServiceUnavailable,
			expectedLogLevel:   zapcore.ErrorLevel,
			expectedLogMsg:     "Request attempt failed",
			expectedResponse:   `{"error": "Service unavailable", "code": 503}`,
		},
		{
			name:        "Long request body should be truncated in logs",
			errorMsg:    "request cancelled by client",
			networkPath: "ethereum-mainnet",
			attempt:     0,
			setupReplacer: func(repl *caddy.Replacer) {
				// Create a request body longer than 1000 characters
				longBody := `{"jsonrpc":"2.0","method":"eth_call","params":[{"to":"0x` +
					string(make([]byte, 1000)) + `","data":"0x123"}],"id":1}`
				repl.Set(RequestBodyKey, []byte(longBody))
				repl.Set(RequestProviderKey, "https://eth-mainnet.infura.io/v3/test")
			},
			expectedStatusCode: http.StatusRequestTimeout,
			expectedLogLevel:   zapcore.ErrorLevel,
			expectedLogMsg:     "Request attempt failed",
			expectedResponse:   `{"error": "Request cancelled by client", "code": 408}`,
		},
		{
			name:        "Missing request body and provider should handle gracefully",
			errorMsg:    "request deadline exceeded",
			networkPath: "optimism-mainnet",
			attempt:     3,
			setupReplacer: func(repl *caddy.Replacer) {
				// Don't set any keys to test empty cases
			},
			expectedStatusCode: http.StatusGatewayTimeout,
			expectedLogLevel:   zapcore.ErrorLevel,
			expectedLogMsg:     "Request attempt failed",
			expectedResponse:   `{"error": "Request timeout exceeded", "code": 504}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create observed logger
			observedZapCore, observedLogs := observer.New(zap.DebugLevel)
			observedLogger := zap.New(observedZapCore)
			loggerClient := &logger.LoggerClient{Logger: observedLogger}

			// Create request with replacer context
			req := httptest.NewRequest("POST", "/"+tt.networkPath, bytes.NewReader([]byte("test")))
			repl := caddy.NewReplacer()
			tt.setupReplacer(repl)
			ctx := context.WithValue(req.Context(), caddy.ReplacerCtxKey, repl)
			req = req.WithContext(ctx)

			// Create response recorder
			rw := httptest.NewRecorder()

			// Create error and start time
			err := fmt.Errorf(tt.errorMsg)
			reqStartTime := time.Now().Add(-5 * time.Second) // Simulate 5 second duration

			// Create a test network object
			testNetwork := &network{
				RequestAttemptCount: 3, // Default for testing
			}

			// Call the function
			handleContextCancellation(loggerClient, nil, rw, req, tt.networkPath, tt.attempt, err, reqStartTime, testNetwork)

			// Verify HTTP response
			assert.Equal(t, tt.expectedStatusCode, rw.Code)
			assert.Equal(t, "application/json", rw.Header().Get("Content-Type"))
			assert.JSONEq(t, tt.expectedResponse, rw.Body.String())

			// Verify logs
			logs := observedLogs.All()
			assert.NotEmpty(t, logs)

			// Find the relevant log entry
			found := false
			for _, log := range logs {
				if log.Level == tt.expectedLogLevel && log.Message == tt.expectedLogMsg {
					found = true
					fields := log.ContextMap()

					// Verify required fields from logFailedAttempt
					assert.Equal(t, tt.networkPath, fields["network"])
					assert.Equal(t, int64(tt.attempt+1), fields["failed_attempt_number"])
					assert.Equal(t, int64(3), fields["max_attempts"]) // testNetwork.RequestAttemptCount
					assert.Equal(t, int64(tt.expectedStatusCode), fields["status_code"])
					assert.Equal(t, "Context cancellation", fields["reason"])
					assert.Contains(t, fields, "error")

					// Check for provider field
					if _, ok := repl.Get(RequestProviderKey); ok {
						assert.Contains(t, fields, "provider")
						assert.NotEqual(t, "unknown", fields["provider"])
					} else {
						// When no provider is set, it defaults to "unknown"
						assert.Equal(t, "unknown", fields["provider"])
					}

					// Check for request method if request body was parsed
					if parsedReqBody, ok := repl.Get(RequestBodyKey); ok && len(parsedReqBody.([]byte)) > 0 {
						assert.Contains(t, fields, "request_method")
						assert.Contains(t, fields, "raw_request_body")
					}

					break
				}
			}
			assert.True(t, found, "Expected log message '%s' with level %s not found", tt.expectedLogMsg, tt.expectedLogLevel)
		})
	}
}

func TestLogFailedAttempt(t *testing.T) {
	tests := []struct {
		name                          string
		networkPath                   string
		failedAttemptNumber           int
		maxAttempts                   int
		statusCodeOfFailure           int
		error                         error
		setupReplacer                 func(repl *caddy.Replacer)
		parsedReqBody                 *din_http.JSONRPCRequest
		jsonRPCError                  *din_http.JSONRPCError
		expectedErrorLogFields        map[string]interface{}
		expectDebugLogForParamFailure bool
		expectedDebugLogFields        map[string]interface{}
	}{
		{
			name:                   "basic log with no errors or complex params",
			networkPath:            "test-network",
			failedAttemptNumber:    1,
			maxAttempts:            3,
			statusCodeOfFailure:    500,
			setupReplacer:          func(repl *caddy.Replacer) { repl.Set(testRequestProviderKey, "test-provider") },
			parsedReqBody:          &din_http.JSONRPCRequest{Method: "test_method"},
			jsonRPCError:           nil,
			expectedErrorLogFields: map[string]interface{}{"network": "test-network", "provider": "test-provider", "failed_attempt_number": int64(1), "max_attempts": int64(3), "status_code": int64(500), "request_method": "test_method", "reason": "Test failure"},
		},
		{
			name:                   "with upstream error",
			networkPath:            "test-network-err",
			failedAttemptNumber:    2,
			maxAttempts:            3,
			statusCodeOfFailure:    503,
			error:                  fmt.Errorf("connection refused"),
			setupReplacer:          func(repl *caddy.Replacer) { repl.Set(testRequestProviderKey, "err-provider") },
			parsedReqBody:          &din_http.JSONRPCRequest{Method: "error_method"},
			jsonRPCError:           nil,
			expectedErrorLogFields: map[string]interface{}{"network": "test-network-err", "provider": "err-provider", "failed_attempt_number": int64(2), "max_attempts": int64(3), "status_code": int64(503), "request_method": "error_method", "error": "connection refused", "reason": "Test failure"},
		},
		{
			name:                   "with JSON-RPC error",
			networkPath:            "test-jsonrpc-error",
			failedAttemptNumber:    1,
			maxAttempts:            3,
			statusCodeOfFailure:    200,
			setupReplacer:          func(repl *caddy.Replacer) { repl.Set(testRequestProviderKey, "jsonrpc-provider") },
			parsedReqBody:          &din_http.JSONRPCRequest{Method: "eth_getBalance"},
			jsonRPCError:           &din_http.JSONRPCError{Code: -32601, Message: "Method not found"},
			expectedErrorLogFields: map[string]interface{}{"network": "test-jsonrpc-error", "provider": "jsonrpc-provider", "failed_attempt_number": int64(1), "max_attempts": int64(3), "status_code": int64(200), "request_method": "eth_getBalance", "reason": "Test failure"},
		},
		{
			name:                   "with JSON-RPC error and data",
			networkPath:            "test-jsonrpc-error-data",
			failedAttemptNumber:    2,
			maxAttempts:            3,
			statusCodeOfFailure:    200,
			setupReplacer:          func(repl *caddy.Replacer) { repl.Set(testRequestProviderKey, "jsonrpc-data-provider") },
			parsedReqBody:          &din_http.JSONRPCRequest{Method: "eth_call"},
			jsonRPCError:           &din_http.JSONRPCError{Code: -32603, Message: "Internal error", Data: map[string]interface{}{"details": "Server overloaded"}},
			expectedErrorLogFields: map[string]interface{}{"network": "test-jsonrpc-error-data", "provider": "jsonrpc-data-provider", "failed_attempt_number": int64(2), "max_attempts": int64(3), "status_code": int64(200), "request_method": "eth_call", "reason": "Test failure"},
		},
		{
			name:                   "with valid JSON array params",
			networkPath:            "test-params-array",
			failedAttemptNumber:    1,
			maxAttempts:            2,
			statusCodeOfFailure:    400,
			setupReplacer:          func(repl *caddy.Replacer) { repl.Set(testRequestProviderKey, "params-provider") },
			parsedReqBody:          &din_http.JSONRPCRequest{Method: "method_array_params", Params: json.RawMessage(`[1, "test", true]`)},
			jsonRPCError:           nil,
			expectedErrorLogFields: map[string]interface{}{"network": "test-params-array", "provider": "params-provider", "failed_attempt_number": int64(1), "max_attempts": int64(2), "status_code": int64(400), "request_method": "method_array_params", "request_params": []interface{}{float64(1), "test", true}, "reason": "Test failure"},
		},
		{
			name:                   "with valid JSON object params",
			networkPath:            "test-params-obj",
			failedAttemptNumber:    1,
			maxAttempts:            1,
			statusCodeOfFailure:    400,
			setupReplacer:          func(repl *caddy.Replacer) { repl.Set(testRequestProviderKey, "params-obj-provider") },
			parsedReqBody:          &din_http.JSONRPCRequest{Method: "method_obj_params", Params: json.RawMessage(`{"key":"value", "num":123}`)},
			jsonRPCError:           nil,
			expectedErrorLogFields: map[string]interface{}{"network": "test-params-obj", "provider": "params-obj-provider", "failed_attempt_number": int64(1), "max_attempts": int64(1), "status_code": int64(400), "request_method": "method_obj_params", "request_params": map[string]interface{}{"key": "value", "num": float64(123)}, "reason": "Test failure"},
		},
		{
			name:                          "with malformed JSON params",
			networkPath:                   "test-malformed-params",
			failedAttemptNumber:           1,
			maxAttempts:                   1,
			statusCodeOfFailure:           400,
			setupReplacer:                 func(repl *caddy.Replacer) { repl.Set(testRequestProviderKey, "malformed-provider") },
			parsedReqBody:                 &din_http.JSONRPCRequest{Method: "method_malformed", Params: json.RawMessage(`{"key":incomplete}`)},
			jsonRPCError:                  nil,
			expectedErrorLogFields:        map[string]interface{}{"network": "test-malformed-params", "provider": "malformed-provider", "failed_attempt_number": int64(1), "max_attempts": int64(1), "status_code": int64(400), "request_method": "method_malformed", "raw_request_params": `{"key":incomplete}`, "reason": "Test failure"},
			expectDebugLogForParamFailure: false, // Debug logging was removed as part of generic refactoring
			expectedDebugLogFields:        nil,
		},
		{
			name:                   "with JSON null params",
			networkPath:            "test-null-params",
			failedAttemptNumber:    1,
			maxAttempts:            1,
			statusCodeOfFailure:    400,
			setupReplacer:          func(repl *caddy.Replacer) { repl.Set(testRequestProviderKey, "null-params-provider") },
			parsedReqBody:          &din_http.JSONRPCRequest{Method: "method_null_params", Params: json.RawMessage(`null`)},
			jsonRPCError:           nil,
			expectedErrorLogFields: map[string]interface{}{"network": "test-null-params", "provider": "null-params-provider", "failed_attempt_number": int64(1), "max_attempts": int64(1), "status_code": int64(400), "request_method": "method_null_params", "reason": "Test failure"},
		},
		{
			name:                   "with empty params",
			networkPath:            "test-empty-params",
			failedAttemptNumber:    1,
			maxAttempts:            1,
			statusCodeOfFailure:    400,
			setupReplacer:          func(repl *caddy.Replacer) { repl.Set(testRequestProviderKey, "empty-params-provider") },
			parsedReqBody:          &din_http.JSONRPCRequest{Method: "method_empty_params", Params: json.RawMessage{}},
			jsonRPCError:           nil,
			expectedErrorLogFields: map[string]interface{}{"network": "test-empty-params", "provider": "empty-params-provider", "failed_attempt_number": int64(1), "max_attempts": int64(1), "status_code": int64(400), "request_method": "method_empty_params", "reason": "Test failure"},
		},
		{
			name:                "parsedReqBody is nil, fallback to raw snippet from replacer",
			networkPath:         "test-nil-parsedbody",
			failedAttemptNumber: 1,
			maxAttempts:         1,
			statusCodeOfFailure: 500,
			setupReplacer: func(repl *caddy.Replacer) {
				repl.Set(testRequestProviderKey, "nil-body-provider")
				repl.Set(testRequestBodyKey, []byte("this is a raw request body snippet that is very long indeed and should be truncated"))
			},
			parsedReqBody:          nil,
			jsonRPCError:           nil,
			expectedErrorLogFields: map[string]interface{}{"network": "test-nil-parsedbody", "provider": "nil-body-provider", "failed_attempt_number": int64(1), "max_attempts": int64(1), "status_code": int64(500), "raw_request_body": "this is a raw request body snippet that is very long indeed and should be truncated", "reason": "Test failure"},
		},
		{
			name:                   "parsedReqBody is nil, replacer has no body",
			networkPath:            "test-nil-parsedbody-no-snippet",
			failedAttemptNumber:    1,
			maxAttempts:            1,
			statusCodeOfFailure:    500,
			setupReplacer:          func(repl *caddy.Replacer) { repl.Set(testRequestProviderKey, "no-snippet-provider") },
			parsedReqBody:          nil,
			jsonRPCError:           nil,
			expectedErrorLogFields: map[string]interface{}{"network": "test-nil-parsedbody-no-snippet", "provider": "no-snippet-provider", "failed_attempt_number": int64(1), "max_attempts": int64(1), "status_code": int64(500), "reason": "Test failure"},
		},
		{
			name:                   "provider not in replacer, defaults to unknown",
			networkPath:            "test-unknown-provider",
			failedAttemptNumber:    1,
			maxAttempts:            1,
			statusCodeOfFailure:    500,
			setupReplacer:          func(repl *caddy.Replacer) { /* Provider key not set */ },
			parsedReqBody:          &din_http.JSONRPCRequest{Method: "some_method"},
			jsonRPCError:           nil,
			expectedErrorLogFields: map[string]interface{}{"network": "test-unknown-provider", "provider": "unknown", "failed_attempt_number": int64(1), "max_attempts": int64(1), "status_code": int64(500), "request_method": "some_method", "reason": "Test failure"},
		},
		{
			name:                   "provider in replacer but not a string",
			networkPath:            "test-provider-not-string",
			failedAttemptNumber:    1,
			maxAttempts:            1,
			statusCodeOfFailure:    500,
			setupReplacer:          func(repl *caddy.Replacer) { repl.Set(testRequestProviderKey, 12345) },
			parsedReqBody:          &din_http.JSONRPCRequest{Method: "another_method"},
			jsonRPCError:           nil,
			expectedErrorLogFields: map[string]interface{}{"network": "test-provider-not-string", "provider": "unknown", "failed_attempt_number": int64(1), "max_attempts": int64(1), "status_code": int64(500), "request_method": "another_method", "reason": "Test failure"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observed, logs := observer.New(zapcore.DebugLevel)
			testZapLogger := zap.New(observed)
			loggerClient := logger.NewLoggerClient(testZapLogger, utils.EnvTest)

			repl := caddy.NewReplacer()
			if tt.setupReplacer != nil {
				tt.setupReplacer(repl)
			}

			// Create raw response body if jsonRPCError is provided
			var rawResponseBody []byte
			if tt.jsonRPCError != nil {
				jsonRPCErrorBytes, err := json.Marshal(map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      1,
					"error":   tt.jsonRPCError,
				})
				require.NoError(t, err, "failed to marshal JSON-RPC error response for test")
				rawResponseBody = jsonRPCErrorBytes
			}

			// Extract method and params directly from JSONRPCRequest for the new logging structure
			var method string = "unknown"
			var params json.RawMessage
			if tt.parsedReqBody != nil {
				method = tt.parsedReqBody.Method
				if len(tt.parsedReqBody.Params) > 0 {
					params = tt.parsedReqBody.Params
				}
			}

			logFailedAttempt(LogFailedAttemptParams{
				Reason:              "Test failure",
				Logger:              loggerClient,
				NetworkPath:         tt.networkPath,
				FailedAttemptNumber: tt.failedAttemptNumber,
				MaxAttempts:         tt.maxAttempts,
				StatusCodeOfFailure: tt.statusCodeOfFailure,
				Error:               tt.error,
				Replacer:            repl,
				RequestMethod:       method,
				RequestParams:       params,
				RawResponseBody:     rawResponseBody,
			})

			foundErrorLog := false
			foundDebugLogForParamFailure := false

			allLogs := logs.All()
			require.NotEmpty(t, allLogs, "expected at least one log message")

			for _, loggedEntry := range allLogs {
				if loggedEntry.Level == zapcore.ErrorLevel && loggedEntry.Message == "Request attempt failed" {
					foundErrorLog = true
					for k, expectedV := range tt.expectedErrorLogFields {
						actualV, ok := loggedEntry.ContextMap()[k]
						require.True(t, ok, "expected field '%s' in warning log", k)

						if k == "error" {
							// Expect erroror to be logged as its string representation
							actualStr, isStr := actualV.(string)
							require.True(t, isStr, "expected field 'error' to be a string in log context, but got %T for key '%s'", actualV, k)

							expectedStr, isExpectedStr := expectedV.(string)
							require.True(t, isExpectedStr, "test expectation for 'error' (key '%s') should be a string", k)
							require.Equal(t, expectedStr, actualStr, "field 'error' (key '%s') string value mismatch", k)
						} else {
							// For all other fields, use direct comparison.
							// This correctly handles int64 expectations for numeric types logged with zap.Int.
							require.Equal(t, expectedV, actualV, "field '%s' value mismatch", k)
						}
					}
					// Check that no unexpected fields are in the warning log context (optional, can be strict)
					// require.Len(t, loggedEntry.ContextMap(), len(tt.expectedErrorLogFields), "warning log has unexpected number of fields")
				} else if loggedEntry.Level == zapcore.DebugLevel && loggedEntry.Message == "Async retry error log: Failed to unmarshal requestParams for structured logging, logging as raw string." {
					require.True(t, tt.expectDebugLogForParamFailure, "unexpected debug log for param failure")
					foundDebugLogForParamFailure = true
					if tt.expectedDebugLogFields != nil {
						for k, expectedV := range tt.expectedDebugLogFields {
							actualV, ok := loggedEntry.ContextMap()[k]
							require.True(t, ok, "expected field '%s' in debug log for param failure", k)
							require.Equal(t, expectedV, actualV, "field '%s' value mismatch in debug log for param failure", k)
						}
					}
				}
			}

			require.True(t, foundErrorLog, "expected error log 'Request attempt failed' was not found")
			if tt.expectDebugLogForParamFailure {
				require.True(t, foundDebugLogForParamFailure, "expected debug log for param failure was not found")
			}
		})
	}
}
