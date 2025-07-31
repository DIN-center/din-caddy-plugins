package modules

import (
	"bytes"
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	reflect "reflect"
	"strings"
	"testing"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
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
					Name: "eth",
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
			name: "unsuccesful request, payload too large",
			request: func() *http.Request {
				req := httptest.NewRequest("POST", "http://localhost:8000/eth", strings.NewReader(largePayload))
				req.Header.Set("Content-Type", "application/json")
				return req
			}(),
			provider: "localhost:8000",
			networks: map[string]*network{
				"eth": {
					Name: "eth",
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
				testMode: true,
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
				assert.NotZero(t, dinMiddleware.RegistryBlockCheckIntervalSec)
				assert.NotZero(t, dinMiddleware.RegistryBlockEpoch)
				assert.Equal(t, 0, dinMiddleware.RegistryPriority)

				// // Assert networks and providers are initialized
				for _, network := range dinMiddleware.Networks {
					assert.NotNil(t, network.HttpClient)
					assert.NotNil(t, network.logger)
					for _, provider := range network.Providers {
						assert.NotNil(t, provider.upstream)
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
			},
			httpClient: &din_http.HTTPClient{},
			wantErr:    false,
		},
		{
			name: "Successful initialization with https URL",
			provider: &provider{
				HttpUrl: "https://example3.com",
				Auth:    nil,
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
			},
			httpClient: &din_http.HTTPClient{},
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dinMiddleware := &DinMiddleware{
				logger: logger,
			}
			err := dinMiddleware.initializeProvider(tt.provider, tt.httpClient, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("DinMiddleware.initializeProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDinMiddlewareProvision(t *testing.T) {
	tests := []struct {
		name            string
		networks        map[string]*network
		registryEnabled bool
		initializeErr   error
		expectedError   error
	}{
		{
			name: "Successful provision in test mode",
			networks: map[string]*network{
				"test-network": {},
			},
			expectedError: nil,
		},
		{
			name:            "network not found but registry is enabled",
			registryEnabled: true,
			networks:        map[string]*network{},
			expectedError:   nil,
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
			}
			if tt.registryEnabled {
				dinMiddleware.RegistryEnabled = tt.registryEnabled
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
					chain_id eip155:0x1
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

func TestProcessHCMethodResponseAsync(t *testing.T) {
	// Helper to capture stdout for fmt.Printf checks
	captureOutput := func(f func()) string {
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		// Note: log.SetOutput(w) might be needed if fmt.Printf is redirected through std log, but usually not.

		f()

		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		io.Copy(&buf, r)
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

				if tt.name == "Successful processing" {
					assert.Contains(t, actualLogOutput, "successfully processed block number and added to network history", "Expected 'successfully processed block number' log for successful case; Log: "+actualLogOutput)
				} else if tt.name == "Error from processBlockNumberResponse - malformed respBody" || tt.name == "Error from processBlockNumberResponse - http error status" {
					assert.Contains(t, actualLogOutput, "error processing block number from response using processBlockNumberResponse", "Expected 'error processing block number' log for error cases; Log: "+actualLogOutput)
				}
			}
		})
	}
}
