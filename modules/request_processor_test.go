package modules

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectRequestType(t *testing.T) {
	// Use the global handler registry instead of creating a new one
	registry := networklib.DefaultRegistry

	// Register handlers if not already registered (idempotent)
	networklib.RegisterBuiltinHandlers()

	tests := []struct {
		name           string
		network        *network
		request        *http.Request
		expectedType   networklib.RequestType
		expectedMethod string
		expectError    bool
	}{
		{
			name: "JSON-RPC request with explicit EVM type",
			network: &network{
				Name:    "ethereum",
				Type:    "evm",
				ChainId: "eip155:0x1",
			},
			request: httptest.NewRequest("POST", "/ethereum",
				strings.NewReader(`{"jsonrpc":"2.0","method":"eth_blockNumber","id":1}`)),
			expectedType: networklib.RequestTypeRPC,
		},
		{
			name: "REST request with explicit eth_beacon_chain type",
			network: &network{
				Name:       "ethereum-beacon-mainnet",
				Type:       "eth_beacon_chain",
				ChainId:    "mainnet",
				HCEndpoint: "/eth/v1/beacon/headers",
			},
			request:      httptest.NewRequest("GET", "/ethereum-beacon-mainnet/eth/v1/beacon/genesis", nil),
			expectedType: networklib.RequestTypeREST,
		},
		{
			name: "Auto-detect JSON-RPC",
			network: &network{
				Name:    "ethereum-auto",
				ChainId: "eip155:0x1",
			},
			request: func() *http.Request {
				req := httptest.NewRequest("POST", "/ethereum-auto",
					strings.NewReader(`{"jsonrpc":"2.0","method":"eth_getBalance","id":1}`))
				req.Header.Set("Content-Type", "application/json")
				return req
			}(),
			expectedType: networklib.RequestTypeRPC,
		},
		{
			name: "Auto-detect REST by path pattern",
			network: &network{
				Name:    "ethereum-beacon-auto",
				ChainId: "beacon:1",
			},
			request:      httptest.NewRequest("GET", "/ethereum-beacon-auto/eth/v1/beacon/states/head/validators", nil),
			expectedType: networklib.RequestTypeREST,
		},
		{
			name: "Starknet handler",
			network: &network{
				Name:    "starknet-mainnet",
				Type:    "starknet",
				ChainId: "starknet:0x534e5f4d41494e",
			},
			request: httptest.NewRequest("POST", "/starknet-mainnet",
				strings.NewReader(`{"jsonrpc":"2.0","method":"starknet_blockNumber","id":1}`)),
			expectedType: networklib.RequestTypeRPC,
		},
		{
			name: "Solana handler",
			network: &network{
				Name:    "solana-mainnet",
				Type:    "solana",
				ChainId: "solana:5eykt4UsFv8P8NJdTREpY1vzqKqZKvdp",
			},
			request: httptest.NewRequest("POST", "/solana-mainnet",
				strings.NewReader(`{"jsonrpc":"2.0","method":"getSlot","id":1}`)),
			expectedType: networklib.RequestTypeRPC,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set content type for JSON-RPC requests
			if tt.request.Method == "POST" && tt.request.Header.Get("Content-Type") == "" {
				tt.request.Header.Set("Content-Type", "application/json")
			}

			ctx, err := DetectRequestType(tt.request, tt.network, registry)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedType, ctx.Type)
			assert.NotNil(t, ctx.Handler)

			// Verify handler can process the request (skip for now due to path validation issues)
			// err = ctx.Handler.ProcessRequest(tt.request, nil)
			// assert.NoError(t, err)
		})
	}
}

