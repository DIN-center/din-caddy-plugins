package modules

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
	din "github.com/DIN-center/din-sc/apps/din-go/lib/din"
	dinreg "github.com/DIN-center/din-sc/apps/din-go/pkg/dinregistry"
	"github.com/caddyserver/caddy/v2"
	"github.com/pkg/errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

// Constants for replacer keys used in tests, mirroring those in din_middleware.go
const (
	testRequestProviderKey = "request_provider"
	testRequestBodyKey     = "request_body"
)

func TestLogFailedAttempt(t *testing.T) {
	tests := []struct {
		name                          string
		networkPath                   string
		failedAttemptNumber           int
		maxAttempts                   int
		statusCodeOfFailure           int
		upstreamErr                   error
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
			expectedErrorLogFields: map[string]interface{}{"network": "test-network", "provider": "test-provider", "failedAttemptNumber": int64(1), "maxAttempts": int64(3), "statusCodeOfFailure": int64(500), "requestMethod": "test_method"},
		},
		{
			name:                   "with upstream error",
			networkPath:            "test-network-err",
			failedAttemptNumber:    2,
			maxAttempts:            3,
			statusCodeOfFailure:    503,
			upstreamErr:            fmt.Errorf("connection refused"),
			setupReplacer:          func(repl *caddy.Replacer) { repl.Set(testRequestProviderKey, "err-provider") },
			parsedReqBody:          &din_http.JSONRPCRequest{Method: "error_method"},
			jsonRPCError:           nil,
			expectedErrorLogFields: map[string]interface{}{"network": "test-network-err", "provider": "err-provider", "failedAttemptNumber": int64(2), "maxAttempts": int64(3), "statusCodeOfFailure": int64(503), "requestMethod": "error_method", "upstreamError": "connection refused"},
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
			expectedErrorLogFields: map[string]interface{}{"network": "test-jsonrpc-error", "provider": "jsonrpc-provider", "failedAttemptNumber": int64(1), "maxAttempts": int64(3), "statusCodeOfFailure": int64(200), "requestMethod": "eth_getBalance", "jsonrpc_error_code": int64(-32601), "jsonrpc_error_message": "Method not found"},
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
			expectedErrorLogFields: map[string]interface{}{"network": "test-jsonrpc-error-data", "provider": "jsonrpc-data-provider", "failedAttemptNumber": int64(2), "maxAttempts": int64(3), "statusCodeOfFailure": int64(200), "requestMethod": "eth_call", "jsonrpc_error_code": int64(-32603), "jsonrpc_error_message": "Internal error", "jsonrpc_error_data": map[string]interface{}{"details": "Server overloaded"}},
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
			expectedErrorLogFields: map[string]interface{}{"network": "test-params-array", "provider": "params-provider", "failedAttemptNumber": int64(1), "maxAttempts": int64(2), "statusCodeOfFailure": int64(400), "requestMethod": "method_array_params", "requestParams": []interface{}{float64(1), "test", true}},
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
			expectedErrorLogFields: map[string]interface{}{"network": "test-params-obj", "provider": "params-obj-provider", "failedAttemptNumber": int64(1), "maxAttempts": int64(1), "statusCodeOfFailure": int64(400), "requestMethod": "method_obj_params", "requestParams": map[string]interface{}{"key": "value", "num": float64(123)}},
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
			expectedErrorLogFields:        map[string]interface{}{"network": "test-malformed-params", "provider": "malformed-provider", "failedAttemptNumber": int64(1), "maxAttempts": int64(1), "statusCodeOfFailure": int64(400), "requestMethod": "method_malformed", "rawRequestParams": `{"key":incomplete}`},
			expectDebugLogForParamFailure: true,
			expectedDebugLogFields:        map[string]interface{}{"rawParamsAttempted": `{"key":incomplete}`},
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
			expectedErrorLogFields: map[string]interface{}{"network": "test-null-params", "provider": "null-params-provider", "failedAttemptNumber": int64(1), "maxAttempts": int64(1), "statusCodeOfFailure": int64(400), "requestMethod": "method_null_params"},
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
			expectedErrorLogFields: map[string]interface{}{"network": "test-empty-params", "provider": "empty-params-provider", "failedAttemptNumber": int64(1), "maxAttempts": int64(1), "statusCodeOfFailure": int64(400), "requestMethod": "method_empty_params"},
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
			expectedErrorLogFields: map[string]interface{}{"network": "test-nil-parsedbody", "provider": "nil-body-provider", "failedAttemptNumber": int64(1), "maxAttempts": int64(1), "statusCodeOfFailure": int64(500), "rawRequestBodySnippet": "this is a raw request body snippet that is very long indeed and should be truncated"},
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
			expectedErrorLogFields: map[string]interface{}{"network": "test-nil-parsedbody-no-snippet", "provider": "no-snippet-provider", "failedAttemptNumber": int64(1), "maxAttempts": int64(1), "statusCodeOfFailure": int64(500)},
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
			expectedErrorLogFields: map[string]interface{}{"network": "test-unknown-provider", "provider": "unknown", "failedAttemptNumber": int64(1), "maxAttempts": int64(1), "statusCodeOfFailure": int64(500), "requestMethod": "some_method"},
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
			expectedErrorLogFields: map[string]interface{}{"network": "test-provider-not-string", "provider": "unknown", "failedAttemptNumber": int64(1), "maxAttempts": int64(1), "statusCodeOfFailure": int64(500), "requestMethod": "another_method"},
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

			logFailedAttempt(
				loggerClient,
				tt.networkPath,
				tt.failedAttemptNumber,
				tt.maxAttempts,
				tt.statusCodeOfFailure,
				tt.upstreamErr,
				repl,
				tt.parsedReqBody,
				tt.jsonRPCError,
			)

			foundErrorLog := false
			foundDebugLogForParamFailure := false

			allLogs := logs.All()
			require.NotEmpty(t, allLogs, "expected at least one log message")

			for _, loggedEntry := range allLogs {
				if loggedEntry.Level == zapcore.WarnLevel && loggedEntry.Message == "Request attempt failed, initiating retry" {
					foundErrorLog = true
					for k, expectedV := range tt.expectedErrorLogFields {
						actualV, ok := loggedEntry.ContextMap()[k]
						require.True(t, ok, "expected field '%s' in warning log", k)

						if k == "upstreamError" {
							// Expect upstreamError to be logged as its string representation
							actualStr, isStr := actualV.(string)
							require.True(t, isStr, "expected field 'upstreamError' to be a string in log context, but got %T for key '%s'", actualV, k)

							expectedStr, isExpectedStr := expectedV.(string)
							require.True(t, isExpectedStr, "test expectation for 'upstreamError' (key '%s') should be a string", k)
							require.Equal(t, expectedStr, actualStr, "field 'upstreamError' (key '%s') string value mismatch", k)
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

			require.True(t, foundErrorLog, "expected warning log 'Request attempt failed, initiating retry' was not found")
			if tt.expectDebugLogForParamFailure {
				require.True(t, foundDebugLogForParamFailure, "expected debug log for param failure was not found")
			}
		})
	}
}

func TestSyncRegistryWithLatestBlock(t *testing.T) {
	logger := logger.NewLoggerClient(zap.NewNop(), utils.Environment("test"))
	mockCtrl := gomock.NewController(t)
	mockDingoClient := din.NewMockIDingoClient(mockCtrl)
	dinMiddleware := &DinMiddleware{
		RegistryBlockEpoch:                  10,
		registryLastUpdatedEpochBlockNumber: 40,
		logger:                              logger,
		DingoClient:                         mockDingoClient,
		testMode:                            true,
	}

	tests := []struct {
		name                                string
		registryLastUpdatedEpochBlockNumber uint64
		latestBlockNumber                   uint64
		expectedUpdateCall                  bool
		expectedBlockFloorByEpoch           uint64
	}{
		{
			name:                                "Sync should update as block difference is equal to or exceeds epoch 50",
			registryLastUpdatedEpochBlockNumber: uint64(40),
			latestBlockNumber:                   uint64(50),
			expectedUpdateCall:                  true,
			expectedBlockFloorByEpoch:           uint64(50),
		},
		{
			name:                                "Sync should update as block difference is equal to or exceeds epoch 52",
			registryLastUpdatedEpochBlockNumber: uint64(40),
			latestBlockNumber:                   uint64(52),
			expectedUpdateCall:                  true,
			expectedBlockFloorByEpoch:           uint64(50),
		},
		{
			name:                                "Sync should update as block difference is equal to or exceeds epoch 1000",
			registryLastUpdatedEpochBlockNumber: uint64(40),
			latestBlockNumber:                   uint64(1001),
			expectedUpdateCall:                  true,
			expectedBlockFloorByEpoch:           uint64(1000),
		},
		{
			name:                                "Sync should not update as block difference is less than epoch 48",
			registryLastUpdatedEpochBlockNumber: uint64(40),
			latestBlockNumber:                   uint64(48),
			expectedUpdateCall:                  false,
			expectedBlockFloorByEpoch:           uint64(40),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dinMiddleware.Networks = map[string]*network{}
			dinMiddleware.registryLastUpdatedEpochBlockNumber = tt.registryLastUpdatedEpochBlockNumber

			mockDingoClient.EXPECT().GetLatestBlockNumber().Return(tt.latestBlockNumber, nil).Times(1)

			// Check if update was called as expected
			if tt.expectedUpdateCall {
				mockDingoClient.EXPECT().GetRegistryData().Return(&din.DinRegistryData{}, nil).Times(1)
			}
			// Call the function
			dinMiddleware.syncRegistryWithLatestBlock()

			// Validate that registryLastUpdatedEpochBlockNumber is updated correctly
			if dinMiddleware.registryLastUpdatedEpochBlockNumber != tt.expectedBlockFloorByEpoch {
				t.Errorf("Expected registryLastUpdatedEpochBlockNumber = %v, got %v", tt.expectedBlockFloorByEpoch, dinMiddleware.registryLastUpdatedEpochBlockNumber)
			}
		})
	}
}

func TestAddNetworkWithRegistryData(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tests := []struct {
		name                     string
		regNetwork               *din.Network
		syncNetworkConfigErr     error
		createNewProviderErr     error
		methodByBitErr           error
		expectedError            error
		expectedNetworkProviders int
		networkServiceCreated    bool
	}{
		{
			name: "Successful add network with providers",
			regNetwork: &din.Network{
				ProxyName: "test-network",
				Providers: map[string]*din.Provider{
					"Provider1": {
						NetworkServices: map[string]*din.NetworkService{
							"http://new-provider.com": {
								Url:     "http://new-provider.com",
								Address: "0x1234567890abcdef",
								Status:  dinreg.Active,
							},
						},
					},
				},
				NetworkConfig: &dinreg.NetworkConfig{
					HealthcheckMethodBit: uint8(1),
				},
			},
			methodByBitErr:           nil,
			syncNetworkConfigErr:     nil,
			createNewProviderErr:     nil,
			expectedError:            nil,
			expectedNetworkProviders: 1,
			networkServiceCreated:    true,
		},
		{
			name: "Error, missing active status",
			regNetwork: &din.Network{
				ProxyName: "test-network",
				Providers: map[string]*din.Provider{
					"Provider1": {
						NetworkServices: map[string]*din.NetworkService{
							"http://new-provider.com": {
								Url:     "http://new-provider.com",
								Address: "0x1234567890abcdef",
								Status:  dinreg.Onboarding,
							},
						},
					},
				},
				NetworkConfig: &dinreg.NetworkConfig{
					HealthcheckMethodBit: uint8(1),
				},
				Status: dinreg.Onboarding,
			},
			methodByBitErr:           nil,
			syncNetworkConfigErr:     nil,
			createNewProviderErr:     nil,
			expectedError:            nil,
			expectedNetworkProviders: 0,
			networkServiceCreated:    false,
		},
		{
			name: "Error syncing network config",
			regNetwork: &din.Network{
				ProxyName: "test-network",
				Providers: map[string]*din.Provider{
					"Provider1": {
						NetworkServices: map[string]*din.NetworkService{
							"http://new-provider.com": {
								Url:     "http://new-provider.com",
								Address: "0x1234567890abcdef",
								Status:  dinreg.Active,
							},
						},
					},
				},
				NetworkConfig: &dinreg.NetworkConfig{
					HealthcheckMethodBit: uint8(1),
				},
			},
			methodByBitErr:           errors.New(""),
			syncNetworkConfigErr:     nil,
			createNewProviderErr:     nil,
			expectedError:            errors.New(""),
			expectedNetworkProviders: 1,
			networkServiceCreated:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock DingoClient and other dependencies
			mockDingoClient := din.NewMockIDingoClient(mockCtrl)

			mockDingoClient.EXPECT().GetNetworkMethodNameByBit(gomock.Any(), gomock.Any()).Return("new-method", tt.methodByBitErr).AnyTimes()
			mockDingoClient.EXPECT().GetNetworkServiceMethods(gomock.Any()).Return([]*string{aws.String("eth_call"), aws.String("eth_blockNumber")}, nil).AnyTimes()

			// Create logger
			logger := logger.NewLoggerClient(zaptest.NewLogger(t), utils.Environment("test"))

			// Create DinMiddleware instance
			dinMiddleware := &DinMiddleware{
				DingoClient: mockDingoClient,
				logger:      logger,
				Networks:    make(map[string]*network),
				testMode:    true,
			}

			// Call the function being tested
			err := dinMiddleware.addNetworkWithRegistryData(tt.regNetwork)

			// Assert expected error
			if tt.expectedError != nil {
				assert.Equal(t, err.Error(), tt.expectedError.Error())
			} else {
				assert.NoError(t, err)

				// Verify the network is added
				network, ok := dinMiddleware.Networks[tt.regNetwork.ProxyName]
				assert.Equal(t, tt.networkServiceCreated, ok)

				if network != nil {
					// Verify the number of providers added to the network
					assert.Equal(t, tt.expectedNetworkProviders, len(network.Providers))
				}
			}
		})
	}
}

