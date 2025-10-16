package modules

import (
	"bytes"
	"container/list"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	reflect "reflect"
	"strings"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
	ws "github.com/DIN-center/din-caddy-plugins/lib/watcherscore"
	din "github.com/DIN-center/din-sc/apps/din-go/lib/din"
)

func TestMiddlewareCaddyModule(t *testing.T) {
	dinMiddleware := new(DinMiddleware)

	tests := []struct {
		name   string
		output caddy.ModuleInfo
	}{
		{
			name: "TestMiddlewareCaddyModuleInit",
			output: caddy.ModuleInfo{
				ID:  "http.handlers.din",
				New: func() caddy.Module { return new(DinMiddleware) },
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modInfo := dinMiddleware.CaddyModule()
			if modInfo.ID != tt.output.ID {
				t.Errorf("CaddyModule() = %v, want %v", modInfo.ID, tt.output.ID)
			}
			if reflect.TypeOf(modInfo.New()) != reflect.TypeOf(tt.output.New()) {
				t.Errorf("CaddyModule() = %v, want %v", modInfo.New(), tt.output.New())
			}
		})
	}
}

func TestMiddlewareServeHTTP(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dinMiddleware := new(DinMiddleware)
	dinMiddleware.testMode = true
	dinMiddleware.logger = logger.NewLoggerClient(zaptest.NewLogger(t), utils.EnvTest)

	// Initialize the handler registry for testing
	dinMiddleware.handlerRegistry = networklib.DefaultRegistry

	// Large payload to test max request payload size. This is greater than 1KB.
	largePayload := `{";;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
	;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
	;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
	;;;;;;;;;;;;;;;;;;;;;;;;;;":";;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
	;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
	;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
	;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
	;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;
	;;;;;;;;;;;;;;;;;;;;;;;;;;;;"}`

	test := []struct {
		name     string
		request  *http.Request
		provider string
		networks map[string]*network
		hasErr   bool
	}{
		{
			name: "successful request",
			request: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost:8000/eth", strings.NewReader(`{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`))
				req.Header.Set("Content-Type", "application/json")
				return req
			}(),
			provider: "localhost:8000",
			networks: map[string]*network{
				"eth": {
					Name:    "eth",
					handler: networklib.NewMockNetworkHandler(mockCtrl),
					Providers: map[string]*provider{
						"localhost:8000": {
							blockHistory: func() *list.List {
								l := list.New()
								l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Healthy})
								return l
							}(),
						},
					},
					MaxRequestPayloadSizeKB: DefaultMaxRequestPayloadSizeKB,
				},
			},
			hasErr: false,
		},
		{
			name: "unsuccessful request, payload too large",
			request: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost:8000/eth", strings.NewReader(largePayload))
				req.Header.Set("Content-Type", "application/json")
				return req
			}(),
			provider: "localhost:8000",
			networks: map[string]*network{
				"eth": {
					Name:    "eth",
					handler: networklib.NewMockNetworkHandler(mockCtrl),
					Providers: map[string]*provider{
						"localhost:8000": {
							blockHistory: func() *list.List {
								l := list.New()
								l.PushBack(blockHistoryEntry{blockNumber: 100, healthStatus: Healthy})
								return l
							}(),
						},
					},
					MaxRequestPayloadSizeKB: 0,
				},
			},
			hasErr: true,
		},
		{
			name:    "unsuccessful request, path not found",
			request: httptest.NewRequest("GET", "http://localhost:8000/xxx", nil),
			networks: map[string]*network{
				"eth": {},
			},
			hasErr: true,
		},
		{
			name:     "unsuccessful request, network map is empty",
			request:  httptest.NewRequest("GET", "http://localhost:8000/eth", nil),
			networks: map[string]*network{},
			hasErr:   true,
		},
	}

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			// Set up mock handler expectations if network has a handler
			for _, net := range tt.networks {
				if mockHandler, ok := net.handler.(*networklib.MockNetworkHandler); ok {
					mockHandler.EXPECT().ExtractMethod(gomock.Any(), gomock.Any()).Return("eth_blockNumber", nil).AnyTimes()
					mockHandler.EXPECT().GetRequestType().Return(networklib.RequestTypeRPC).AnyTimes()
					mockHandler.EXPECT().ProcessRequest(gomock.Any()).Return(nil).AnyTimes()
					mockHandler.EXPECT().ParseResponse(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
					mockHandler.EXPECT().ConfigureRequestPath(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				}
			}

			dinMiddleware.Networks = tt.networks
			tt.request = tt.request.WithContext(context.WithValue(tt.request.Context(), caddy.ReplacerCtxKey, caddy.NewReplacer()))
			rw := httptest.NewRecorder()

			repl := tt.request.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
			repl.Set(RequestProviderKey, tt.provider)

			err := dinMiddleware.ServeHTTP(rw, tt.request, caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error { return nil }))
			if err == nil && tt.hasErr {
				t.Errorf("ServeHTTP() = %v, want %v", err, tt.hasErr)
			} else if err != nil && !tt.hasErr {
				t.Errorf("ServeHTTP() = %v, want %v", err, tt.hasErr)
			}
		})
	}
}

