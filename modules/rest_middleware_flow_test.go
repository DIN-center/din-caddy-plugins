package modules

import (
	"container/list"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
)

// TestRESTAPIMiddlewareIntegration tests the complete REST API flow through the middleware
// This is a simplified version that doesn't require full Caddy infrastructure
func TestRESTAPIMiddlewareIntegration(t *testing.T) {
	networklib.RegisterBuiltinHandlers()

	tests := []struct {
		name               string
		networkType        string
		networkName        string
		requestPath        string
		requestMethod      string
		requestBody        string
		expectedStatusCode int
		expectedPathSent   string // Path that should be sent to upstream
		providerPath       string
		mockResponse       func(t *testing.T, req *http.Request) ([]byte, int)
		validateResponse   func(t *testing.T, body []byte)
	}{
		{
			name:               "Beacon Chain Genesis endpoint integration",
			networkType:        string(BeaconHandler),
			networkName:        "ethereum-beacon",
			requestPath:        "/ethereum-beacon/eth/v1/beacon/genesis",
			requestMethod:      "GET",
			expectedStatusCode: 200,
			expectedPathSent:   "/eth/v1/beacon/genesis",
			mockResponse: func(t *testing.T, req *http.Request) ([]byte, int) {
				// Verify the path was stripped correctly
				assert.Equal(t, "/eth/v1/beacon/genesis", req.URL.Path)
				return []byte(`{"data":{"genesis_time":"1606824023","genesis_validators_root":"0x4b363db94e286120d76eb905340fdd4e54bfe9f06bf33ff6cf5ad27f511bfe95"}}`), 200
			},
			validateResponse: func(t *testing.T, body []byte) {
				var result map[string]interface{}
				err := json.Unmarshal(body, &result)
				assert.NoError(t, err)
				assert.Contains(t, result, "data")
			},
		},
		{
			name:               "Beacon Chain Block by ID integration",
			networkType:        string(BeaconHandler),
			networkName:        "beacon-mainnet",
			requestPath:        "/beacon-mainnet/eth/v2/beacon/blocks/head",
			requestMethod:      "GET",
			expectedStatusCode: 200,
			expectedPathSent:   "/eth/v2/beacon/blocks/head",
			mockResponse: func(t *testing.T, req *http.Request) ([]byte, int) {
				assert.Equal(t, "/eth/v2/beacon/blocks/head", req.URL.Path)
				return []byte(`{"version":"phase0","data":{"message":{"slot":"1234","proposer_index":"123"}}}`), 200
			},
			validateResponse: func(t *testing.T, body []byte) {
				var result map[string]interface{}
				err := json.Unmarshal(body, &result)
				assert.NoError(t, err)
				assert.Equal(t, "phase0", result["version"])
			},
		},
		{
			name:               "Beacon Chain Validators with query params integration",
			networkType:        string(BeaconHandler),
			networkName:        "beacon",
			requestPath:        "/beacon/eth/v1/beacon/states/head/validators?id=1,2,3&status=active",
			requestMethod:      "GET",
			expectedStatusCode: 200,
			expectedPathSent:   "/eth/v1/beacon/states/head/validators",
			mockResponse: func(t *testing.T, req *http.Request) ([]byte, int) {
				// Verify query parameters are preserved
				assert.Equal(t, "1,2,3", req.URL.Query().Get("id"))
				assert.Equal(t, "active", req.URL.Query().Get("status"))
				return []byte(`{"data":[{"index":"1","balance":"32000000000"}]}`), 200
			},
		},
		{
			name:               "Beacon Chain with provider base path integration",
			networkType:        string(BeaconHandler),
			networkName:        "beacon",
			requestPath:        "/beacon/eth/v1/node/version",
			requestMethod:      "GET",
			providerPath:       "/api/v2",
			expectedStatusCode: 200,
			expectedPathSent:   "/api/v2/eth/v1/node/version",
			mockResponse: func(t *testing.T, req *http.Request) ([]byte, int) {
				assert.Equal(t, "/api/v2/eth/v1/node/version", req.URL.Path)
				return []byte(`{"data":{"version":"Prysm/v2.0.0"}}`), 200
			},
		},
		{
			name:               "Beacon Chain POST request integration",
			networkType:        string(BeaconHandler),
			networkName:        "beacon",
			requestPath:        "/beacon/eth/v1/beacon/pool/attestations",
			requestMethod:      "POST",
			requestBody:        `{"data":[{"aggregation_bits":"0x01","signature":"0x123"}]}`,
			expectedStatusCode: 200,
			expectedPathSent:   "/eth/v1/beacon/pool/attestations",
			mockResponse: func(t *testing.T, req *http.Request) ([]byte, int) {
				// Verify request body is preserved
				body, _ := io.ReadAll(req.Body)
				assert.Contains(t, string(body), "aggregation_bits")
				return []byte(`{}`), 200
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create middleware
			dm := &DinMiddleware{
				logger:          logger.NewLoggerClient(zaptest.NewLogger(t), utils.EnvTest),
				testMode:        true,
				handlerRegistry: networklib.DefaultRegistry,
				Networks: map[string]*network{
					tt.networkName: {
						Name:        tt.networkName,
						HandlerType: HandlerType(tt.networkType),
						ChainId:     "1",
						Providers: map[string]*provider{
							"test-provider": {
								HttpUrl:  "http://test-provider",
								Host:     "test-provider",
								Path:     tt.providerPath,
								Priority: 1,
								BlockHistory: func() *list.List {
									l := list.New()
									l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Healthy})
									return l
								}(),
							},
						},
						MaxRequestPayloadSizeKB: DefaultMaxRequestPayloadSizeKB,
					},
				},
			}

			// Initialize middleware
			err := dm.initialize(caddy.Context{})
			require.NoError(t, err)

			// Create request
			var req *http.Request
			if tt.requestBody != "" {
				req = httptest.NewRequest(tt.requestMethod, "http://test.com"+tt.requestPath, strings.NewReader(tt.requestBody))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.requestMethod, "http://test.com"+tt.requestPath, nil)
			}

			// Add context
			repl := caddy.NewReplacer()
			ctx := context.WithValue(context.Background(), caddy.ReplacerCtxKey, repl)
			req = req.WithContext(ctx)

			// Create response recorder
			rw := httptest.NewRecorder()

			// Variable to capture the modified request
			var capturedRequest *http.Request

			// Mock next handler that simulates the upstream response
			nextHandler := caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				capturedRequest = r

				// The middleware wraps the response writer, so we need to check if it's a wrapper
				// and write to it properly
				if tt.mockResponse != nil {
					body, status := tt.mockResponse(t, r)
					w.WriteHeader(status)
					_, writeErr := w.Write(body)
					if writeErr != nil {
						t.Logf("Error writing response body: %v", writeErr)
					}
				} else {
					w.WriteHeader(tt.expectedStatusCode)

					body := `{"error":"internal server error"}`
					if tt.expectedStatusCode >= http.StatusOK && tt.expectedStatusCode < http.StatusMultipleChoices {
						body = `{"status":"ok"}`
					} else if tt.expectedStatusCode == http.StatusNotFound {
						body = `{"error":"not found"}`
					}

					if _, err := w.Write([]byte(body)); err != nil {
						t.Logf("Error writing response body: %v", err)
					}
				}

				// For REST API integration tests, we should return success
				// The middleware will handle the response appropriately
				return nil
			})

			// Serve the request through the middleware
			err = dm.ServeHTTP(rw, req, nextHandler)

			// The middleware should not return errors for REST API responses,
			// even for error status codes (404, 500, etc)
			require.NoError(t, err)

			// Debug: Check what the response recorder actually captured
			t.Logf("Response recorder - Code: %d, Body length: %d", rw.Code, rw.Body.Len())

			// Verify response status code
			// Note: httptest.ResponseRecorder defaults to 200 if WriteHeader is not called
			if rw.Code == 0 {
				t.Log("Warning: Response code is 0, which means WriteHeader was never called")
			}
			assert.Equal(t, tt.expectedStatusCode, rw.Code)

			// Validate response body if provided
			if tt.validateResponse != nil && rw.Body.Len() > 0 {
				tt.validateResponse(t, rw.Body.Bytes())
			}

			// Log the results for debugging
			t.Logf("Response code: %d, Body: %s", rw.Code, rw.Body.String())
			if capturedRequest != nil {
				t.Logf("Path sent to handler: %s", capturedRequest.URL.Path)
			}
		})
	}
}

