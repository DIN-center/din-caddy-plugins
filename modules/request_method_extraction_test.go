package modules

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dinHttp "github.com/DIN-center/din-caddy-plugins/lib/http"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	"github.com/caddyserver/caddy/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestRequestMethodExtraction(t *testing.T) {
	// Register built-in handlers
	networklib.RegisterBuiltinHandlers()
	registry := networklib.DefaultRegistry

	tests := []struct {
		name           string
		network        *network
		request        *http.Request
		expectedMethod string
		requestType    networklib.RequestType
	}{
		// REST API Tests
		{
			name: "REST request method extraction - Beacon v1",
			network: &network{
				Name:    "ethereum-beacon",
				Type:    "beacon-chain",
				ChainId: "beacon:1",
			},
			request:        httptest.NewRequest("GET", "/ethereum-beacon/eth/v1/beacon/genesis", nil),
			expectedMethod: "/ethereum-beacon/eth/v1/beacon/genesis",
			requestType:    networklib.RequestTypeREST,
		},
		{
			name: "REST request method extraction - Beacon v2",
			network: &network{
				Name:    "beacon-mainnet",
				Type:    "beacon-chain",
				ChainId: "beacon:1",
			},
			request:        httptest.NewRequest("GET", "/beacon-mainnet/eth/v2/beacon/blocks/head", nil),
			expectedMethod: "/beacon-mainnet/eth/v2/beacon/blocks/head",
			requestType:    networklib.RequestTypeREST,
		},
		{
			name: "REST request with query parameters",
			network: &network{
				Name:    "beacon",
				Type:    "beacon-chain",
				ChainId: "beacon:1",
			},
			request:        httptest.NewRequest("GET", "/beacon/eth/v1/beacon/states/head/validators?id=1,2,3", nil),
			expectedMethod: "/beacon/eth/v1/beacon/states/head/validators",
			requestType:    networklib.RequestTypeREST,
		},
		{
			name: "REST POST request",
			network: &network{
				Name:    "beacon",
				Type:    "beacon-chain",
				ChainId: "beacon:1",
			},
			request: func() *http.Request {
				req := httptest.NewRequest("POST", "/beacon/eth/v1/beacon/pool/attestations",
					strings.NewReader(`{"data":[{"aggregation_bits":"0x01"}]}`))
				req.Header.Set("Content-Type", "application/json")
				return req
			}(),
			expectedMethod: "/beacon/eth/v1/beacon/pool/attestations",
			requestType:    networklib.RequestTypeREST,
		},
		{
			name: "REST request auto-detection without explicit type",
			network: &network{
				Name:    "beacon-test",
				Type:    "", // No explicit type
				ChainId: "beacon:1",
			},
			request:        httptest.NewRequest("GET", "/beacon-test/eth/v1/node/version", nil),
			expectedMethod: "/beacon-test/eth/v1/node/version",
			requestType:    networklib.RequestTypeREST,
		},
		// JSON-RPC Tests
		{
			name: "JSON-RPC request method extraction",
			network: &network{
				Name:    "ethereum",
				Type:    "evm",
				ChainId: "eip155:0x1",
			},
			request: func() *http.Request {
				req := httptest.NewRequest("POST", "/ethereum",
					strings.NewReader(`{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`))
				req.Header.Set("Content-Type", "application/json")
				return req
			}(),
			expectedMethod: "unknown", // Will be updated when request body is parsed later
			requestType:    networklib.RequestTypeRPC,
		},
		{
			name: "Solana JSON-RPC request method extraction",
			network: &network{
				Name:    "solana-mainnet",
				Type:    "solana",
				ChainId: "solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp",
			},
			request: func() *http.Request {
				req := httptest.NewRequest("POST", "/solana-mainnet",
					strings.NewReader(`{"jsonrpc":"2.0","method":"getSlot","params":[],"id":1}`))
				req.Header.Set("Content-Type", "application/json")
				return req
			}(),
			expectedMethod: "unknown", // Will be updated when request body is parsed later
			requestType:    networklib.RequestTypeRPC,
		},
		{
			name: "Starknet JSON-RPC request",
			network: &network{
				Name:    "starknet-mainnet",
				Type:    "starknet",
				ChainId: "starknet:0x534e5f4d41494e",
			},
			request: func() *http.Request {
				req := httptest.NewRequest("POST", "/starknet-mainnet",
					strings.NewReader(`{"jsonrpc":"2.0","method":"starknet_getBlockWithTxs","params":[],"id":1}`))
				req.Header.Set("Content-Type", "application/json")
				return req
			}(),
			expectedMethod: "unknown",
			requestType:    networklib.RequestTypeRPC,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqContext, err := DetectRequestType(tt.request, tt.network, registry)
			require.NoError(t, err)

			// Verify request type is correct
			assert.Equal(t, tt.requestType, reqContext.Type)

			// Verify method is properly set based on request type
			if tt.requestType == networklib.RequestTypeREST {
				// For REST requests, method should be the original path
				assert.Equal(t, tt.expectedMethod, reqContext.Method)
				assert.NotEmpty(t, reqContext.OriginalPath)
			} else if tt.requestType == networklib.RequestTypeRPC {
				// For RPC requests, method starts as "unknown" until body is parsed
				assert.Equal(t, tt.expectedMethod, reqContext.Method)
			}

			// Verify handler is properly set
			assert.NotNil(t, reqContext.Handler)
			if tt.network.Type != "" {
				assert.Equal(t, tt.network.Type, reqContext.NetworkType)
			} else {
				// Auto-detection should have filled in the network type
				assert.NotEmpty(t, reqContext.NetworkType)
			}
		})
	}
}