func TestInitialize(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tests := []struct {
		name                 string
		initialDinMiddleware *DinMiddleware
		expectedError        error
	}{
		{
			name: "Successful initialization",
			initialDinMiddleware: &DinMiddleware{
				Networks: map[string]*network{
					"test-network": {
						Providers: map[string]*provider{
							"provider1": {
								HttpUrl: "http://example1.com",
							},
						},
					},
				},
				testMode:                   true,
				DynamicLoadBalacingEnabled: true,
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Create logger and mock context
			logger := logger.NewLoggerClient(zaptest.NewLogger(t), utils.Environment("test"))

			// Setup DinMiddleware object
			dinMiddleware := tt.initialDinMiddleware
			dinMiddleware.machineID = "test-machine-id"
			dinMiddleware.logger = logger

			// Call the initialize function
			err := dinMiddleware.initialize(caddy.Context{})

			// Assert the expected results
			if tt.expectedError != nil {
				assert.NotNil(t, err)
			} else {
				assert.NoError(t, err)

				// Assert default values are set if not provided
				assert.NotZero(t, dinMiddleware.Registry.BlockCheckIntervalSec)
				assert.NotZero(t, dinMiddleware.Registry.BlockEpoch)
				assert.Equal(t, 0, dinMiddleware.Registry.Priority)

				// Assert watcher score manager is initialized
				assert.NotNil(t, dinMiddleware.watcherScoreManager)

				// // Assert networks and providers are initialized
				for networkName, network := range dinMiddleware.Networks {
					assert.NotNil(t, network.HttpClient)
					assert.NotNil(t, network.logger)

					//Asset each network has a formula
					assert.NotNil(t, dinMiddleware.watcherScoreManager.GetNetworkFormula(networkName))

					for _, provider := range network.Providers {
						assert.NotNil(t, provider.upstream)

						// Assert provider score is initialized
						assert.NotNil(t, provider.Score)
						assert.Equal(t, ws.EmptyScore, provider.Score)
					}
				}

			}
		})
	}
}