func TestUpdateNetworkWithRegistryData(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tests := []struct {
		name                       string
		regNetwork                 *din.Network
		newNetwork                 *network
		blockNumberMethodByBitErr  error
		chainIdMethodByBitErr      error
		callContractMethodByBitErr error
		syncNetworkConfigErr       error
		createNewProviderErr       error
		expectedError              error
		expectedProviderCount      int
		expectedRemainingProviders int
	}{
		{
			name: "Successful update with new providers",
			regNetwork: &din.Network{
				Name: "test-network",
				Providers: map[string]*din.Provider{
					"Provider1": {
						NetworkServices: map[string]*din.NetworkService{
							"http://new-provider.com": {
								Url:     "http://new-provider.com",
								Address: "0x1234567890abcdef",
								Status:  dinreg.Active,
							},
						},
					},
				},
				NetworkConfig: &dinreg.NetworkConfig{
					HealthcheckMethodBit: 1,
				},
			},
			newNetwork: &network{
				Name:      "test-network",
				Providers: map[string]*provider{},
			},
			blockNumberMethodByBitErr:  nil,
			chainIdMethodByBitErr:      nil,
			callContractMethodByBitErr: nil,
			syncNetworkConfigErr:       nil,
			createNewProviderErr:       nil,
			expectedError:              nil,
			expectedProviderCount:      1,
			expectedRemainingProviders: 0,
		},
		{
			name: "Error, non active status",
			regNetwork: &din.Network{
				Name: "test-network",
				Providers: map[string]*din.Provider{
					"Provider1": {
						NetworkServices: map[string]*din.NetworkService{
							"http://new-provider.com": {
								Url:     "http://new-provider.com",
								Address: "0x1234567890abcdef",
							},
						},
					},
				},
				NetworkConfig: &dinreg.NetworkConfig{
					HealthcheckMethodBit: 1,
				},
				Status: dinreg.Onboarding,
			},
			newNetwork: &network{
				Name:      "test-network",
				Providers: map[string]*provider{},
			},
			blockNumberMethodByBitErr:  nil,
			chainIdMethodByBitErr:      nil,
			callContractMethodByBitErr: nil,
			syncNetworkConfigErr:       nil,
			createNewProviderErr:       nil,
			expectedError:              nil,
			expectedProviderCount:      0,
			expectedRemainingProviders: 0,
		},
		{
			name: "Error syncing network config",
			regNetwork: &din.Network{
				Name: "test-network",
				NetworkConfig: &dinreg.NetworkConfig{
					HealthcheckMethodBit: 1,
				},
				Status: dinreg.Active,
			},
			newNetwork: &network{
				Name: "test-network",
			},
			blockNumberMethodByBitErr:  errors.New("sync error"),
			chainIdMethodByBitErr:      errors.New("sync error"),
			callContractMethodByBitErr: errors.New("sync error"),
			syncNetworkConfigErr:       nil,
			createNewProviderErr:       nil,
			expectedError:              errors.New("sync error"),
			expectedProviderCount:      0,
			expectedRemainingProviders: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock DingoClient and other dependencies
			mockDingoClient := din.NewMockIDingoClient(mockCtrl)

			// Create logger
			logger := logger.NewLoggerClient(zaptest.NewLogger(t), utils.Environment("test"))

			// Create DinMiddleware instance
			dinMiddleware := &DinMiddleware{
				DingoClient: mockDingoClient,
				logger:      logger,
				Networks: map[string]*network{
					tt.newNetwork.Name: tt.newNetwork,
				},
				testMode: true,
			}

			mockDingoClient.EXPECT().GetNetworkServiceMethods(gomock.Any()).Return([]*string{aws.String("eth_call"), aws.String("eth_blockNumber")}, nil).AnyTimes()
			mockDingoClient.EXPECT().GetNetworkMethodNameByBit(gomock.Any(), gomock.Any()).Return("new-method", tt.blockNumberMethodByBitErr).AnyTimes()
			mockDingoClient.EXPECT().GetNetworkMethodNameByBit(gomock.Any(), gomock.Any()).Return("new-method", tt.chainIdMethodByBitErr).AnyTimes()
			mockDingoClient.EXPECT().GetNetworkMethodNameByBit(gomock.Any(), gomock.Any()).Return("new-method", tt.callContractMethodByBitErr).AnyTimes()

			// Call the function being tested
			err := dinMiddleware.updateNetworkWithRegistryData(tt.regNetwork, tt.newNetwork)

			// Assert expected error
			if tt.expectedError != nil {
				assert.Equal(t, err.Error(), tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}

			// Assert the number of providers after the update
			assert.Equal(t, tt.expectedProviderCount, len(tt.newNetwork.Providers))
		})
	}
}