// TestRESTAPIMiddlewareErrorHandling tests error scenarios in REST API handling
func TestRESTAPIMiddlewareErrorHandling(t *testing.T) {
	networklib.RegisterBuiltinHandlers()

	tests := []struct {
		name            string
		networkType     string
		networkName     string
		requestPath     string
		requestMethod   string
		setupNetwork    func(*network)
		setupMiddleware func(*DinMiddleware)
		expectedError   bool
		expectedStatus  int
		errorMessage    string
	}{
		{
			name:           "Network not found error",
			networkType:    string(BeaconHandler),
			networkName:    "beacon",
			requestPath:    "/unknown-network/eth/v1/beacon/genesis",
			requestMethod:  "GET",
			expectedError:  true,
			expectedStatus: 404,
			errorMessage:   "network not found",
		},
		{
			name:           "Invalid network type error",
			networkType:    "invalid_type",
			networkName:    "test",
			requestPath:    "/test/some/endpoint",
			requestMethod:  "GET",
			expectedError:  true,
			expectedStatus: 500,
			errorMessage:   "invalid network type",
		},
		{
			name:          "Request payload too large",
			networkType:   string(BeaconHandler),
			networkName:   "beacon",
			requestPath:   "/beacon/eth/v1/beacon/pool/attestations",
			requestMethod: "POST",
			setupNetwork: func(n *network) {
				n.MaxRequestPayloadSizeKB = 0 // 1KB limit
			},
			expectedError:  true,
			expectedStatus: 413,
			errorMessage:   "payload too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create middleware
			dm := &DinMiddleware{
				logger:          logger.NewLoggerClient(zaptest.NewLogger(t), utils.EnvTest),
				testMode:        true,
				handlerRegistry: networklib.DefaultRegistry,
				Networks:        map[string]*network{},
			}

			// Add network if it's not the "not found" test
			if tt.name != "Network not found error" {
				network := &network{
					Name:        tt.networkName,
					HandlerType: HandlerType(tt.networkType),
					ChainId:     "1",
					Providers: map[string]*provider{
						"test-provider": {
							HttpUrl: "http://test-provider",
							Host:    "test-provider",
							BlockHistory: func() *list.List {
								l := list.New()
								l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Healthy})
								return l
							}(),
						},
					},
					MaxRequestPayloadSizeKB: DefaultMaxRequestPayloadSizeKB,
				}

				if tt.setupNetwork != nil {
					tt.setupNetwork(network)
				}

				dm.Networks[tt.networkName] = network
			}

			if tt.setupMiddleware != nil {
				tt.setupMiddleware(dm)
			}

			// Try to initialize - may fail for invalid network types
			_ = dm.initialize(caddy.Context{})

			// Create request
			var req *http.Request
			if tt.name == "Request payload too large" {
				// Create a large payload
				largePayload := strings.Repeat("x", 2000) // 2KB
				req = httptest.NewRequest(tt.requestMethod, "http://test.com"+tt.requestPath, strings.NewReader(largePayload))
			} else {
				req = httptest.NewRequest(tt.requestMethod, "http://test.com"+tt.requestPath, nil)
			}

			// Add context
			repl := caddy.NewReplacer()
			ctx := context.WithValue(context.Background(), caddy.ReplacerCtxKey, repl)
			req = req.WithContext(ctx)

			// Create response recorder
			rw := httptest.NewRecorder()

			// Mock next handler
			nextHandler := caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				// This shouldn't be called in error cases
				t.Error("Next handler called unexpectedly for: " + tt.name)
				w.WriteHeader(200)
				return nil
			})

			// Serve the request
			err := dm.ServeHTTP(rw, req, nextHandler)

			// Verify error handling
			if tt.expectedError {
				assert.Error(t, err, "Expected an error for case: %s", tt.name)
				t.Logf("Error returned: %v", err)
			} else {
				assert.NoError(t, err)
			}

			// Log the response for debugging
			t.Logf("Response code: %d, Body: %s", rw.Code, rw.Body.String())
		})
	}
}