func TestLogParameterExtraction(t *testing.T) {
	tests := []struct {
		name           string
		requestType    networklib.RequestType
		originalPath   string
		contextMethod  string
		requestBody    *dinHttp.JSONRPCRequest
		expectedMethod string
		expectedParams json.RawMessage
	}{
		// REST API parameter tests
		{
			name:           "REST request parameter extraction",
			requestType:    networklib.RequestTypeREST,
			originalPath:   "/ethereum-beacon/eth/v1/beacon/genesis",
			contextMethod:  "/ethereum-beacon/eth/v1/beacon/genesis",
			requestBody:    nil,
			expectedMethod: "/ethereum-beacon/eth/v1/beacon/genesis",
			expectedParams: nil,
		},
		{
			name:           "REST request with query params",
			requestType:    networklib.RequestTypeREST,
			originalPath:   "/beacon/eth/v1/beacon/states/head/validators?id=1,2,3&status=active",
			contextMethod:  "/beacon/eth/v1/beacon/states/head/validators?id=1,2,3&status=active",
			requestBody:    nil,
			expectedMethod: "/beacon/eth/v1/beacon/states/head/validators?id=1,2,3&status=active",
			expectedParams: nil,
		},
		{
			name:           "REST POST with JSON body (not JSON-RPC)",
			requestType:    networklib.RequestTypeREST,
			originalPath:   "/beacon/eth/v1/beacon/pool/attestations",
			contextMethod:  "/beacon/eth/v1/beacon/pool/attestations",
			requestBody:    nil, // REST APIs don't parse into JSONRPCRequest
			expectedMethod: "/beacon/eth/v1/beacon/pool/attestations",
			expectedParams: nil,
		},
		{
			name:           "REST v2 endpoint",
			requestType:    networklib.RequestTypeREST,
			originalPath:   "/beacon-mainnet/eth/v2/beacon/blocks/head",
			contextMethod:  "/beacon-mainnet/eth/v2/beacon/blocks/head",
			requestBody:    nil,
			expectedMethod: "/beacon-mainnet/eth/v2/beacon/blocks/head",
			expectedParams: nil,
		},
		{
			name:           "REST request with no JSON-RPC body",
			requestType:    networklib.RequestTypeREST,
			originalPath:   "/beacon/eth/v1/node/health",
			contextMethod:  "/beacon/eth/v1/node/health",
			requestBody:    nil,
			expectedMethod: "/beacon/eth/v1/node/health",
			expectedParams: nil,
		},
		// JSON-RPC parameter tests
		{
			name:         "JSON-RPC request parameter extraction",
			requestType:  networklib.RequestTypeRPC,
			originalPath: "/ethereum",
			requestBody: &dinHttp.JSONRPCRequest{
				Method: "eth_blockNumber",
				Params: json.RawMessage(`[]`),
			},
			expectedMethod: "eth_blockNumber",
			expectedParams: json.RawMessage(`[]`),
		},
		{
			name:         "JSON-RPC request with object parameters",
			requestType:  networklib.RequestTypeRPC,
			originalPath: "/ethereum",
			requestBody: &dinHttp.JSONRPCRequest{
				Method: "eth_call",
				Params: json.RawMessage(`[{"to":"0x123","data":"0x456"},{"latest":"true"}]`),
			},
			expectedMethod: "eth_call",
			expectedParams: json.RawMessage(`[{"to":"0x123","data":"0x456"},{"latest":"true"}]`),
		},
		{
			name:         "Solana JSON-RPC with parameters",
			requestType:  networklib.RequestTypeRPC,
			originalPath: "/solana",
			requestBody: &dinHttp.JSONRPCRequest{
				Method: "getBalance",
				Params: json.RawMessage(`["vines1vzrYbzLMRdu58ou5XTby4qAqVRLmqo36NKPTg"]`),
			},
			expectedMethod: "getBalance",
			expectedParams: json.RawMessage(`["vines1vzrYbzLMRdu58ou5XTby4qAqVRLmqo36NKPTg"]`),
		},
		{
			name:         "Starknet JSON-RPC with complex params",
			requestType:  networklib.RequestTypeRPC,
			originalPath: "/starknet",
			requestBody: &dinHttp.JSONRPCRequest{
				Method: "starknet_call",
				Params: json.RawMessage(`[{"contract_address":"0x123","entry_point_selector":"0x456","calldata":["0x789"]},"latest"]`),
			},
			expectedMethod: "starknet_call",
			expectedParams: json.RawMessage(`[{"contract_address":"0x123","entry_point_selector":"0x456","calldata":["0x789"]},"latest"]`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the logging extraction logic from our fixes
			var method string = "unknown"
			var params json.RawMessage

			// Mock request context
			reqContext := &RequestProcessor{
				Type:         tt.requestType,
				Method:       tt.contextMethod,
				OriginalPath: tt.originalPath,
			}

			// Apply the same logic we implemented in the middleware
			if reqContext.Type == networklib.RequestTypeREST {
				// For REST requests, use the path as the method
				method = reqContext.OriginalPath
				if reqContext.Method != "" {
					method = reqContext.Method
				}
				// REST requests don't have JSON-RPC style parameters
				params = nil
			} else if tt.requestBody != nil {
				// For JSON-RPC requests, extract from request body
				method = tt.requestBody.Method
				if len(tt.requestBody.Params) > 0 {
					params = tt.requestBody.Params
				}
			}

			// Verify extraction worked correctly
			assert.Equal(t, tt.expectedMethod, method)
			if tt.expectedParams == nil {
				assert.Nil(t, params)
			} else {
				assert.Equal(t, tt.expectedParams, params)
			}
		})
	}
}