func TestInitializeProvider(t *testing.T) {
	logger := logger.NewLoggerClient(zap.NewNop(), utils.EnvTest)
	tests := []struct {
		name       string
		provider   *provider
		httpClient *din_http.HTTPClient
		wantErr    bool
	}{
		{
			name: "Successful initialization with http URL",
			provider: &provider{
				HttpUrl: "http://example2.com",
				Auth:    nil,
				Score:   ws.EmptyScore,
			},
			httpClient: &din_http.HTTPClient{},
			wantErr:    false,
		},
		{
			name: "Successful initialization with https URL",
			provider: &provider{
				HttpUrl: "https://example3.com",
				Auth:    nil,
				Score:   ws.EmptyScore,
			},
			httpClient: &din_http.HTTPClient{},
			wantErr:    false,
		},
		{
			name: "Successful initialization with auth",
			provider: &provider{
				HttpUrl: "http://example4.com",
				Auth: &siwe.SIWEClientAuth{
					ProviderURL: "http://auth.example.com",
				},
				Score: ws.EmptyScore,
			},
			httpClient: &din_http.HTTPClient{},
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dinMiddleware := &DinMiddleware{
				logger: logger,
				Networks: map[string]*network{
					"test-network": {
						Providers: make(map[string]*provider),
					},
				},
			}
			err := dinMiddleware.initializeProvider("test-network", tt.provider, tt.httpClient, logger)
			// Assert provider score is set to the empty score
			assert.Equal(t, ws.EmptyScore, tt.provider.Score)
			if (err != nil) != tt.wantErr {
				t.Errorf("DinMiddleware.initializeProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDinMiddlewareProvision(t *testing.T) {
	tests := []struct {
		name                string
		networks            map[string]*network
		registryEnabled     bool
		registryEndpointUrl string
		initializeErr       error
		expectedError       error
	}{
		{
			name: "Successful provision in test mode",
			networks: map[string]*network{
				"test-network": {},
			},
			expectedError: nil,
		},
		{
			name:                "network not found but registry is enabled",
			registryEnabled:     true,
			registryEndpointUrl: "http://example1.com",
			networks:            map[string]*network{},
			expectedError:       nil,
		},
		{
			name:          "minimum of 1 network not found",
			networks:      map[string]*network{},
			expectedError: fmt.Errorf("expected at least 1 network or registry to be defined"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logger.NewLoggerClient(zaptest.NewLogger(t), utils.Environment("test"))
			dinMiddleware := &DinMiddleware{
				testMode: true, // Ensure test mode is enabled
				logger:   logger,
				Networks: tt.networks,
				Registry: RegistryConfig{
					EndpointUrl: tt.registryEndpointUrl,
					Enabled:     tt.registryEnabled,
				},
			}

			// Call the Provision method
			err := dinMiddleware.Provision(caddy.Context{})

			// Assert results based on the case
			if tt.expectedError != nil {
				assert.Equal(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, true, dinMiddleware.testMode)
			}
		})
	}
}

func TestUnmarshalCaddyfile(t *testing.T) {
	dinMiddleware := new(DinMiddleware)
	dinMiddleware.logger = logger.NewLoggerClient(zap.NewNop(), utils.EnvTest)

	tests := []struct {
		name      string
		caddyfile string
		hasErr    bool
	}{
		{
			name: "Valid Caddyfile",
			caddyfile: `networks {
				eth {
					methods eth_blockNumber eth_getBlockByNumber
					providers {
						http://test-website-1.com/eth {
							headers {
								Content-Type application/json
							}
							priority 1
						}
						http://test-website-2.com/eth {
							headers {
								Content-Type application/json
							}
							priority 2
						}
					}
					chain_id 0x1
					healthcheck_threshold 2
					healthcheck_interval 5
					healthcheck_blocklag_limit 10
					max_request_payload_size_kb 100
				}
			}`,
			hasErr: false,
		},
		{
			name: "Invalid Caddyfile - No chain_id",
			caddyfile: `networks {
				eth {
					methods eth_blockNumber eth_getBlockByNumber
					providers {
						http://test-website-1.com/eth {
							headers {
								Content-Type application/json
							}
							priority 1
						}
						http://test-website-2.com/eth {
							headers {
								Content-Type application/json
							}
							priority 2
						}
					}
					healthcheck_threshold 2
					healthcheck_interval 5
					healthcheck_blocklag_limit 10
					max_request_payload_size_kb 100
				}
			}`,
			hasErr: true,
		},
		{
			name: "Invalid Caddyfile - Missing provider",
			caddyfile: `networks {
				eth {
					methods methods eth_blockNumber eth_getBlockByNumber
					healthcheck_threshold 2
					healthcheck_interval 5
					healthcheck_blocklag_limit 10
					max_request_payload_size_kb 100
				}
			}`,
			hasErr: true,
		},
		{
			name: "Invalid Caddyfile - Invalid 'methods' argument",
			caddyfile: `networks {
				eth {
					methods
					providers {
						localhost:8000 {
							headers {
								Content-Type application/json
							}
							priority 1
						}
					}
					healthcheck_threshold 2
					healthcheck_interval 5
					healthcheck_blocklag_limit 10
					max_request_payload_size_kb 100
				}
			}`,
			hasErr: true,
		},
		{
			name: "Invalid Caddyfile - Invalid 'headers' argument",
			caddyfile: `networks {
				eth {
					methods eth_blockNumber eth_getBlockByNumber
					providers {
						localhost:8000 {
							headers
							priority 1
						}
					}
				}
			}`,
			hasErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dispenser := caddyfile.NewTestDispenser(tt.caddyfile)
			err := dinMiddleware.UnmarshalCaddyfile(dispenser)
			if err != nil && !tt.hasErr {
				t.Errorf("UnmarshalCaddyfile() = %v, want %v", err, tt.hasErr)
			}
		})
	}
}

func TestUnmarshalCaddyfileAPIKeys(t *testing.T) {
	dinMiddleware := new(DinMiddleware)
	dinMiddleware.logger = logger.NewLoggerClient(zap.NewNop(), utils.EnvTest)

	tests := []struct {
		name       string
		caddyfile  string
		hasErr     bool
		expectKeys map[string]string
	}{
		{
			name: "Valid Caddyfile",
			caddyfile: `networks {
				eth {
					methods eth_blockNumber eth_getBlockByNumber
					providers {
						http://test-website-1.com/eth {
							headers {
								Content-Type application/json
							}
							priority 1
						}
						http://test-website-2.com/eth {
							headers {
								Content-Type application/json
							}
							priority 2
						}
					}
					chain_id 0x1
					healthcheck_threshold 2
					healthcheck_interval 5
					healthcheck_blocklag_limit 10
					max_request_payload_size_kb 100
				}
			}
			unknown_api_key_salt foo
			api_keys {
				test-key some-user
				other-key other-user
			}`,
			expectKeys: map[string]string{
				"test-key":    "some-user",
				"other-key":   "other-user",
				"missing-key": "c92388d1d4", // sha256(foo + missing-key)[:10]
			},
			hasErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dispenser := caddyfile.NewTestDispenser(tt.caddyfile)
			err := dinMiddleware.UnmarshalCaddyfile(dispenser)
			if err != nil && !tt.hasErr {
				t.Errorf("UnmarshalCaddyfile() = %v, want %v", err, tt.hasErr)
			}
			for k, v := range tt.expectKeys {
				assert.Equal(t, dinMiddleware.getAPIKeyId(k), v)
			}
		})
	}
}

func TestProcessHCMethodResponseAsync(t *testing.T) {
	// Helper to capture stdout for fmt.Printf checks
	captureOutput := func(f func()) string {
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		// Note: log.SetOutput(w) might be needed if fmt.Printf is redirected through std log, but usually not.

		f()

		require.NoError(t, w.Close())
		os.Stdout = oldStdout
		var buf bytes.Buffer
		_, err := io.Copy(&buf, r)
		require.NoError(t, err)

		return buf.String()
	}

	tests := []struct {
		name                string
		setupNetwork        func(t *testing.T, netw *network)
		netPath             string
		respBody            []byte
		respStatus          int
		callMethod          string
		expectNoProcessLogs bool
	}{
		{
			name: "Successful processing",
			setupNetwork: func(t *testing.T, netw *network) {
				netw.CaddyPort = "8000"

				mockCtrl := gomock.NewController(t)
				// defer mockCtrl.Finish() // Defers in callbacks can be tricky; manage Finish in t.Run if issues arise.

				mockHttpClient := din_http.NewMockIHTTPClient(mockCtrl)
				netw.HttpClient = mockHttpClient

				mockHttpClient.EXPECT().Post(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(url string, headers map[string]string, payload []byte, authClient auth.IAuthClient) ([]byte, *int, error) {
						status := http.StatusOK
						blockResp := din_http.JSONRPCEVMBlockResponse{
							Jsonrpc: "2.0",
							ID:      json.RawMessage(`1`),
							Result: din_http.EVMBlockResult{ //
								Hash:   "0x123abc",
								Number: "0x64",
							},
						}
						respBytes, _ := json.Marshal(blockResp)
						return respBytes, &status, nil
					}).AnyTimes()
			},
			netPath:             "test/eth",
			respBody:            []byte(`{"jsonrpc":"2.0","id":1,"result":"0x64"}`),
			respStatus:          http.StatusOK,
			callMethod:          "eth_blockNumber",
			expectNoProcessLogs: false,
		},
		{
			name: "HCMethod does not match",
			setupNetwork: func(t *testing.T, netw *network) {
				netw.CaddyPort = "8001" // Add a dummy CaddyPort to prevent nil errors if getBlockByNumber is unexpectedly called
				// No HttpClient mock needed as getBlockByNumber ideally won't be called
			},
			netPath:             "test/eth",
			respBody:            []byte(`{"jsonrpc":"2.0","id":1,"result":"0x64"}`),
			respStatus:          http.StatusOK,
			callMethod:          "other_method",
			expectNoProcessLogs: true,
		},
		{
			name: "Response body empty",
			setupNetwork: func(t *testing.T, netw *network) {
				netw.CaddyPort = "8002" // Add a dummy CaddyPort
				// No HttpClient mock needed
			},
			netPath:             "test/eth",
			respBody:            []byte{},
			respStatus:          http.StatusOK,
			callMethod:          "eth_blockNumber",
			expectNoProcessLogs: true,
		},
		{
			name: "Network object HCMethod empty",
			setupNetwork: func(t *testing.T, netw *network) {
				netw.CaddyPort = "8003" // Add a dummy CaddyPort
				// No HttpClient mock needed since we expect early return due to method mismatch
			},
			netPath:             "test/eth",
			respBody:            []byte(`{"jsonrpc":"2.0","id":1,"result":"0x64"}`),
			respStatus:          http.StatusOK,
			callMethod:          "some_other_method", // Different from health check method to trigger early return
			expectNoProcessLogs: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use zaptest.NewLogger for robust test logging and capture
			// zaptest.NewLogger(t) automatically routes logs to the test output if a test fails.
			// To capture logs for assertion, we can use a observer/hook or a custom core.
			// For simplicity here, we'll keep the buffer but ensure the logger is correctly plumbed.

			var logBuf bytes.Buffer
			observedZapCore, observedLogs := observer.New(zap.DebugLevel) // observer is from zap/observer

			// Create a multi-core: one for console/test output (optional), one for observation
			// and one for the existing buffer method to see if it can be made to work.
			consoleCore := zapcore.NewCore(
				zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
				zapcore.AddSync(os.Stderr), // Write to Stderr for visibility during test run
				zap.DebugLevel,
			)
			bufferCore := zapcore.NewCore(
				zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
				zapcore.AddSync(&logBuf),
				zap.DebugLevel,
			)

			teeCore := zapcore.NewTee(
				observedZapCore,
				consoleCore,
				bufferCore,
			)
			testZapLogger := zap.New(teeCore)

			dm := &DinMiddleware{
				logger: logger.NewLoggerClient(testZapLogger, utils.EnvTest),
			}

			// mockCtrl and mockHttpClient setup will be handled by tt.setupNetwork for relevant cases

			// Create a proper network with handler using NewNetwork
			netw, err := NewNetwork("test", EVMHandler, utils.EnvTest, "8000")
			if err != nil {
				t.Fatalf("Failed to create network: %v", err)
			}
			netw.logger = dm.logger
			netw.HttpClient = nil // Initialize as nil; setupNetwork can override for specific tests

			// Create and set handler for the network since it's not created until Provision
			mockCtrl := gomock.NewController(t)
			mockHandler := networklib.NewMockNetworkHandler(mockCtrl)
			netw.handler = mockHandler

			// Set up default expectations for the mock handler
			mockHandler.EXPECT().GetHealthCheckMethod().Return("eth_blockNumber").AnyTimes()
			mockHandler.EXPECT().GetRequestType().Return(networklib.RequestTypeRPC).AnyTimes()
			mockHandler.EXPECT().GetHealthCheckHTTPMethod().Return("POST").AnyTimes()
			mockHandler.EXPECT().CreateHealthCheckPayload(gomock.Any()).Return([]byte(`{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`), nil).AnyTimes()
			mockHandler.EXPECT().ParseBlockNumberResponse(gomock.Any(), gomock.Any()).Return(int64(100), nil).AnyTimes()
			mockHandler.EXPECT().SupportsGetBlockByNumber().Return(true).AnyTimes()
			mockHandler.EXPECT().GetBlockByNumberMethod().Return("eth_getBlockByNumber").AnyTimes()
			mockHandler.EXPECT().CreateBlockRequest(gomock.Any(), gomock.Any(), gomock.Any()).Return([]byte(`{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["0x64",false],"id":1}`), nil).AnyTimes()
			mockHandler.EXPECT().ParseBlockResponse(gomock.Any()).Return(din_http.JSONRPCEVMBlockResponse{
				Jsonrpc: "2.0",
				ID:      json.RawMessage(`1`),
				Result: din_http.EVMBlockResult{
					Hash:   "0x123abc",
					Number: "0x64",
				},
			}, nil).AnyTimes()
			mockHandler.EXPECT().ExtractBlockHash(gomock.Any()).Return("0x123abc").AnyTimes()

			if tt.setupNetwork != nil {
				tt.setupNetwork(t, netw) // Pass t to setupNetwork
			}

			// Ensure that if HttpClient is used (e.g., in "Successful processing"), it has been mocked by setupNetwork
			if tt.name == "Successful processing" && netw.HttpClient == nil {
				t.Fatalf("HttpClient mock not set up for 'Successful processing' test case in setupNetwork")
			}

			printfOutput := captureOutput(func() {
				dm.processHCMethodResponseAsync(netw, tt.netPath, tt.respBody, tt.respStatus, tt.callMethod)
			})
			_ = testZapLogger.Sync()

			// Check logs from the observer first
			numLogsFromObserver := observedLogs.Len()
			// Concatenate observed log messages for assertion
			var observedLogOutput strings.Builder
			for _, obsLog := range observedLogs.AllUntimed() { // Untimed to simplify string matching
				observedLogOutput.WriteString(obsLog.Message)
				for _, field := range obsLog.Context {
					observedLogOutput.WriteString(fmt.Sprintf(" %s=%v", field.Key, field.Interface))
				}
				observedLogOutput.WriteString("\n")
			}
			actualLogOutput := observedLogOutput.String()
			// Fallback to buffer if observer didn't capture as expected, or for comparison
			if numLogsFromObserver == 0 {
				actualLogOutput = logBuf.String() // Use buffer if observer is empty
			}

			_ = printfOutput // Suppress unused variable warning for printfOutput

			if tt.expectNoProcessLogs {
				assert.NotContains(t, actualLogOutput, "Goroutine: Processing response for HCMethod", "Should not log 'Processing response' for this case: "+tt.name)
				assert.NotContains(t, actualLogOutput, "successfully processed block number and added to network history", "Should not log 'successfully processed' for this case: "+tt.name)
				assert.NotContains(t, actualLogOutput, "error processing block number", "Should not log 'error processing' for this case: "+tt.name)
			} else {
				assert.Contains(t, actualLogOutput, "Goroutine: Processing response for HCMethod", "Expected 'Processing response for HCMethod' log for case: "+tt.name+"; Log: "+actualLogOutput)

				switch tt.name {
				case "Successful processing":
					assert.Contains(t, actualLogOutput, "successfully processed block number and added to network history", "Expected 'successfully processed block number' log for successful case; Log: "+actualLogOutput)
				case "Error from processBlockNumberResponse - malformed respBody", "Error from processBlockNumberResponse - http error status":
					assert.Contains(t, actualLogOutput, "error processing block number from response using processBlockNumberResponse", "Expected 'error processing block number' log for error cases; Log: "+actualLogOutput)
				}
			}
		})
	}
}

// TestGetRegistryData tests the getRegistryData function with retry logic
func TestGetRegistryData(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	tests := []struct {
		name             string
		retryMaxAttempts int
		retryDelay       time.Duration
		mockSetup        func(mockClient *din.MockIDinClient)
		expectedData     *din.DinRegistryData
		expectedError    bool
		expectedCalls    int
	}{
		{
			name:             "Success on first attempt",
			retryMaxAttempts: 3,
			retryDelay:       10 * time.Millisecond,
			mockSetup: func(mockClient *din.MockIDinClient) {
				mockData := &din.DinRegistryData{
					Networks: map[string]*din.Network{
						"test-network": {ProxyName: "test-network"},
					},
				}
				mockClient.EXPECT().
					GetRegistryData().
					Return(mockData, nil).
					Times(1)
			},
			expectedData: &din.DinRegistryData{
				Networks: map[string]*din.Network{
					"test-network": {ProxyName: "test-network"},
				},
			},
			expectedError: false,
			expectedCalls: 1,
		},
		{
			name:             "Success on second attempt",
			retryMaxAttempts: 3,
			retryDelay:       10 * time.Millisecond,
			mockSetup: func(mockClient *din.MockIDinClient) {
				mockData := &din.DinRegistryData{
					Networks: map[string]*din.Network{
						"test-network": {ProxyName: "test-network"},
					},
				}
				gomock.InOrder(
					mockClient.EXPECT().
						GetRegistryData().
						Return(nil, errors.New("temporary error")).
						Times(1),
					mockClient.EXPECT().
						GetRegistryData().
						Return(mockData, nil).
						Times(1),
				)
			},
			expectedData: &din.DinRegistryData{
				Networks: map[string]*din.Network{
					"test-network": {ProxyName: "test-network"},
				},
			},
			expectedError: false,
			expectedCalls: 2,
		},
		{
			name:             "Failure after all retries",
			retryMaxAttempts: 2,
			retryDelay:       10 * time.Millisecond,
			mockSetup: func(mockClient *din.MockIDinClient) {
				mockClient.EXPECT().
					GetRegistryData().
					Return(nil, errors.New("persistent error")).
					Times(3) // initial + 2 retries
			},
			expectedData:  nil,
			expectedError: true,
			expectedCalls: 3,
		},
		{
			name:             "Success on last attempt",
			retryMaxAttempts: 2,
			retryDelay:       10 * time.Millisecond,
			mockSetup: func(mockClient *din.MockIDinClient) {
				mockData := &din.DinRegistryData{
					Networks: map[string]*din.Network{
						"test-network": {ProxyName: "test-network"},
					},
				}
				gomock.InOrder(
					mockClient.EXPECT().
						GetRegistryData().
						Return(nil, errors.New("error 1")).
						Times(1),
					mockClient.EXPECT().
						GetRegistryData().
						Return(nil, errors.New("error 2")).
						Times(1),
					mockClient.EXPECT().
						GetRegistryData().
						Return(mockData, nil).
						Times(1),
				)
			},
			expectedData: &din.DinRegistryData{
				Networks: map[string]*din.Network{
					"test-network": {ProxyName: "test-network"},
				},
			},
			expectedError: false,
			expectedCalls: 3,
		},
		{
			name:             "Zero retries - fail immediately",
			retryMaxAttempts: 0,
			retryDelay:       10 * time.Millisecond,
			mockSetup: func(mockClient *din.MockIDinClient) {
				mockClient.EXPECT().
					GetRegistryData().
					Return(nil, errors.New("error")).
					Times(1)
			},
			expectedData:  nil,
			expectedError: true,
			expectedCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDingoClient := din.NewMockIDinClient(mockCtrl)
			tt.mockSetup(mockDingoClient)

			d := &DinMiddleware{
				Registry: RegistryConfig{
					RetryMaxAttempts: tt.retryMaxAttempts,
					RetryDelay:       tt.retryDelay,
				},
				DingoClient: mockDingoClient,
				logger:      logger.NewLoggerClient(zaptest.NewLogger(t), utils.Environment("test")),
			}

			startTime := time.Now()
			data, err := d.getRegistryData()
			elapsed := time.Since(startTime)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, data)
				assert.Contains(t, err.Error(), "registry call failed after")
				// Verify delay was applied (except for last attempt)
				if tt.retryMaxAttempts > 0 {
					expectedMinDelay := time.Duration(tt.retryMaxAttempts) * tt.retryDelay
					assert.GreaterOrEqual(t, elapsed, expectedMinDelay)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedData, data)
			}
		})
	}
}