func TestSyncNetworkConfig(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tests := []struct {
		name                     string
		regNetwork               *din.Network
		existingNetwork          *network
		hcMethodName             string
		chainIDMethodName        string
		callContractMethodName   string
		archiveEnabled           bool
		getHCMethodErr           error
		callsHealthcheckMethod   bool
		getChainIDMethodErr      error
		callsChainIDMethod       bool
		getCallContractMethodErr error
		callsCallContractMethod  bool
		expectedError            error
		expectedNetwork          *network
	}{
		{
			name: "successful sync with all new values",
			regNetwork: &din.Network{
				Name: "test-network",
				NetworkConfig: &dinreg.NetworkConfig{
					HealthcheckMethodBit:    1,
					ChainIdMethodBit:        1,
					CallContractMethodBit:   1,
					ChainId:                 "0x1",
					HealthcheckIntervalSec:  20,
					BlockLagLimit:           10,
					BlockJumpLimit:          5,
					MaxRequestPayloadSizeKb: 2048,
					RequestAttemptCount:     5,
					ArchiveEnabled:          true,
				},
			},
			existingNetwork: &network{
				Name:                    "test-network",
				HCMethod:                "old-method",
				ChainIdMethod:           "old-chain-method",
				CallContractMethod:      "old-call-method",
				ChainId:                 "0x0",
				HCInterval:              10,
				BlockLagLimit:           5,
				BlockJumpLimit:          3,
				MaxRequestPayloadSizeKB: 1024,
				RequestAttemptCount:     3,
				ArchiveEnabled:          false,
			},
			hcMethodName:            "eth_blockNumber",
			callsHealthcheckMethod:  true,
			chainIDMethodName:       "eth_chainId",
			callsChainIDMethod:      true,
			callContractMethodName:  "eth_call",
			callsCallContractMethod: true,
			expectedNetwork: &network{
				Name:                    "test-network",
				HCMethod:                "eth_blockNumber",
				ChainIdMethod:           "eth_chainId",
				CallContractMethod:      "eth_call",
				ChainId:                 "0x1",
				HCInterval:              20,
				BlockLagLimit:           10,
				BlockJumpLimit:          5,
				MaxRequestPayloadSizeKB: 2048,
				RequestAttemptCount:     5,
				ArchiveEnabled:          true,
			},
		},
		{
			name: "error getting healthcheck method",
			regNetwork: &din.Network{
				Name: "test-network",
				NetworkConfig: &dinreg.NetworkConfig{
					HealthcheckMethodBit: 1,
				},
			},
			existingNetwork: &network{
				Name: "test-network",
			},
			getHCMethodErr:         errors.New("failed to get healthcheck method"),
			callsHealthcheckMethod: true,
			expectedError:          errors.New("failed to get network healthcheck method"),
		},
		{
			name: "error getting chain ID method",
			regNetwork: &din.Network{
				Name: "test-network",
				NetworkConfig: &dinreg.NetworkConfig{
					ChainIdMethodBit: 1,
				},
			},
			existingNetwork:        &network{Name: "test-network"},
			callsHealthcheckMethod: true,
			getChainIDMethodErr:    errors.New("failed to get chain ID method"),
			callsChainIDMethod:     true,
			expectedError:          errors.New("failed to get network chain ID method"),
		},
		{
			name: "error getting call contract method",
			regNetwork: &din.Network{
				Name: "test-network",
				NetworkConfig: &dinreg.NetworkConfig{
					CallContractMethodBit: 1,
				},
			},
			existingNetwork: &network{
				Name: "test-network",
			},
			callsHealthcheckMethod:   true,
			getCallContractMethodErr: errors.New("failed to get call contract method"),
			callsChainIDMethod:       true,
			callsCallContractMethod:  true,
			expectedError:            errors.New("failed to get network call contract method"),
		},
		{
			name: "no updates needed when registry values are zero",
			regNetwork: &din.Network{
				Name: "test-network",
				NetworkConfig: &dinreg.NetworkConfig{
					HealthcheckMethodBit:    1,
					ChainIdMethodBit:        1,
					ChainId:                 "0x1",
					HealthcheckIntervalSec:  0,
					BlockLagLimit:           0,
					BlockJumpLimit:          0,
					MaxRequestPayloadSizeKb: 0,
					RequestAttemptCount:     0,
					ArchiveEnabled:          false,
				},
			},
			existingNetwork: &network{
				Name:                    "test-network",
				HCMethod:                "eth_blockNumber",
				ChainIdMethod:           "eth_chainId",
				CallContractMethod:      "eth_call",
				ChainId:                 "0x1",
				HCInterval:              10,
				BlockLagLimit:           5,
				BlockJumpLimit:          3,
				MaxRequestPayloadSizeKB: 1024,
				RequestAttemptCount:     3,
				ArchiveEnabled:          false,
			},
			callsHealthcheckMethod:  true,
			callsChainIDMethod:      true,
			callsCallContractMethod: true,
			hcMethodName:            "eth_blockNumber",
			chainIDMethodName:       "eth_chainId",
			callContractMethodName:  "eth_call",
			expectedNetwork: &network{
				Name:                    "test-network",
				HCMethod:                "eth_blockNumber",
				ChainIdMethod:           "eth_chainId",
				CallContractMethod:      "eth_call",
				ChainId:                 "0x1",
				HCInterval:              10,
				BlockLagLimit:           5,
				BlockJumpLimit:          3,
				MaxRequestPayloadSizeKB: 1024,
				RequestAttemptCount:     3,
				ArchiveEnabled:          false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDingoClient := din.NewMockIDingoClient(mockCtrl)

			// Create logger
			logger := logger.NewLoggerClient(zaptest.NewLogger(t), utils.Environment("test"))

			if tt.callsHealthcheckMethod {
				mockDingoClient.EXPECT().
					GetNetworkMethodNameByBit(tt.regNetwork.Name, tt.regNetwork.NetworkConfig.HealthcheckMethodBit).
					Return(tt.hcMethodName, tt.getHCMethodErr).Times(1)
			}

			if tt.callsChainIDMethod {
				mockDingoClient.EXPECT().
					GetNetworkMethodNameByBit(tt.regNetwork.Name, tt.regNetwork.NetworkConfig.ChainIdMethodBit).
					Return(tt.chainIDMethodName, tt.getChainIDMethodErr).Times(1)
			}

			if tt.callsCallContractMethod {
				mockDingoClient.EXPECT().
					GetNetworkMethodNameByBit(tt.regNetwork.Name, tt.regNetwork.NetworkConfig.CallContractMethodBit).
					Return(tt.callContractMethodName, tt.getCallContractMethodErr).Times(1)
			}

			dinMiddleware := &DinMiddleware{
				DingoClient: mockDingoClient,
				logger:      logger,
			}

			result, err := dinMiddleware.syncNetworkConfig(tt.regNetwork, tt.existingNetwork)
			if tt.expectedError != nil {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedNetwork.HCMethod, result.HCMethod)
			assert.Equal(t, tt.expectedNetwork.ChainIdMethod, result.ChainIdMethod)
			assert.Equal(t, tt.expectedNetwork.ChainId, result.ChainId)
			assert.Equal(t, tt.expectedNetwork.CallContractMethod, result.CallContractMethod)
			assert.Equal(t, tt.expectedNetwork.HCInterval, result.HCInterval)
			assert.Equal(t, tt.expectedNetwork.BlockLagLimit, result.BlockLagLimit)
			assert.Equal(t, tt.expectedNetwork.BlockJumpLimit, result.BlockJumpLimit)
			assert.Equal(t, tt.expectedNetwork.MaxRequestPayloadSizeKB, result.MaxRequestPayloadSizeKB)
			assert.Equal(t, tt.expectedNetwork.RequestAttemptCount, result.RequestAttemptCount)
			assert.Equal(t, tt.expectedNetwork.ArchiveEnabled, result.ArchiveEnabled)
		})
	}
}