func TestApplyProviderConfiguration_PathStripping(t *testing.T) {
	tests := []struct {
		name         string
		requestPath  string
		requestType  networklib.RequestType
		providerPath string
		providerURL  string
		expectedPath string
		description  string
	}{
		// REST API Path Stripping Tests
		{
			name:         "REST request strips network prefix - beacon",
			requestPath:  "/ethereum-beacon/eth/v1/beacon/genesis",
			requestType:  networklib.RequestTypeREST,
			providerPath: "",
			providerURL:  "https://beacon-provider.example.com",
			expectedPath: "/eth/v1/beacon/genesis",
			description:  "Should remove network prefix from REST API path",
		},
		{
			name:         "REST request strips network prefix - with provider base path",
			requestPath:  "/ethereum-beacon/eth/v1/beacon/blocks/head",
			requestType:  networklib.RequestTypeREST,
			providerPath: "/v2",
			providerURL:  "https://beacon-provider.example.com",
			expectedPath: "/v2/eth/v1/beacon/blocks/head",
			description:  "Should combine provider base path with stripped path",
		},
		{
			name:         "REST request strips network prefix - provider trailing slash",
			requestPath:  "/solana-mainnet/api/v1/accounts",
			requestType:  networklib.RequestTypeREST,
			providerPath: "/base/",
			providerURL:  "https://solana-provider.example.com",
			expectedPath: "/base/api/v1/accounts",
			description:  "Should handle trailing slash in provider path correctly",
		},
		{
			name:         "REST request with single segment path",
			requestPath:  "/beacon",
			requestType:  networklib.RequestTypeREST,
			providerPath: "",
			providerURL:  "https://beacon-provider.example.com",
			expectedPath: "/beacon",
			description:  "Should not strip single segment paths",
		},
		{
			name:         "REST request with empty path after network",
			requestPath:  "/ethereum-beacon/",
			requestType:  networklib.RequestTypeREST,
			providerPath: "",
			providerURL:  "https://beacon-provider.example.com",
			expectedPath: "/",
			description:  "Should handle empty path after network prefix",
		},
		{
			name:         "REST request with query parameters",
			requestPath:  "/ethereum-beacon/eth/v1/beacon/states/head/validators?id=1234",
			requestType:  networklib.RequestTypeREST,
			providerPath: "",
			providerURL:  "https://beacon-provider.example.com",
			expectedPath: "/eth/v1/beacon/states/head/validators",
			description:  "Query parameters are preserved separately in req.URL.RawQuery",
		},
		{
			name:         "REST request with URL encoded path",
			requestPath:  "/ethereum-beacon/eth/v1/beacon/blocks/0x%20abc",
			requestType:  networklib.RequestTypeREST,
			providerPath: "",
			providerURL:  "https://beacon-provider.example.com",
			expectedPath: "/eth/v1/beacon/blocks/0x abc",
			description:  "URL decoding happens automatically in httptest.NewRequest",
		},
		// JSON-RPC Path Tests (no stripping)
		{
			name:         "JSON-RPC request preserves full path",
			requestPath:  "/ethereum",
			requestType:  networklib.RequestTypeRPC,
			providerPath: "",
			providerURL:  "https://eth-provider.example.com",
			expectedPath: "/ethereum",
			description:  "JSON-RPC requests should not strip paths",
		},
		{
			name:         "JSON-RPC request with provider path",
			requestPath:  "/ethereum",
			requestType:  networklib.RequestTypeRPC,
			providerPath: "/rpc",
			providerURL:  "https://eth-provider.example.com",
			expectedPath: "/rpc",
			description:  "JSON-RPC requests use provider path directly",
		},
		// Edge Cases
		{
			name:         "REST request with multiple slashes",
			requestPath:  "/ethereum-beacon//eth//v1//beacon",
			requestType:  networklib.RequestTypeREST,
			providerPath: "",
			providerURL:  "https://beacon-provider.example.com",
			expectedPath: "//eth//v1//beacon",
			description:  "Should preserve multiple slashes after stripping",
		},
		{
			name:         "REST request with dots in path",
			requestPath:  "/ethereum-beacon/../eth/v1/beacon",
			requestType:  networklib.RequestTypeREST,
			providerPath: "",
			providerURL:  "https://beacon-provider.example.com",
			expectedPath: "/../eth/v1/beacon",
			description:  "Should preserve path traversal segments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test infrastructure
			zapLogger := zaptest.NewLogger(t)
			dinSelect := &DinSelect{
				logger: zapLogger,
			}

			// Create provider
			provider := &provider{
				HttpUrl: tt.providerURL,
				path:    tt.providerPath,
			}

			// Create request
			req := httptest.NewRequest("GET", "http://test.com"+tt.requestPath, nil)

			// Create request context
			reqContext := &RequestProcessor{
				Type:         tt.requestType,
				OriginalPath: tt.requestPath,
			}

			// Create response writer and replacer
			rw := httptest.NewRecorder()
			repl := caddy.NewReplacer()

			// Apply provider configuration
			dinSelect.applyProviderConfiguration(provider, req, rw, repl, reqContext)

			// Verify the path was transformed correctly
			assert.Equal(t, tt.expectedPath, req.URL.Path, tt.description)
		})
	}
}

