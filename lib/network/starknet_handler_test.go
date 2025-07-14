package network

import (
	"net/http"
	"testing"
)

func TestStarknetHandler_GetType(t *testing.T) {
	handler := NewStarknetHandler(&NetworkConfig{})

	if handler.GetType() != "starknet" {
		t.Errorf("Expected type 'starknet', got '%s'", handler.GetType())
	}
}

func TestStarknetHandler_GetName(t *testing.T) {
	handler := NewStarknetHandler(&NetworkConfig{})

	expected := "Starknet JSON-RPC Handler"
	if handler.GetName() != expected {
		t.Errorf("Expected name '%s', got '%s'", expected, handler.GetName())
	}
}

func TestStarknetHandler_GetRequestType(t *testing.T) {
	handler := NewStarknetHandler(&NetworkConfig{})

	if handler.GetRequestType() != RequestTypeRPC {
		t.Errorf("Expected RequestTypeRPC, got %v", handler.GetRequestType())
	}
}

func TestStarknetHandler_ValidateChainID(t *testing.T) {
	handler := NewStarknetHandler(&NetworkConfig{})

	tests := []struct {
		name     string
		chainID  string
		expected bool
	}{
		{
			name:     "valid mainnet chain ID",
			chainID:  "starknet:0x534e5f4d41494e",
			expected: true,
		},
		{
			name:     "valid sepolia chain ID",
			chainID:  "starknet:0x534e5f5345504f4c4941",
			expected: true,
		},
		{
			name:     "invalid prefix",
			chainID:  "ethereum:0x1",
			expected: false,
		},
		{
			name:     "missing 0x prefix",
			chainID:  "starknet:534e5f4d41494e",
			expected: false,
		},
		{
			name:     "empty hex part",
			chainID:  "starknet:0x",
			expected: false,
		},
		{
			name:     "invalid hex characters",
			chainID:  "starknet:0xghi",
			expected: false,
		},
		{
			name:     "uppercase hex",
			chainID:  "starknet:0x534E5F4D41494E",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.ValidateChainID(tt.chainID)
			if result != tt.expected {
				t.Errorf("ValidateChainID(%s) = %v, expected %v", tt.chainID, result, tt.expected)
			}
		})
	}
}

func TestStarknetHandler_GetDefaultMethods(t *testing.T) {
	handler := NewStarknetHandler(&NetworkConfig{})

	methods := handler.GetDefaultMethods()

	expectedMethods := []string{
		"starknet_blockNumber",
		"starknet_chainId",
		"starknet_call",
		"starknet_getBlockByNumber",
		"starknet_getBlockByHash",
		"starknet_getTransactionByHash",
		"starknet_getTransactionReceipt",
		"starknet_getBalance",
		"starknet_syncing",
		"starknet_sendTransaction",
	}

	if len(methods) != len(expectedMethods) {
		t.Errorf("Expected %d methods, got %d", len(expectedMethods), len(methods))
	}

	for i, expected := range expectedMethods {
		if i >= len(methods) || methods[i] != expected {
			t.Errorf("Expected method %d to be '%s', got '%s'", i, expected, methods[i])
		}
	}
}

func TestStarknetHandler_GetHealthCheckMethod(t *testing.T) {
	handler := NewStarknetHandler(&NetworkConfig{})

	if handler.GetHealthCheckMethod() != "starknet_blockNumber" {
		t.Errorf("Expected health check method 'starknet_blockNumber', got '%s'", handler.GetHealthCheckMethod())
	}
}

func TestStarknetHandler_GetChainIDMethod(t *testing.T) {
	handler := NewStarknetHandler(&NetworkConfig{})

	if handler.GetChainIDMethod() != "starknet_chainId" {
		t.Errorf("Expected chain ID method 'starknet_chainId', got '%s'", handler.GetChainIDMethod())
	}
}

func TestStarknetHandler_GetCallContractMethod(t *testing.T) {
	handler := NewStarknetHandler(&NetworkConfig{})

	if handler.GetCallContractMethod() != "starknet_call" {
		t.Errorf("Expected call contract method 'starknet_call', got '%s'", handler.GetCallContractMethod())
	}
}

func TestStarknetHandler_ValidateRequest(t *testing.T) {
	handler := NewStarknetHandler(&NetworkConfig{})

	tests := []struct {
		name        string
		method      string
		contentType string
		expectError bool
	}{
		{
			name:        "valid request",
			method:      "POST",
			contentType: "application/json",
			expectError: false,
		},
		{
			name:        "invalid method",
			method:      "GET",
			contentType: "application/json",
			expectError: true,
		},
		{
			name:        "invalid content type",
			method:      "POST",
			contentType: "text/plain",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(tt.method, "/", nil)
			req.Header.Set("Content-Type", tt.contentType)

			err := handler.ValidateRequest(req)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// MockProvider implements the Provider interface for testing
type MockStarknetProvider struct {
	url      string
	headers  map[string]string
	path     string
	host     string
	priority int
}

func (m *MockStarknetProvider) GetURL() string {
	return m.url
}

func (m *MockStarknetProvider) GetHeaders() map[string]string {
	return m.headers
}

func (m *MockStarknetProvider) GetPath() string {
	return m.path
}

func (m *MockStarknetProvider) GetHost() string {
	return m.host
}

func (m *MockStarknetProvider) GetPriority() int {
	return m.priority
}

func TestStarknetHandler_TranslatePath(t *testing.T) {
	handler := NewStarknetHandler(&NetworkConfig{})

	tests := []struct {
		name         string
		gatewayPath  string
		provider     Provider
		expectedPath string
	}{
		{
			name:         "nil provider",
			gatewayPath:  "/starknet-mainnet",
			provider:     nil,
			expectedPath: "/starknet-mainnet",
		},
		{
			name:        "provider with path",
			gatewayPath: "/starknet-mainnet",
			provider: &MockStarknetProvider{
				path: "/rpc/v0_7",
			},
			expectedPath: "/rpc/v0_7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler.TranslatePath(tt.gatewayPath, tt.provider)
			if err != nil {
				t.Errorf("TranslatePath() error = %v", err)
				return
			}
			if result != tt.expectedPath {
				t.Errorf("TranslatePath() = %v, expected %v", result, tt.expectedPath)
			}
		})
	}
}