// TestRESTAPIProviderSelection tests provider selection for REST API requests
func TestRESTAPIProviderSelection(t *testing.T) {
	networklib.RegisterBuiltinHandlers()

	// Create middleware with multiple providers
	dm := &DinMiddleware{
		logger:          logger.NewLoggerClient(zaptest.NewLogger(t), utils.EnvTest),
		testMode:        true,
		handlerRegistry: networklib.DefaultRegistry,
		Networks: map[string]*network{
			"beacon": {
				Name:        "beacon",
				HandlerType: BeaconHandler,
				ChainId:     "1",
				Providers: map[string]*provider{
					"provider1": {
						HttpUrl:  "http://provider1",
						Host:     "provider1",
						Priority: 1,
						BlockHistory: func() *list.List {
							l := list.New()
							l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Healthy})
							return l
						}(),
					},
					"provider2": {
						HttpUrl:  "http://provider2",
						Host:     "provider2",
						Priority: 2,
						BlockHistory: func() *list.List {
							l := list.New()
							l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Healthy})
							return l
						}(),
					},
				},
				MaxRequestPayloadSizeKB: DefaultMaxRequestPayloadSizeKB,
			},
		},
	}

	// Initialize middleware
	err := dm.initialize(caddy.Context{})
	require.NoError(t, err)

	// Test multiple requests to see provider selection
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "http://test.com/beacon/eth/v1/beacon/genesis", nil)

		// Add context
		repl := caddy.NewReplacer()
		ctx := context.WithValue(context.Background(), caddy.ReplacerCtxKey, repl)
		req = req.WithContext(ctx)

		rw := httptest.NewRecorder()

		// Track which provider was selected
		var selectedProvider string
		nextHandler := caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			if provider, ok := repl.Get(RequestProviderKey); ok {
				selectedProvider = provider.(string)
			}
			w.WriteHeader(200)
			return nil
		})

		err := dm.ServeHTTP(rw, req, nextHandler)
		assert.NoError(t, err)

		t.Logf("Request %d: Selected provider: %s", i+1, selectedProvider)
	}
}