func expectedMethodsMap(m []*string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, k := range m {
		result[*k] = struct{}{}
	}
	return result
}

func TestCreateNewProvider(t *testing.T) {
	tests := []struct {
		name                  string
		provider              *provider
		authConfig            *dinreg.NetworkServiceAuthConfig
		networkServiceAddress string
		initializeProviderErr error
		getMethodsErr         error
		expectedError         error
		expectedMethods       []*string
		expectedAuth          *siwe.SIWEClientAuth
		expectAuthCreation    bool
	}{
		{
			name: "Successful provider creation with SIWE auth",
			provider: &provider{
				HttpUrl: "http://example5.com",
			},
			authConfig: &dinreg.NetworkServiceAuthConfig{
				Type: dinreg.SIWE,
				Url:  "http://example6.com",
			},
			networkServiceAddress: "0x1234567890abcdef",
			initializeProviderErr: nil,
			getMethodsErr:         nil,
			expectedError:         nil,
			expectedMethods:       []*string{aws.String("eth_call"), aws.String("eth_blockNumber")},
			expectedAuth: &siwe.SIWEClientAuth{
				ProviderURL:  "http://example6.com",
				SessionCount: 16,
				Signer:       nil, // Signer is set in the createNewProvider function
			},
			expectAuthCreation: true,
		},
		{
			name: "Successful provider creation with no auth",
			provider: &provider{
				HttpUrl: "http://example7.com",
			},
			authConfig:            &dinreg.NetworkServiceAuthConfig{Type: dinreg.None},
			networkServiceAddress: "0x1234567890abcdef",
			initializeProviderErr: nil,
			getMethodsErr:         nil,
			expectedError:         nil,
			expectedMethods:       []*string{aws.String("eth_call"), aws.String("eth_blockNumber")},
			expectedAuth:          nil,
			expectAuthCreation:    false,
		},
		{
			name: "Error fetching network service methods",
			provider: &provider{
				HttpUrl: "http://example8.com",
			},
			authConfig:            &dinreg.NetworkServiceAuthConfig{Type: dinreg.SIWE, Url: "http://example9.com"},
			networkServiceAddress: "0x1234567890abcdef",
			initializeProviderErr: nil,
			getMethodsErr:         errors.New("failed to fetch methods"),
			expectedError:         errors.New("failed to get network service methods: failed to fetch methods"),
			expectedMethods:       nil,
			expectedAuth: &siwe.SIWEClientAuth{
				ProviderURL:  "http://example9.com",
				SessionCount: 16,
				Signer:       nil,
			},
			expectAuthCreation: true,
		},
		{
			name: "AUTH type is unknown ",
			provider: &provider{
				HttpUrl: "http://example12.com",
			},
			authConfig: &dinreg.NetworkServiceAuthConfig{
				Type: "unknown",
				Url:  "http://example13.com",
			},
			networkServiceAddress: "0x1234567890abcdef",
			initializeProviderErr: nil,
			getMethodsErr:         nil,
			expectedError:         nil,
			expectedMethods:       []*string{aws.String("eth_call"), aws.String("eth_blockNumber")},
			expectedAuth:          nil,
			expectAuthCreation:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			privateKeyData := make([]byte, 32)
			_, err := rand.Read(privateKeyData)
			if err != nil {
				t.Errorf("Failed to generate random data in test: %v", err)
			}

			mockDingoClient := din.NewMockIDingoClient(mockCtrl)
			mockSiweSignerClient := siwe.NewMockISIWESignerClient(mockCtrl)
			defaultSigner := &siwe.SigningConfig{
				PrivateKey: privateKeyData,
			}

			// Setup mock expectations
			if tt.expectAuthCreation {
				mockSiweSignerClient.EXPECT().
					CreateNewSIWEAuth(tt.authConfig.Url, gomock.Eq(16)).
					Return(tt.expectedAuth).
					Times(1)
			}

			// Create DinMiddleware instance
			dinMiddleware := &DinMiddleware{
				DingoClient:       mockDingoClient,
				SiweSignerClient:  mockSiweSignerClient,
				RegistryPriority:  10,
				logger:            logger.NewLoggerClient(zaptest.NewLogger(t), utils.Environment("test")),
				testMode:          true,
				DefaultSiweSigner: defaultSigner,
			}

			// Mock GetNetworkServiceMethods
			mockDingoClient.EXPECT().
				GetNetworkServiceMethods(tt.networkServiceAddress).
				Return(tt.expectedMethods, tt.getMethodsErr).
				Times(1)

			// Call the function being tested
			createdProvider, err := dinMiddleware.createNewProvider(tt.provider, tt.authConfig, tt.networkServiceAddress)

			// Assert results
			if tt.expectedError != nil {
				assert.Equal(t, err.Error(), tt.expectedError.Error())
				assert.Nil(t, createdProvider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, createdProvider)

				// Verify that the provider was updated correctly
				assert.DeepEqual(t, expectedMethodsMap(tt.expectedMethods), createdProvider.Methods)
				assert.Equal(t, dinMiddleware.RegistryPriority, createdProvider.Priority)
				assert.Equal(t, tt.expectedAuth, createdProvider.Auth)
			}
		})
	}
}