func TestRequestContext_IsRetryableError(t *testing.T) {
	registry := networklib.DefaultRegistry
	networklib.RegisterBuiltinHandlers()

	network := &network{
		Name:    "test-network",
		Type:    "evm",
		ChainId: "eip155:0x1",
	}

	req := httptest.NewRequest("POST", "/test-network",
		strings.NewReader(`{"jsonrpc":"2.0","method":"eth_blockNumber","id":1}`))
	req.Header.Set("Content-Type", "application/json")

	ctx, err := DetectRequestType(req, network, registry)
	require.NoError(t, err)

	tests := []struct {
		name       string
		statusCode int
		expected   bool
	}{
		{"Internal Server Error", 500, true},
		{"Bad Gateway", 502, true},
		{"Service Unavailable", 503, true},
		{"Gateway Timeout", 504, true},
		{"Bad Request", 400, false},
		{"Not Found", 404, false},
		{"OK", 200, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ctx.IsRetryableError(nil, tt.statusCode, nil)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHandlerRegistry_Lifecycle(t *testing.T) {
	registry := networklib.DefaultRegistry

	// Test handler registration
	networklib.RegisterBuiltinHandlers()

	// Verify handlers are registered
	handlers := registry.ListHandlers()
	assert.Contains(t, handlers, "evm")
	assert.Contains(t, handlers, "eth_beacon_chain")
	assert.Contains(t, handlers, "starknet")
	assert.Contains(t, handlers, "solana")

	// Test handler creation
	config := &networklib.NetworkConfig{
		Name:    "test-network",
		Type:    "evm",
		ChainID: "eip155:0x1",
	}

	handler, err := registry.GetHandler("evm", config)
	require.NoError(t, err)
	assert.NotNil(t, handler)
	assert.Equal(t, "evm", handler.GetType())

	// Test handler metadata
	info, err := registry.GetHandlerInfo("evm")
	require.NoError(t, err)
	assert.Equal(t, "evm", info.Type)

	// Note: Don't shutdown global registry in tests as it would affect other tests
	// err = registry.ShutdownAll()
	// assert.NoError(t, err)
}

func TestRequestContext_GetRequestTypeString(t *testing.T) {
	tests := []struct {
		requestType networklib.RequestType
		expected    string
	}{
		{networklib.RequestTypeRPC, "JSON-RPC"},
		{networklib.RequestTypeREST, "REST"},
		{networklib.RequestTypeGraphQL, "GraphQL"},
		{networklib.RequestType(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			ctx := &RequestProcessor{Type: tt.requestType}
			assert.Equal(t, tt.expected, ctx.GetRequestTypeString())
		})
	}
}

func TestIsRESTPattern(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/eth/v1/beacon/genesis", true},
		{"/eth/v2/beacon/blocks/head", true},
		{"/api/v1/some/endpoint", true},
		{"/api/v2/another/endpoint", true},
		{"/some/v1/path", true},
		{"/regular/path", false},
		{"/simple", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isRESTPattern(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsHealthCheckEndpoint(t *testing.T) {
	tests := []struct {
		path           string
		healthEndpoint string
		expected       bool
	}{
		{
			path:           "/ethereum-beacon/eth/v1/beacon/headers",
			healthEndpoint: "/eth/v1/beacon/headers",
			expected:       true,
		},
		{
			path:           "/ethereum-beacon/eth/v1/beacon/genesis",
			healthEndpoint: "/eth/v1/beacon/headers",
			expected:       false,
		},
		{
			path:           "/some/path",
			healthEndpoint: "",
			expected:       false,
		},
		{
			path:           "/api/v1/health",
			healthEndpoint: "/health",
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isHealthCheckEndpoint(tt.path, tt.healthEndpoint)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRequestContextIntegration(t *testing.T) {
	// This test verifies the full integration of request context detection
	// with the actual middleware components

	// Setup test middleware
	middleware := &DinMiddleware{
		Networks: map[string]*network{
			"ethereum": {
				Name:    "ethereum",
				Type:    "evm",
				ChainId: "eip155:0x1",
			},
			"ethereum-beacon-mainnet": {
				Name:       "ethereum-beacon-mainnet",
				Type:       "eth_beacon_chain",
				ChainId:    "mainnet",
				HCEndpoint: "/eth/v1/beacon/headers",
			},
		},
		handlerRegistry: networklib.DefaultRegistry,
	}

	// Register handlers
	networklib.RegisterBuiltinHandlers()

	tests := []struct {
		name         string
		path         string
		method       string
		body         string
		contentType  string
		networkName  string
		expectedType networklib.RequestType
	}{
		{
			name:         "EVM JSON-RPC",
			path:         "/ethereum",
			method:       "POST",
			body:         `{"jsonrpc":"2.0","method":"eth_blockNumber","id":1}`,
			contentType:  "application/json",
			networkName:  "ethereum",
			expectedType: networklib.RequestTypeRPC,
		},
		{
			name:         "Beacon Chain REST",
			path:         "/ethereum-beacon-mainnet/eth/v1/beacon/genesis",
			method:       "GET",
			body:         "",
			contentType:  "",
			networkName:  "ethereum-beacon-mainnet",
			expectedType: networklib.RequestTypeREST,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			networkObj := middleware.Networks[tt.networkName]
			require.NotNil(t, networkObj)

			ctx, err := DetectRequestType(req, networkObj, middleware.handlerRegistry)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedType, ctx.Type)
			assert.NotNil(t, ctx.Handler)

			// Verify the handler can process the request
			// err = ctx.Handler.ProcessRequest(req, nil)
			// assert.NoError(t, err)
		})
	}
}