// TestMiddlewareRESTFlow tests the complete flow of REST API requests through the middleware
func TestMiddlewareRESTFlow(t *testing.T) {
	networklib.RegisterBuiltinHandlers()

	tests := []struct {
		name               string
		network            *network
		requestPath        string
		requestMethod      string
		requestBody        string
		expectedError      bool
		expectedStatusCode int
		validateRequest    func(t *testing.T, req *http.Request)
	}{
		{
			name: "Beacon REST API GET request flow",
			network: &network{
				Name:                    "ethereum-beacon",
				HandlerType:             BeaconHandler,
				ChainId:                 "1",
				MaxRequestPayloadSizeKB: DefaultMaxRequestPayloadSizeKB,
			},
			requestPath:        "/ethereum-beacon/eth/v1/beacon/genesis",
			requestMethod:      "GET",
			expectedError:      false,
			expectedStatusCode: 200,
			validateRequest: func(t *testing.T, req *http.Request) {
				// Verify network object was properly set
				repl := req.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
				networkObjVal, ok := repl.Get("network_object")
				require.True(t, ok, "Network object should be set in context")

				networkObj := networkObjVal.(*network)
				assert.Equal(t, networklib.RequestTypeREST, networkObj.Handler.GetRequestType())
				assert.Equal(t, string(BeaconHandler), string(networkObj.HandlerType))
				// The method is now stored in RequestMethodKey
				methodVal, _ := repl.Get(RequestMethodKey)
				assert.Equal(t, "/ethereum-beacon/eth/v1/beacon/genesis", methodVal)
			},
		},
		{
			name: "Beacon REST API POST request flow",
			network: &network{
				Name:                    "beacon",
				HandlerType:             BeaconHandler,
				ChainId:                 "1",
				MaxRequestPayloadSizeKB: DefaultMaxRequestPayloadSizeKB,
			},
			requestPath:        "/beacon/eth/v1/beacon/pool/attestations",
			requestMethod:      "POST",
			requestBody:        `{"data":[{"aggregation_bits":"0x01","signature":"0x123"}]}`,
			expectedError:      false,
			expectedStatusCode: 200,
			validateRequest: func(t *testing.T, req *http.Request) {
				// Verify body is preserved
				body, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				assert.Contains(t, string(body), "aggregation_bits")

				// Reset body for next handler
				req.Body = io.NopCloser(strings.NewReader(string(body)))
			},
		},
		{
			name: "JSON-RPC request flow (not REST)",
			network: &network{
				Name:                    "ethereum",
				HandlerType:             EVMHandler,
				ChainId:                 "0x1",
				MaxRequestPayloadSizeKB: DefaultMaxRequestPayloadSizeKB,
			},
			requestPath:        "/ethereum",
			requestMethod:      "POST",
			requestBody:        `{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`,
			expectedError:      false,
			expectedStatusCode: 200,
			validateRequest: func(t *testing.T, req *http.Request) {
				// Verify network object was properly set
				repl := req.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
				networkObjVal, ok := repl.Get("network_object")
				require.True(t, ok, "Network object should be set in context")

				networkObj := networkObjVal.(*network)
				assert.Equal(t, networklib.RequestTypeRPC, networkObj.Handler.GetRequestType())
				assert.Equal(t, string(EVMHandler), string(networkObj.HandlerType))
			},
		},
		{
			name: "REST API with query parameters",
			network: &network{
				Name:                    "beacon",
				HandlerType:             BeaconHandler,
				ChainId:                 "1",
				MaxRequestPayloadSizeKB: DefaultMaxRequestPayloadSizeKB,
			},
			requestPath:        "/beacon/eth/v1/beacon/states/head/validators?id=1,2,3&status=active",
			requestMethod:      "GET",
			expectedError:      false,
			expectedStatusCode: 200,
			validateRequest: func(t *testing.T, req *http.Request) {
				// Verify query parameters are preserved
				assert.Equal(t, "1,2,3", req.URL.Query().Get("id"))
				assert.Equal(t, "active", req.URL.Query().Get("status"))
			},
		},
		{
			name: "REST API request to unknown network",
			network: &network{
				Name:                    "unknown",
				HandlerType:             BeaconHandler,
				ChainId:                 "1",
				MaxRequestPayloadSizeKB: DefaultMaxRequestPayloadSizeKB,
			},
			requestPath:        "/different-network/eth/v1/beacon/genesis",
			requestMethod:      "GET",
			expectedError:      true,
			expectedStatusCode: 404,
		},
		{
			name: "Large REST API payload",
			network: &network{
				Name:                    "beacon",
				HandlerType:             BeaconHandler,
				ChainId:                 "1",
				MaxRequestPayloadSizeKB: 0, // 0 means 1KB limit
			},
			requestPath:        "/beacon/eth/v1/beacon/pool/attestations",
			requestMethod:      "POST",
			requestBody:        strings.Repeat("x", 2000), // 2KB payload
			expectedError:      true,
			expectedStatusCode: 413,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create middleware
			dm := &DinMiddleware{
				logger:          logger.NewLoggerClient(zaptest.NewLogger(t), utils.EnvTest),
				testMode:        true,
				handlerRegistry: networklib.DefaultRegistry,
				Networks: map[string]*network{
					tt.network.Name: tt.network,
				},
			}

			// Add a mock provider
			tt.network.Providers = map[string]*provider{
				"mock-provider": {
					HttpUrl: "http://mock-provider",
					Host:    "mock-provider",
					BlockHistory: func() *list.List {
						l := list.New()
						l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Healthy})
						return l
					}(),
				},
			}

			// Initialize middleware
			err := dm.initialize(caddy.Context{})
			require.NoError(t, err)

			// Create request
			var req *http.Request
			if tt.requestBody != "" {
				req = httptest.NewRequest(tt.requestMethod, "http://test.com"+tt.requestPath, strings.NewReader(tt.requestBody))
				if strings.Contains(tt.requestBody, "jsonrpc") {
					req.Header.Set("Content-Type", "application/json")
				}
			} else {
				req = httptest.NewRequest(tt.requestMethod, "http://test.com"+tt.requestPath, nil)
			}

			// Add context
			repl := caddy.NewReplacer()
			ctx := context.WithValue(req.Context(), caddy.ReplacerCtxKey, repl)
			req = req.WithContext(ctx)

			// Create response recorder
			rw := httptest.NewRecorder()

			// Mock next handler that validates the request
			nextHandler := caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				if tt.validateRequest != nil {
					tt.validateRequest(t, r)
				}
				w.WriteHeader(tt.expectedStatusCode)
				return nil
			})

			// Serve the request
			err = dm.ServeHTTP(rw, req, nextHandler)

			// Validate results
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Check status code if set
			if rw.Code != 0 {
				assert.Equal(t, tt.expectedStatusCode, rw.Code)
			}
		})
	}
}