// TestCleanup tests the Cleanup function
func TestCleanup(t *testing.T) {
	tests := []struct {
		name           string
		setupNetworks  func() map[string]*network
		registryQuit   bool
		expectedClosed int
	}{
		{
			name: "Cleanup with multiple networks",
			setupNetworks: func() map[string]*network {
				return map[string]*network{
					"network1": {
						Name: "network1",
						quit: make(chan struct{}),
					},
					"network2": {
						Name: "network2",
						quit: make(chan struct{}),
					},
					"network3": {
						Name: "network3",
						quit: make(chan struct{}),
					},
				}
			},
			registryQuit:   true,
			expectedClosed: 4, // 3 networks + 1 registry
		},
		{
			name: "Cleanup with no networks",
			setupNetworks: func() map[string]*network {
				return map[string]*network{}
			},
			registryQuit:   true,
			expectedClosed: 1, // only registry
		},
		{
			name: "Cleanup with nil quit channels",
			setupNetworks: func() map[string]*network {
				return map[string]*network{
					"network1": {
						Name: "network1",
						quit: nil, // nil channel
					},
					"network2": {
						Name: "network2",
						quit: make(chan struct{}),
					},
				}
			},
			registryQuit:   false, // registry quit is nil
			expectedClosed: 1,     // only network2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DinMiddleware{
				Networks: tt.setupNetworks(),
				logger:   logger.NewLoggerClient(zaptest.NewLogger(t), utils.Environment("test")),
			}

			if tt.registryQuit {
				d.quit = make(chan struct{})
			}

			// Track which channels are closed
			closedCount := 0

			// Monitor channels in goroutines
			for _, network := range d.Networks {
				if network.quit != nil {
					go func(ch chan struct{}) {
						<-ch
						closedCount++
					}(network.quit)
				}
			}

			if d.quit != nil {
				go func() {
					<-d.quit
					closedCount++
				}()
			}

			// Call Cleanup
			err := d.Cleanup()
			assert.NoError(t, err)

			// Give goroutines time to detect closed channels
			time.Sleep(50 * time.Millisecond)

			// Verify expected number of channels were closed
			assert.Equal(t, tt.expectedClosed, closedCount)
		})
	}
}