func TestApplyProviderConfiguration_PathStripping_ComplexScenarios(t *testing.T) {
	tests := []struct {
		name         string
		requestPath  string
		requestType  networklib.RequestType
		providerPath string
		providerURL  string
		rawPath      string // For testing RawPath handling
		expectedPath string
		expectedRaw  string
		description  string
	}{
		{
			name:         "REST with invalid provider URL",
			requestPath:  "/ethereum-beacon/eth/v1/beacon/genesis",
			requestType:  networklib.RequestTypeREST,
			providerPath: "/base",
			providerURL:  "://invalid-url", // Invalid URL
			expectedPath: "/eth/v1/beacon/genesis",
			expectedRaw:  "",
			description:  "Should fallback to standard path handling on URL parse error",
		},
		{
			name:         "JSON-RPC with escaped provider path",
			requestPath:  "/ethereum",
			requestType:  networklib.RequestTypeRPC,
			providerPath: "/rpc%20endpoint",
			providerURL:  "https://eth-provider.example.com",
			rawPath:      "/rpc%20endpoint",
			expectedPath: "/rpc endpoint", // Unescaped
			expectedRaw:  "/rpc%20endpoint",
			description:  "JSON-RPC should handle escaped provider paths",
		},
		{
			name:         "REST with root provider path",
			requestPath:  "/ethereum-beacon/eth/v1/beacon/genesis",
			requestType:  networklib.RequestTypeREST,
			providerPath: "/",
			providerURL:  "https://beacon-provider.example.com",
			expectedPath: "/eth/v1/beacon/genesis",
			expectedRaw:  "",
			description:  "Root provider path should be ignored for REST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test infrastructure
			zapLogger := zaptest.NewLogger(t)
			dinSelect := &DinSelect{
				logger: zapLogger,
			}

			// Create provider
			provider := &provider{
				HttpUrl: tt.providerURL,
				path:    tt.providerPath,
			}

			// Create request
			req := httptest.NewRequest("GET", "http://test.com"+tt.requestPath, nil)
			if tt.rawPath != "" {
				req.URL.RawPath = tt.rawPath
			}

			// Create request context
			reqContext := &RequestProcessor{
				Type:         tt.requestType,
				OriginalPath: tt.requestPath,
			}

			// Create response writer and replacer
			rw := httptest.NewRecorder()
			repl := caddy.NewReplacer()

			// Apply provider configuration
			dinSelect.applyProviderConfiguration(provider, req, rw, repl, reqContext)

			// Verify the path was transformed correctly
			assert.Equal(t, tt.expectedPath, req.URL.Path, tt.description)
			assert.Equal(t, tt.expectedRaw, req.URL.RawPath, "RawPath should match expected")
		})
	}
}