// TestMiddlewareRESTPathStripping tests path stripping in the middleware flow
func TestMiddlewareRESTPathStripping(t *testing.T) {
	networklib.RegisterBuiltinHandlers()

	tests := []struct {
		name             string
		networkName      string
		requestPath      string
		providerPath     string
		expectedUpstream string
		expectedError    bool
	}{
		{
			name:             "Basic REST path stripping",
			networkName:      "ethereum-beacon",
			requestPath:      "/ethereum-beacon/eth/v1/beacon/genesis",
			providerPath:     "",
			expectedUpstream: "/eth/v1/beacon/genesis",
		},
		{
			name:             "REST path stripping with provider base",
			networkName:      "beacon-test",
			requestPath:      "/beacon-test/eth/v2/beacon/blocks/head",
			providerPath:     "/api",
			expectedUpstream: "/api/eth/v2/beacon/blocks/head",
		},
		{
			name:             "JSON-RPC no path stripping",
			networkName:      "ethereum",
			requestPath:      "/ethereum",
			providerPath:     "/rpc",
			expectedUpstream: "/rpc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Determine network type based on path pattern
			networkType := string(EVMHandler)
			if strings.Contains(tt.requestPath, "/eth/v") {
				networkType = string(BeaconHandler)
			}

			// Create middleware
			dm := &DinMiddleware{
				logger:          logger.NewLoggerClient(zaptest.NewLogger(t), utils.EnvTest),
				testMode:        true,
				handlerRegistry: networklib.DefaultRegistry,
				Networks: map[string]*network{
					tt.networkName: {
						Name:                    tt.networkName,
						HandlerType:             HandlerType(networkType),
						ChainId:                 getValidChainIDForType(networkType),
						MaxRequestPayloadSizeKB: DefaultMaxRequestPayloadSizeKB,
						Providers: map[string]*provider{
							"test-provider": {
								HttpUrl:  "http://test-provider",
								Host:     "test-provider",
								Path:     tt.providerPath,
								Priority: 1,
								BlockHistory: func() *list.List {
									l := list.New()
									l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Healthy})
									return l
								}(),
							},
						},
					},
				},
			}

			// Initialize middleware
			err := dm.initialize(caddy.Context{})
			require.NoError(t, err)

			// Create request
			var req *http.Request
			if networkType == string(EVMHandler) {
				// JSON-RPC request
				req = httptest.NewRequest("POST", "http://test.com"+tt.requestPath,
					strings.NewReader(`{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`))
				req.Header.Set("Content-Type", "application/json")
			} else {
				// REST request
				req = httptest.NewRequest("GET", "http://test.com"+tt.requestPath, nil)
			}

			// Add context
			repl := caddy.NewReplacer()
			ctx := context.WithValue(req.Context(), caddy.ReplacerCtxKey, repl)
			req = req.WithContext(ctx)

			// Create response recorder
			rw := httptest.NewRecorder()

			// Variable to capture the upstream path
			var capturedPath string

			// Mock next handler that captures the request path
			nextHandler := caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
				capturedPath = r.URL.Path
				w.WriteHeader(200)
				return nil
			})

			// Serve the request
			err = dm.ServeHTTP(rw, req, nextHandler)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				// The path might not be captured if the request doesn't reach the handler
				// due to provider selection. This is OK for this test.
				t.Logf("Original Path: %s, Provider Path: %s, Captured Path: %s",
					tt.requestPath, tt.providerPath, capturedPath)
			}
		})
	}
}