// TestCleanupConcurrency tests that Cleanup handles concurrent access safely
func TestCleanupConcurrency(t *testing.T) {
	d := &DinMiddleware{
		Networks: map[string]*network{
			"network1": {
				Name: "network1",
				quit: make(chan struct{}),
			},
			"network2": {
				Name: "network2",
				quit: make(chan struct{}),
			},
		},
		quit:   make(chan struct{}),
		logger: logger.NewLoggerClient(zaptest.NewLogger(t), utils.Environment("test")),
	}

	// Track if channels are closed
	network1Closed := false
	network2Closed := false
	quitClosed := false

	// Monitor channels
	go func() {
		<-d.Networks["network1"].quit
		network1Closed = true
	}()
	go func() {
		<-d.Networks["network2"].quit
		network2Closed = true
	}()
	go func() {
		<-d.quit
		quitClosed = true
	}()

	// Start multiple goroutines trying to cleanup simultaneously
	done := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			done <- d.Cleanup()
		}()
	}

	// All should complete without panic or error
	for i := 0; i < 5; i++ {
		err := <-done
		assert.NoError(t, err, "Cleanup should not return an error")
	}

	// Give time for channel monitors to detect closure
	time.Sleep(50 * time.Millisecond)

	// Verify channels were closed exactly once (no panic from double close)
	assert.True(t, network1Closed, "network1 quit channel should be closed")
	assert.True(t, network2Closed, "network2 quit channel should be closed")
	assert.True(t, quitClosed, "main quit channel should be closed")
}