func TestPathStrippingLogic(t *testing.T) {
	tests := []struct {
		name         string
		inputPath    string
		expectedPath string
		description  string
	}{
		{
			name:         "Standard network prefix removal",
			inputPath:    "/ethereum-beacon/eth/v1/beacon/genesis",
			expectedPath: "/eth/v1/beacon/genesis",
			description:  "Should remove first segment",
		},
		{
			name:         "Network with hyphens",
			inputPath:    "/eth-beacon-mainnet/eth/v1/beacon/blocks/head",
			expectedPath: "/eth/v1/beacon/blocks/head",
			description:  "Should handle network names with multiple hyphens",
		},
		{
			name:         "Network with numbers",
			inputPath:    "/ethereum2-beacon/eth/v2/beacon/blocks/head",
			expectedPath: "/eth/v2/beacon/blocks/head",
			description:  "Should handle network names with numbers",
		},
		{
			name:         "Very long network name",
			inputPath:    "/this-is-a-very-long-network-name-for-testing/api/v1/data",
			expectedPath: "/api/v1/data",
			description:  "Should handle long network names",
		},
		{
			name:         "Path with fragment",
			inputPath:    "/ethereum-beacon/eth/v1/beacon/genesis#section",
			expectedPath: "/eth/v1/beacon/genesis#section",
			description:  "Should preserve URL fragments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the core path stripping logic
			pathSegments := splitPath(tt.inputPath)
			if len(pathSegments) > 1 {
				strippedPath := "/" + joinPath(pathSegments[1:])
				assert.Equal(t, tt.expectedPath, strippedPath, tt.description)
			}
		})
	}
}

// Helper functions to test path manipulation logic
func splitPath(path string) []string {
	trimmed := path
	if path != "" && path[0] == '/' {
		trimmed = path[1:]
	}

	// Handle empty path
	if trimmed == "" {
		return []string{}
	}

	// Split by / but preserve the structure
	segments := []string{}
	current := ""
	for i, ch := range trimmed {
		if ch == '/' {
			segments = append(segments, current)
			current = ""
		} else {
			current += string(ch)
			if i == len(trimmed)-1 {
				segments = append(segments, current)
			}
		}
	}

	return segments
}

func joinPath(segments []string) string {
	if len(segments) == 0 {
		return ""
	}

	result := ""
	for i, segment := range segments {
		result += segment
		if i < len(segments)-1 {
			result += "/"
		}
	}

	return result
}