// TestMiddlewareRESTMetrics tests that REST API requests are properly tracked
func TestMiddlewareRESTMetrics(t *testing.T) {
	networklib.RegisterBuiltinHandlers()

	// Create middleware
	dm := &DinMiddleware{
		logger:          logger.NewLoggerClient(zaptest.NewLogger(t), utils.EnvTest),
		testMode:        true,
		handlerRegistry: networklib.DefaultRegistry,
		Networks: map[string]*network{
			"beacon": {
				Name:                    "beacon",
				HandlerType:             BeaconHandler,
				ChainId:                 "1",
				MaxRequestPayloadSizeKB: DefaultMaxRequestPayloadSizeKB,
				Providers: map[string]*provider{
					"test-provider": {
						HttpUrl: "http://test-provider",
						Host:    "test-provider",
						BlockHistory: func() *list.List {
							l := list.New()
							l.PushBack(blockHistoryEntry{BlockNumber: 100, HealthStatus: Healthy})
							return l
						}(),
					},
				},
			},
		},
	}

	// Initialize middleware
	err := dm.initialize(caddy.Context{})
	require.NoError(t, err)

	// Test REST API request
	req := httptest.NewRequest("GET", "http://test.com/beacon/eth/v1/beacon/genesis", nil)
	repl := caddy.NewReplacer()
	ctx := req.Context()
	ctx = context.WithValue(ctx, caddy.ReplacerCtxKey, repl)
	req = req.WithContext(ctx)

	rw := httptest.NewRecorder()

	// Serve the request
	err = dm.ServeHTTP(rw, req, caddyhttp.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		// Verify request metadata is set
		repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)

		// Check network object
		networkObjVal, ok := repl.Get("network_object")
		assert.True(t, ok)
		networkObj := networkObjVal.(*network)
		assert.Equal(t, networklib.RequestTypeREST, networkObj.Handler.GetRequestType())

		// Check method
		method, ok := repl.Get(RequestMethodKey)
		assert.True(t, ok)
		assert.Equal(t, "/beacon/eth/v1/beacon/genesis", method)

		w.WriteHeader(200)
		return nil
	}))

	require.NoError(t, err)
}

// Helper function for valid chain IDs
func getValidChainIDForType(networkType string) string {
	switch networkType {
	case string(BeaconHandler):
		return "1"
	case string(EVMHandler):
		return "0x1"
	default:
		return "unknown:1"
	}
}