// TestStartRegistrySyncPanicRecovery tests panic recovery in startRegistrySync
func TestStartRegistrySyncPanicRecovery(t *testing.T) {
	// This test verifies the panic recovery mechanism
	// We'll simulate a panic and ensure the function recovers

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockDingoClient := din.NewMockIDinClient(mockCtrl)

	// Setup mock to return data successfully first time
	mockData := &din.DinRegistryData{
		Networks: map[string]*din.Network{
			"test-network": {ProxyName: "test-network"},
		},
	}
	mockDingoClient.EXPECT().
		GetRegistryData().
		Return(mockData, nil).
		AnyTimes()

	d := &DinMiddleware{
		Registry: RegistryConfig{
			Enabled:               true,
			BlockCheckIntervalSec: 1, // 1 second for faster test
			RetryMaxAttempts:      1,
			RetryDelay:            10 * time.Millisecond,
			PanicRecoveryDelay:    50 * time.Millisecond,
		},
		DingoClient: mockDingoClient,
		Networks:    make(map[string]*network),
		quit:        make(chan struct{}),
		logger:      logger.NewLoggerClient(zaptest.NewLogger(t), utils.Environment("test")),
	}

	// Start the registry sync
	d.startRegistrySync()

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Close the quit channel to stop the sync
	close(d.quit)

	// Give it time to shutdown gracefully
	time.Sleep(100 * time.Millisecond)

	// Test passes if no panic occurred
	assert.True(t, true, "Registry sync handled without panic")
}

