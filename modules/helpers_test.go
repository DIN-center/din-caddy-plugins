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
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
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

func TestProcessHCMethodResponseAsyncLogging(t *testing.T) {
	tests := []struct {
		name           string
		respBody       []byte
		respStatus     int
		method         string
		expectLogCall  bool
		expectLogLevel zapcore.Level
		expectLogMsg   string
	}{
		{
			name:           "JSON parsing error should trigger robust logging",
			respBody:       []byte(`{"invalid": json}`),
			respStatus:     200,
			method:         "eth_blockNumber",
			expectLogCall:  true,
			expectLogLevel: zapcore.WarnLevel,
			expectLogMsg:   "Request attempt failed, initiating retry",
		},
		{
			name:           "Non-200 status should trigger robust logging",
			respBody:       []byte(`{"jsonrpc":"2.0","id":1,"result":"0x64"}`),
			respStatus:     500,
			method:         "eth_blockNumber",
			expectLogCall:  true,
			expectLogLevel: zapcore.WarnLevel,
			expectLogMsg:   "Request attempt failed, initiating retry",
		},
		{
			name:           "JSON-RPC error should trigger robust logging",
			respBody:       []byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"Server error"}}`),
			respStatus:     200,
			method:         "eth_blockNumber",
			expectLogCall:  true,
			expectLogLevel: zapcore.WarnLevel,
			expectLogMsg:   "Request attempt failed, initiating retry",
		},
		{
			name:           "Method mismatch should not trigger failure logging",
			respBody:       []byte(`{"jsonrpc":"2.0","id":1,"result":"0x64"}`),
			respStatus:     200,
			method:         "eth_getBalance",
			expectLogCall:  false,
			expectLogLevel: zapcore.DebugLevel,
			expectLogMsg:   "",
		},
		{
			name:           "Empty response body should not trigger failure logging",
			respBody:       []byte{},
			respStatus:     200,
			method:         "eth_blockNumber",
			expectLogCall:  false,
			expectLogLevel: zapcore.DebugLevel,
			expectLogMsg:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create observed logger
			observedZapCore, observedLogs := observer.New(zap.DebugLevel)
			observedLogger := zap.New(observedZapCore)
			loggerClient := &logger.LoggerClient{Logger: observedLogger}

			// Create test network with CaddyPort to avoid getBlockByNumber errors
			network := &network{
				Name:      "test/eth",
				HCMethod:  "eth_blockNumber",
				logger:    loggerClient,
				CaddyPort: "8080", // Set CaddyPort to avoid errors in successful case
			}

			// Create test middleware
			middleware := &DinMiddleware{
				logger: loggerClient,
			}

			// Run the function
			middleware.processHCMethodResponseAsync(network, "test/eth", tt.respBody, tt.respStatus, tt.method)

			// Give the goroutine time to complete
			time.Sleep(200 * time.Millisecond)

			// Check logs
			logs := observedLogs.All()

			if tt.expectLogCall {
				// Should have at least one log entry with the expected message
				found := false
				for _, log := range logs {
					if log.Level == tt.expectLogLevel && log.Message == tt.expectLogMsg {
						found = true

						// Verify log contains expected fields
						fields := log.ContextMap()
						assert.Contains(t, fields, "network")
						assert.Contains(t, fields, "provider")
						assert.Contains(t, fields, "requestMethod")
						assert.Equal(t, "test/eth", fields["network"])
						assert.Equal(t, "din", fields["provider"])
						assert.Equal(t, tt.method, fields["requestMethod"])

						// Check for error-specific fields based on the test case
						if tt.respStatus != 200 {
							assert.Contains(t, fields, "statusCodeOfFailure")
							assert.Equal(t, int64(tt.respStatus), fields["statusCodeOfFailure"])
						}

						break
					}
				}
				assert.True(t, found, "Expected log message '%s' with level %s not found in logs", tt.expectLogMsg, tt.expectLogLevel)
			} else {
				// Should not have any failure logs with the expected message
				for _, log := range logs {
					assert.NotEqual(t, "Request attempt failed, initiating retry", log.Message, "Unexpected failure log found")
				}
			}
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
			middleware := &DinMiddleware{
				logger: loggerClient,
			}

			// Create request with test context
			req := httptest.NewRequest("POST", "/"+tt.networkPath, nil)
			req = req.WithContext(tt.setupCtx())

			// Call the function
			err := middleware.checkRequestContext(req, tt.networkPath, tt.attempt)

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
			expectedLogLevel:   zapcore.WarnLevel,
			expectedLogMsg:     "Request context timeout",
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
			expectedLogLevel:   zapcore.WarnLevel,
			expectedLogMsg:     "Request context timeout",
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
			expectedLogMsg:     "Request context error",
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
			expectedLogLevel:   zapcore.WarnLevel,
			expectedLogMsg:     "Request context timeout",
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
			expectedLogLevel:   zapcore.WarnLevel,
			expectedLogMsg:     "Request context timeout",
			expectedResponse:   `{"error": "Request timeout exceeded", "code": 504}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create observed logger
			observedZapCore, observedLogs := observer.New(zap.DebugLevel)
			observedLogger := zap.New(observedZapCore)
			loggerClient := &logger.LoggerClient{Logger: observedLogger}

			// Create test middleware
			middleware := &DinMiddleware{
				logger: loggerClient,
			}

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

			// Call the function
			middleware.handleContextCancellation(rw, req, tt.networkPath, tt.attempt, err, reqStartTime)

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

					// Verify required fields
					assert.Equal(t, tt.networkPath, fields["network"])
					assert.Equal(t, int64(tt.attempt+1), fields["attempt"])
					assert.Equal(t, int64(tt.expectedStatusCode), fields["status_code"])
					assert.Contains(t, fields, "duration")
					assert.Equal(t, "POST", fields["method"])
					assert.Equal(t, "/"+tt.networkPath, fields["path"])
					assert.Contains(t, fields, "remote_addr")
					assert.Contains(t, fields, "request_body")
					assert.Equal(t, tt.expectedResponse, fields["response_body"])
					assert.Contains(t, fields, "error")

					// Verify provider field
					if _, ok := repl.Get(RequestProviderKey); ok {
						assert.Contains(t, fields, "provider")
						assert.NotEqual(t, "No provider selected before cancellation", fields["provider"])
					} else {
						assert.Equal(t, "No provider selected before cancellation", fields["provider"])
					}

					// Verify request body truncation for long body test
					if tt.name == "Long request body should be truncated in logs" {
						requestBody := fields["request_body"].(string)
						assert.Contains(t, requestBody, "... (truncated)")
						assert.LessOrEqual(t, len(requestBody), 1015) // 1000 + "... (truncated)"
					}

					break
				}
			}
			assert.True(t, found, "Expected log message '%s' with level %s not found", tt.expectedLogMsg, tt.expectedLogLevel)
		})
	}
}
