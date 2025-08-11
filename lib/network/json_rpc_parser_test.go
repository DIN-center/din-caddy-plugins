package network

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestParseHexBlockNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    json.RawMessage
		expected int64
		wantErr  bool
	}{
		{
			name:     "valid hex number",
			input:    json.RawMessage(`"0x10"`),
			expected: 16,
			wantErr:  false,
		},
		{
			name:     "large hex number",
			input:    json.RawMessage(`"0x1234567"`),
			expected: 19088743,
			wantErr:  false,
		},
		{
			name:     "zero hex",
			input:    json.RawMessage(`"0x0"`),
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "invalid format - missing 0x",
			input:    json.RawMessage(`"123"`),
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "empty string",
			input:    json.RawMessage(`""`),
			expected: 0,
			wantErr:  true,
		},
		{
			name:     "not a string",
			input:    json.RawMessage(`123`),
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseHexBlockNumber(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseHexBlockNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("ParseHexBlockNumber() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseNumericBlockNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    json.RawMessage
		expected int64
		wantErr  bool
	}{
		{
			name:     "valid integer",
			input:    json.RawMessage(`123`),
			expected: 123,
			wantErr:  false,
		},
		{
			name:     "valid float",
			input:    json.RawMessage(`123.0`),
			expected: 123,
			wantErr:  false,
		},
		{
			name:     "zero",
			input:    json.RawMessage(`0`),
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "not a number",
			input:    json.RawMessage(`"abc"`),
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseNumericBlockNumber(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseNumericBlockNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("ParseNumericBlockNumber() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseJSONRPCPayload(t *testing.T) {
	tests := []struct {
		name           string
		payload        []byte
		expectedMethod string
		expectedParams json.RawMessage
		wantErr        bool
	}{
		{
			name:           "valid payload with params",
			payload:        []byte(`{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`),
			expectedMethod: "eth_blockNumber",
			expectedParams: json.RawMessage(`[]`),
			wantErr:        false,
		},
		{
			name:           "valid payload with complex params",
			payload:        []byte(`{"jsonrpc":"2.0","method":"eth_call","params":[{"to":"0x123"},"latest"],"id":1}`),
			expectedMethod: "eth_call",
			expectedParams: json.RawMessage(`[{"to":"0x123"},"latest"]`),
			wantErr:        false,
		},
		{
			name:           "invalid JSON",
			payload:        []byte(`{invalid json}`),
			expectedMethod: "",
			expectedParams: nil,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method, params, err := ParseJSONRPCPayload(tt.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseJSONRPCPayload() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if method != tt.expectedMethod {
				t.Errorf("ParseJSONRPCPayload() method = %v, want %v", method, tt.expectedMethod)
			}
			if !tt.wantErr && string(params) != string(tt.expectedParams) {
				t.Errorf("ParseJSONRPCPayload() params = %v, want %v", string(params), string(tt.expectedParams))
			}
		})
	}
}

func TestCreateJSONRPCRequestContext(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		method  string
	}{
		{
			name:    "basic request context",
			payload: []byte(`{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`),
			method:  "eth_blockNumber",
		},
		{
			name:    "request with params",
			payload: []byte(`{"jsonrpc":"2.0","method":"eth_call","params":[{"to":"0x123"}],"id":1}`),
			method:  "eth_call",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context, err := CreateJSONRPCRequestContext(tt.payload, tt.method)
			if err != nil {
				t.Errorf("CreateJSONRPCRequestContext() error = %v", err)
				return
			}

			// Verify context contains required fields
			if context["jsonrpc"] != "2.0" {
				t.Errorf("Expected jsonrpc = 2.0, got %v", context["jsonrpc"])
			}
			if context["method"] != tt.method {
				t.Errorf("Expected method = %s, got %v", tt.method, context["method"])
			}
			if _, ok := context["id"]; !ok {
				t.Error("Expected context to contain id field")
			}
			if _, ok := context["params"]; !ok {
				t.Error("Expected context to contain params field")
			}
		})
	}
}

// TestConfigureJSONRPCRequestPath_EdgeCases tests all edge cases for JSON-RPC path configuration
func TestConfigureJSONRPCRequestPath_EdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		initialPath     string
		providerPath    string
		expectedPath    string
		expectedRawPath string
		description     string
	}{
		// Root path cases
		{
			name:            "Empty provider path - should set to root",
			initialPath:     "/mantle-mainnet",
			providerPath:    "",
			expectedPath:    "/",
			expectedRawPath: "",
			description:     "Empty string provider path should be treated as root",
		},
		{
			name:            "Slash provider path - should set to root",
			initialPath:     "/mantle-mainnet",
			providerPath:    "/",
			expectedPath:    "/",
			expectedRawPath: "",
			description:     "Single slash provider path should be treated as root",
		},

		// Specific path cases
		{
			name:            "Provider with specific path",
			initialPath:     "/mantle-mainnet",
			providerPath:    "/v1/rpc",
			expectedPath:    "/v1/rpc",
			expectedRawPath: "",
			description:     "Provider with specific path should use that path",
		},
		{
			name:            "Provider with nested path",
			initialPath:     "/polygon-mainnet",
			providerPath:    "/api/v2/json-rpc",
			expectedPath:    "/api/v2/json-rpc",
			expectedRawPath: "",
			description:     "Provider with nested path should preserve full path",
		},

		// Edge cases with trailing slashes
		{
			name:            "Provider path with trailing slash",
			initialPath:     "/eth-mainnet",
			providerPath:    "/rpc/",
			expectedPath:    "/rpc/",
			expectedRawPath: "",
			description:     "Trailing slashes should be preserved",
		},

		// Cases with query parameters
		{
			name:            "Initial path with query params - empty provider",
			initialPath:     "/mantle-mainnet?foo=bar",
			providerPath:    "",
			expectedPath:    "/",
			expectedRawPath: "",
			description:     "Query params should be preserved, path set to root",
		},
		{
			name:            "Initial path with query params - specific provider",
			initialPath:     "/mantle-mainnet?foo=bar",
			providerPath:    "/v1/rpc",
			expectedPath:    "/v1/rpc",
			expectedRawPath: "",
			description:     "Query params should be preserved, path replaced",
		},

		// Special characters in paths
		{
			name:            "Provider path with encoded characters",
			initialPath:     "/network",
			providerPath:    "/path%20with%20spaces",
			expectedPath:    "/path%20with%20spaces",
			expectedRawPath: "",
			description:     "Encoded characters should be preserved",
		},

		// Double slash cases
		{
			name:            "Provider path with double slashes",
			initialPath:     "/network",
			providerPath:    "//double//slash",
			expectedPath:    "//double//slash",
			expectedRawPath: "",
			description:     "Double slashes should be preserved as-is",
		},

		// No initial path cases
		{
			name:            "No initial path - empty provider",
			initialPath:     "",
			providerPath:    "",
			expectedPath:    "/",
			expectedRawPath: "",
			description:     "Both empty should result in root",
		},
		{
			name:            "No initial path - specific provider",
			initialPath:     "",
			providerPath:    "/api/v1",
			expectedPath:    "/api/v1",
			expectedRawPath: "",
			description:     "Empty initial with specific provider should use provider path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request with initial path
			req, err := http.NewRequest("POST", "http://example.com"+tt.initialPath, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			// Apply the configuration
			ConfigureJSONRPCRequestPath(req, tt.providerPath)

			// Check results
			if req.URL.Path != tt.expectedPath {
				t.Errorf("Path mismatch for %s:\n  Description: %s\n  Got: %q\n  Want: %q",
					tt.name, tt.description, req.URL.Path, tt.expectedPath)
			}
			if req.URL.RawPath != tt.expectedRawPath {
				t.Errorf("RawPath mismatch for %s:\n  Description: %s\n  Got: %q\n  Want: %q",
					tt.name, tt.description, req.URL.RawPath, tt.expectedRawPath)
			}
		})
	}
}

// TestConfigureRESTRequestPath_EdgeCases tests all edge cases for REST API path configuration
func TestConfigureRESTRequestPath_EdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		initialPath     string
		providerPath    string
		networkName     string
		expectedPath    string
		expectedRawPath string
		description     string
	}{
		// Network stripping cases
		{
			name:            "Strip network prefix - basic",
			initialPath:     "/eth-beacon-mainnet/eth/v1/beacon/genesis",
			providerPath:    "",
			networkName:     "eth-beacon-mainnet",
			expectedPath:    "/eth/v1/beacon/genesis",
			expectedRawPath: "",
			description:     "Should strip network prefix from path",
		},
		{
			name:            "Strip network prefix with provider base",
			initialPath:     "/eth-beacon-mainnet/eth/v1/beacon/genesis",
			providerPath:    "/beacon-api",
			networkName:     "eth-beacon-mainnet",
			expectedPath:    "/beacon-api/eth/v1/beacon/genesis",
			expectedRawPath: "",
			description:     "Should strip network and prepend provider base",
		},

		// No network prefix cases
		{
			name:            "No network prefix in path",
			initialPath:     "/api/v1/data",
			providerPath:    "",
			networkName:     "bitcoin-mainnet",
			expectedPath:    "/api/v1/data",
			expectedRawPath: "",
			description:     "Path without network prefix should remain unchanged",
		},

		// Root path cases
		{
			name:            "Provider with root path",
			initialPath:     "/bitcoin-mainnet/api/tx",
			providerPath:    "/",
			networkName:     "bitcoin-mainnet",
			expectedPath:    "/api/tx",
			expectedRawPath: "",
			description:     "Root provider path should only strip network",
		},

		// Empty provider path
		{
			name:            "Empty provider path",
			initialPath:     "/bitcoin-mainnet/api/tx",
			providerPath:    "",
			networkName:     "bitcoin-mainnet",
			expectedPath:    "/api/tx",
			expectedRawPath: "",
			description:     "Empty provider path should only strip network",
		},

		// Complex path joining
		{
			name:            "Provider path with trailing slash",
			initialPath:     "/network/api/v1",
			providerPath:    "/base/",
			networkName:     "network",
			expectedPath:    "/base/api/v1",
			expectedRawPath: "",
			description:     "Should handle trailing slashes correctly",
		},
		{
			name:            "Both paths with leading slash",
			initialPath:     "/network/api",
			providerPath:    "/base",
			networkName:     "network",
			expectedPath:    "/base/api",
			expectedRawPath: "",
			description:     "Should join paths correctly with slashes",
		},

		// Edge cases with special characters
		{
			name:            "Path with query parameters",
			initialPath:     "/network/api?param=value",
			providerPath:    "/base",
			networkName:     "network",
			expectedPath:    "/base/api",
			expectedRawPath: "",
			description:     "Query parameters are handled by URL, not path",
		},

		// Network name not at start
		{
			name:            "Network name appears later in path",
			initialPath:     "/api/network/data",
			providerPath:    "/base",
			networkName:     "network",
			expectedPath:    "/base/api/network/data",
			expectedRawPath: "",
			description:     "Should not strip network name if not at start",
		},

		// Single segment paths
		{
			name:            "Single segment matching network",
			initialPath:     "/network",
			providerPath:    "/api",
			networkName:     "network",
			expectedPath:    "/api/network",
			expectedRawPath: "",
			description:     "Single segment matching network should NOT be stripped (could be legitimate endpoint)",
		},
		{
			name:            "Single segment not matching network",
			initialPath:     "/other",
			providerPath:    "/api",
			networkName:     "network",
			expectedPath:    "/api/other",
			expectedRawPath: "",
			description:     "Single segment not matching should be preserved",
		},

		// Empty and root cases
		{
			name:            "Empty initial path",
			initialPath:     "",
			providerPath:    "/api",
			networkName:     "network",
			expectedPath:    "/api",
			expectedRawPath: "",
			description:     "Empty initial path should use provider path",
		},
		{
			name:            "Just slash initial path",
			initialPath:     "/",
			providerPath:    "/api",
			networkName:     "network",
			expectedPath:    "/api/",
			expectedRawPath: "",
			description:     "Root initial path should append to provider path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request with initial path
			req, err := http.NewRequest("GET", "http://example.com"+tt.initialPath, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			// Apply the configuration
			ConfigureRESTRequestPath(req, tt.providerPath, tt.networkName)

			// Check results
			if req.URL.Path != tt.expectedPath {
				t.Errorf("Path mismatch for %s:\n  Description: %s\n  Got: %q\n  Want: %q",
					tt.name, tt.description, req.URL.Path, tt.expectedPath)
			}
			if req.URL.RawPath != tt.expectedRawPath {
				t.Errorf("RawPath mismatch for %s:\n  Description: %s\n  Got: %q\n  Want: %q",
					tt.name, tt.description, req.URL.RawPath, tt.expectedRawPath)
			}
		})
	}
}