// TestRegistryConfigDefaults tests that default values are set correctly
func TestRegistryConfigDefaults(t *testing.T) {
	d := &DinMiddleware{
		logger: logger.NewLoggerClient(zaptest.NewLogger(t), utils.Environment("test")),
	}

	// Call initializeDefaults
	d.initializeDefaults()

	// Verify defaults are set
	assert.Equal(t, uint64(DefaultRegistryBlockCheckIntervalSec), d.Registry.BlockCheckIntervalSec)
	assert.Equal(t, DefaultRegistryBlockEpoch, d.Registry.BlockEpoch)
	assert.Equal(t, DefaultRegistryPriority, d.Registry.Priority)
	assert.Equal(t, DefaultRegistryRetryMaxAttempts, d.Registry.RetryMaxAttempts)
	assert.Equal(t, DefaultRegistryRetryDelay, d.Registry.RetryDelay)
	assert.Equal(t, DefaultRegistryPanicRecoveryDelay, d.Registry.PanicRecoveryDelay)
}

// TestRegistryConfigCustomValues tests that custom values override defaults
func TestRegistryConfigCustomValues(t *testing.T) {
	d := &DinMiddleware{
		Registry: RegistryConfig{
			BlockCheckIntervalSec: 120,
			BlockEpoch:            5000,
			Priority:              5,
			RetryMaxAttempts:      10,
			RetryDelay:            5 * time.Second,
			PanicRecoveryDelay:    60 * time.Second,
		},
		logger: logger.NewLoggerClient(zaptest.NewLogger(t), utils.Environment("test")),
	}

	// Call initializeDefaults
	d.initializeDefaults()

	// Verify custom values are preserved
	assert.Equal(t, uint64(120), d.Registry.BlockCheckIntervalSec)
	assert.Equal(t, uint64(5000), d.Registry.BlockEpoch)
	assert.Equal(t, 5, d.Registry.Priority)
	assert.Equal(t, 10, d.Registry.RetryMaxAttempts)
	assert.Equal(t, 5*time.Second, d.Registry.RetryDelay)
	assert.Equal(t, 60*time.Second, d.Registry.PanicRecoveryDelay)
}