func TestUpdateNetworkData(t *testing.T) {
	tests := []struct {
		name            string
		initialNetwork  *network
		updatedNetwork  *network
		expectedNetwork *network
	}{
		{
			name: "Successful update of network data",
			initialNetwork: &network{
				Name:                    "test-network",
				HCMethod:                "initial-method",
				HCInterval:              10,
				BlockLagLimit:           5,
				MaxRequestPayloadSizeKB: 1024,
				RequestAttemptCount:     3,
				Providers: map[string]*provider{
					"existing-host": {
						host: "existing-host",
					},
				},
			},
			updatedNetwork: &network{
				Name:                    "test-network",
				HCMethod:                "new-method",
				HCInterval:              20,
				BlockLagLimit:           10,
				MaxRequestPayloadSizeKB: 2048,
				RequestAttemptCount:     5,
				Providers: map[string]*provider{
					"new-host": {
						host: "new-host",
					},
				},
			},
			expectedNetwork: &network{
				Name:                    "test-network",
				HCMethod:                "new-method",
				HCInterval:              20,
				BlockLagLimit:           10,
				MaxRequestPayloadSizeKB: 2048,
				RequestAttemptCount:     5,
				Providers: map[string]*provider{
					"existing-host": {
						host: "existing-host",
					},
					"new-host": {
						host: "new-host",
					},
				},
			},
		},
		{
			name: "Update with empty providers",
			initialNetwork: &network{
				Name:                    "test-network",
				HCMethod:                "initial-method",
				HCInterval:              10,
				BlockLagLimit:           5,
				MaxRequestPayloadSizeKB: 1024,
				RequestAttemptCount:     3,
				Providers: map[string]*provider{
					"existing-host": {
						host: "existing-host",
					},
				},
			},
			updatedNetwork: &network{
				Name:                    "test-network",
				HCMethod:                "new-method",
				HCInterval:              20,
				BlockLagLimit:           10,
				MaxRequestPayloadSizeKB: 2048,
				RequestAttemptCount:     5,
				Providers:               map[string]*provider{},
			},
			expectedNetwork: &network{
				Name:                    "test-network",
				HCMethod:                "new-method",
				HCInterval:              20,
				BlockLagLimit:           10,
				MaxRequestPayloadSizeKB: 2048,
				RequestAttemptCount:     5,
				Providers: map[string]*provider{
					"existing-host": {
						host: "existing-host",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize DinMiddleware and lock
			dinMiddleware := &DinMiddleware{
				Networks: map[string]*network{
					tt.initialNetwork.Name: tt.initialNetwork,
				},
				mu:       sync.RWMutex{},
				testMode: true,
			}

			// Lock for writing
			dinMiddleware.mu.Lock()
			// Call the function being tested
			dinMiddleware.updateNetworkData(tt.updatedNetwork)
			// Unlock
			dinMiddleware.mu.Unlock()

			// Assert that the network data was updated correctly
			updatedNetwork := dinMiddleware.Networks[tt.initialNetwork.Name]
			assert.Equal(t, tt.expectedNetwork.HCMethod, updatedNetwork.HCMethod)
			assert.Equal(t, tt.expectedNetwork.HCInterval, updatedNetwork.HCInterval)
			assert.Equal(t, tt.expectedNetwork.BlockLagLimit, updatedNetwork.BlockLagLimit)
			assert.Equal(t, tt.expectedNetwork.MaxRequestPayloadSizeKB, updatedNetwork.MaxRequestPayloadSizeKB)
			assert.Equal(t, tt.expectedNetwork.RequestAttemptCount, updatedNetwork.RequestAttemptCount)
			assert.Equal(t, len(tt.expectedNetwork.Providers), len(updatedNetwork.Providers))

			for host, provider := range tt.expectedNetwork.Providers {
				assert.Equal(t, provider.host, updatedNetwork.Providers[host].host)
			}
		})
	}
}

func TestCreateProviderSIWEAuth(t *testing.T) {
	tests := []struct {
		name               string
		authConfig         *dinreg.NetworkServiceAuthConfig
		defaultSignerSet   bool
		expectedAuth       *siwe.SIWEClientAuth
		expectedError      error
		expectAuthCreation bool
	}{
		{
			name: "Successful SIWE auth creation with default signer",
			authConfig: &dinreg.NetworkServiceAuthConfig{
				Type: dinreg.SIWE,
				Url:  "http://example.com",
			},
			defaultSignerSet:   true,
			expectAuthCreation: true,
			expectedAuth: &siwe.SIWEClientAuth{
				ProviderURL:  "http://example.com",
				SessionCount: 16,
			},
			expectedError: nil,
		},
		{
			name: "Auth type None should return nil",
			authConfig: &dinreg.NetworkServiceAuthConfig{
				Type: dinreg.None,
				Url:  "http://example.com",
			},
			defaultSignerSet:   true,
			expectAuthCreation: false,
			expectedAuth:       nil,
			expectedError:      nil,
		},
		{
			name: "Unknown auth type should return nil",
			authConfig: &dinreg.NetworkServiceAuthConfig{
				Type: "unknown",
				Url:  "http://example.com",
			},
			defaultSignerSet:   true,
			expectAuthCreation: false,
			expectedAuth:       nil,
			expectedError:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockSiweSignerClient := siwe.NewMockISIWESignerClient(mockCtrl)

			// Create random private key data for the default signer
			privateKeyData := make([]byte, 32)
			_, err := rand.Read(privateKeyData)
			if err != nil {
				t.Errorf("Failed to generate random data in test: %v", err)
			}

			// Create DinMiddleware instance
			dinMiddleware := &DinMiddleware{
				SiweSignerClient: mockSiweSignerClient,
				logger:           logger.NewLoggerClient(zaptest.NewLogger(t), utils.Environment("test")),
			}

			// Set default signer if required by test case
			if tt.defaultSignerSet {
				dinMiddleware.DefaultSiweSigner = &siwe.SigningConfig{
					PrivateKey: privateKeyData,
				}
			}

			// Setup mock expectations
			if tt.expectAuthCreation {
				mockSiweSignerClient.EXPECT().
					CreateNewSIWEAuth(tt.authConfig.Url, 16).
					Return(tt.expectedAuth).
					Times(1)
			}

			// Call the function being tested
			auth, err := dinMiddleware.createProviderSIWEAuth(tt.authConfig)

			// Assert results
			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError.Error(), err.Error())
				assert.Nil(t, auth)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedAuth, auth)
			}
		})
	}
}