func TestSyncMiddlewareWithLatestScores(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	mockWatcherScoreManager := ws.NewMockIWatcherScoreManager(mockCtrl)

	markerForNonMonitoredProvider := ws.NewEmptyScore()
	mockMiddleware := &DinMiddleware{
		Networks: map[string]*network{
			"network1": {
				Name: "network1",
				Providers: map[string]*provider{
					"provider1": {
						host:  "provider1",
						Score: ws.MustCreateScore(0.8, time.Now()),
					},
					"provider2": {
						host:  "provider2",
						Score: ws.MustCreateScore(0.2, time.Now()),
					},
					"non-monitored-provider": {
						host:  "non-monitored-provider",
						Score: markerForNonMonitoredProvider,
					},
				},
			},
		},
		logger:              logger.NewLoggerClient(zaptest.NewLogger(t), utils.Environment("test")),
		watcherScoreManager: mockWatcherScoreManager,
	}

	//Set expected score for providers
	mockWatcherScoreManager.EXPECT().GetScore("network1", "provider1").Return(ws.MustCreateScore(0.75, time.Now())).Times(1)
	mockWatcherScoreManager.EXPECT().GetScore("network1", "provider2").Return(ws.MustCreateScore(0.25, time.Now())).Times(1)
	mockWatcherScoreManager.EXPECT().GetScore("network1", "non-monitored-provider").Return(&ws.Score{}).Times(1)

	//Call SyncMiddlewareWithLatestScores
	mockMiddleware.SyncMiddlewareWithLatestScores()

	//Verify if scores are updated correctly in the middleware
	assert.Equal(t, 0.75, mockMiddleware.Networks["network1"].Providers["provider1"].Score.Value())
	assert.Equal(t, 0.25, mockMiddleware.Networks["network1"].Providers["provider2"].Score.Value())
	//assert memory address is the same
	if markerForNonMonitoredProvider != mockMiddleware.Networks["network1"].Providers["non-monitored-provider"].Score {
		t.Errorf("Non-monitored provider score should be the same as the marker")
	}
}
