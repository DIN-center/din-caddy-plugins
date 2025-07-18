package network

import (
	"net/http"
	"testing"
)

func TestBeaconChainHandler_GetType(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	if handler.GetType() != "eth_beacon_chain" {
		t.Errorf("Expected type 'eth_beacon_chain', got '%s'", handler.GetType())
	}
}

func TestBeaconChainHandler_GetName(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	expected := "Ethereum Beacon Chain Handler"
	if handler.GetName() != expected {
		t.Errorf("Expected name '%s', got '%s'", expected, handler.GetName())
	}
}

func TestBeaconChainHandler_GetRequestType(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	if handler.GetRequestType() != RequestTypeREST {
		t.Errorf("Expected RequestTypeREST, got %v", handler.GetRequestType())
	}
}

func TestBeaconChainHandler_GetNamespace(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	// Beacon chain uses its own namespace
	if handler.GetNamespace() != "beacon" {
		t.Errorf("Expected namespace 'beacon', got '%s'", handler.GetNamespace())
	}
}

func TestBeaconChainHandler_ValidateChainID(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	tests := []struct {
		name      string
		chainID   string
		shouldErr bool
	}{
		{
			name:      "valid mainnet chain ID",
			chainID:   "beacon:1",
			shouldErr: false,
		},
		{
			name:      "valid sepolia chain ID",
			chainID:   "beacon:11155111",
			shouldErr: false,
		},
		{
			name:      "valid goerli chain ID",
			chainID:   "beacon:5",
			shouldErr: false,
		},
		{
			name:      "invalid prefix",
			chainID:   "starknet:0x1",
			shouldErr: true,
		},
		{
			name:      "missing colon",
			chainID:   "eip1551",
			shouldErr: true,
		},
		{
			name:      "empty chain reference",
			chainID:   "eip155:",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.ValidateChainID(tt.chainID)
			if tt.shouldErr && err == nil {
				t.Errorf("ValidateChainID(%s) expected error but got none", tt.chainID)
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("ValidateChainID(%s) expected no error but got: %v", tt.chainID, err)
			}
		})
	}
}

func TestBeaconChainHandler_FormatChainID(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	tests := []struct {
		name             string
		networkReference string
		expected         string
	}{
		{
			name:             "mainnet",
			networkReference: "1",
			expected:         "beacon:1",
		},
		{
			name:             "sepolia",
			networkReference: "11155111",
			expected:         "beacon:11155111",
		},
		{
			name:             "goerli",
			networkReference: "5",
			expected:         "beacon:5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.FormatChainID(tt.networkReference)
			if result != tt.expected {
				t.Errorf("FormatChainID(%s) = %s, expected %s", tt.networkReference, result, tt.expected)
			}
		})
	}
}

func TestBeaconChainHandler_ExtractChainReference(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	tests := []struct {
		name      string
		result    interface{}
		expected  string
		shouldErr bool
	}{
		{
			name:      "valid string reference",
			result:    "1",
			expected:  "1",
			shouldErr: false,
		},
		{
			name:      "valid hex string",
			result:    "0x1",
			expected:  "0x1",
			shouldErr: false,
		},
		{
			name:      "invalid type",
			result:    42,
			expected:  "",
			shouldErr: true,
		},
		{
			name:      "nil result",
			result:    nil,
			expected:  "",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler.ExtractChainReference(tt.result)
			if tt.shouldErr && err == nil {
				t.Errorf("ExtractChainReference(%v) expected error but got none", tt.result)
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("ExtractChainReference(%v) expected no error but got: %v", tt.result, err)
			}
			if result != tt.expected {
				t.Errorf("ExtractChainReference(%v) = %s, expected %s", tt.result, result, tt.expected)
			}
		})
	}
}

func TestBeaconChainHandler_FormatBlockHeight(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	tests := []struct {
		name     string
		blockNum int64
		expected string
	}{
		{
			name:     "slot 0",
			blockNum: 0,
			expected: "0",
		},
		{
			name:     "slot 1",
			blockNum: 1,
			expected: "1",
		},
		{
			name:     "slot 12345",
			blockNum: 12345,
			expected: "12345",
		},
		{
			name:     "large slot number",
			blockNum: 7300000,
			expected: "7300000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.FormatBlockHeight(tt.blockNum)
			if result != tt.expected {
				t.Errorf("FormatBlockHeight(%d) = %s, expected %s", tt.blockNum, result, tt.expected)
			}
		})
	}
}

func TestBeaconChainHandler_CreateBlockRequest(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	// Beacon chain doesn't use JSON-RPC, so this should return an error or not be supported
	_, err := handler.CreateBlockRequest("getBlock", 12345, false)
	if err == nil {
		t.Error("Expected CreateBlockRequest to return an error for Beacon chain (REST API)")
	}
}

func TestBeaconChainHandler_SupportsArchiveMode(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	// Beacon chain typically doesn't support archive mode in the same way as EVM
	if handler.SupportsArchiveMode() {
		t.Error("Expected Beacon chain handler to not support archive mode")
	}
}

func TestBeaconChainHandler_GetArchiveMethod(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	// Should return empty string since archive mode is not supported
	if handler.GetArchiveMethod() != "" {
		t.Errorf("Expected empty archive method for Beacon chain, got '%s'", handler.GetArchiveMethod())
	}
}

func TestBeaconChainHandler_CreateArchivePayload(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	// Should return an error since archive mode is not supported
	_, err := handler.CreateArchivePayload("someMethod", "12345")
	if err == nil {
		t.Error("Expected CreateArchivePayload to return error for Beacon chain")
	}
}

func TestBeaconChainHandler_SupportsGetBlockByNumber(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	if handler.SupportsGetBlockByNumber() {
		t.Error("Expected Beacon chain handler to not support getBlockByNumber")
	}
}

func TestBeaconChainHandler_GetSupportedMethods(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	methods := handler.GetSupportedMethods()

	// Beacon chain uses REST endpoints instead of JSON-RPC methods
	expectedMethods := []string{
		"/eth/v1/beacon/headers/head",
		"/eth/v1/beacon/blocks/head",
		"/eth/v1/beacon/states/head/validators",
		"/eth/v1/beacon/genesis",
		"/eth/v1/node/version",
		"/eth/v1/node/health",
		"/eth/v1/config/fork_schedule",
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

func TestBeaconChainHandler_GetHealthCheckMethod(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	expected := "/eth/v1/node/health"
	if handler.GetHealthCheckMethod() != expected {
		t.Errorf("Expected health check method '%s', got '%s'", expected, handler.GetHealthCheckMethod())
	}
}

func TestBeaconChainHandler_GetChainIDMethod(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	expected := "/eth/v1/config/spec"
	if handler.GetChainIDMethod() != expected {
		t.Errorf("Expected chain ID method '%s', got '%s'", expected, handler.GetChainIDMethod())
	}
}

func TestBeaconChainHandler_CreateHealthCheckPayload(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	// Beacon chain uses REST API, so payload should be empty or nil
	payload, err := handler.CreateHealthCheckPayload("/eth/v1/beacon/headers/head")
	if err != nil {
		t.Errorf("CreateHealthCheckPayload() error = %v", err)
		return
	}

	// For REST API, payload should be empty
	if len(payload) != 0 {
		t.Errorf("Expected empty payload for REST API, got %s", string(payload))
	}
}

func TestBeaconChainHandler_ExtractBlockHash(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	tests := []struct {
		name      string
		blockData interface{}
		expected  string
	}{
		{
			name: "valid beacon block data",
			blockData: BeaconHeadResponse{
				Data: struct {
					Root   string `json:"root"`
					Header struct {
						Message struct {
							Slot          string `json:"slot"`
							ProposerIndex string `json:"proposer_index"`
							ParentRoot    string `json:"parent_root"`
							StateRoot     string `json:"state_root"`
						} `json:"message"`
					} `json:"header"`
				}{
					Root: "0x1234567890abcdef",
				},
			},
			expected: "0x1234567890abcdef",
		},
		{
			name:      "invalid block data",
			blockData: "invalid",
			expected:  "",
		},
		{
			name:      "nil block data",
			blockData: nil,
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.ExtractBlockHash(tt.blockData)
			if result != tt.expected {
				t.Errorf("ExtractBlockHash(%v) = %s, expected %s", tt.blockData, result, tt.expected)
			}
		})
	}
}

func TestBeaconChainHandler_ExtractBlockNumber(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	tests := []struct {
		name      string
		response  string
		expected  int64
		shouldErr bool
	}{
		{
			name: "valid beacon response",
			response: `{
				"data": {
					"header": {
						"message": {
							"slot": "7300000"
						}
					}
				}
			}`,
			expected:  7300000,
			shouldErr: false,
		},
		{
			name:      "invalid JSON",
			response:  `invalid json`,
			expected:  0,
			shouldErr: true,
		},
		{
			name: "missing slot",
			response: `{
				"data": {
					"header": {
						"message": {}
					}
				}
			}`,
			expected:  0,
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handler.ExtractBlockNumber([]byte(tt.response))
			if tt.shouldErr && err == nil {
				t.Errorf("ExtractBlockNumber(%s) expected error but got none", tt.name)
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("ExtractBlockNumber(%s) expected no error but got: %v", tt.name, err)
			}
			if result != tt.expected {
				t.Errorf("ExtractBlockNumber(%s) = %d, expected %d", tt.name, result, tt.expected)
			}
		})
	}
}

func TestBeaconChainHandler_ValidateRequest(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	tests := []struct {
		name        string
		method      string
		path        string
		expectError bool
	}{
		{
			name:        "valid GET request",
			method:      "GET",
			path:        "/eth/v1/beacon/headers/head",
			expectError: false,
		},
		{
			name:        "valid POST request",
			method:      "POST",
			path:        "/eth/v1/beacon/blocks",
			expectError: false,
		},
		{
			name:        "invalid method",
			method:      "DELETE",
			path:        "/eth/v1/beacon/headers/head",
			expectError: true,
		},
		{
			name:        "invalid path",
			method:      "GET",
			path:        "/invalid/path",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(tt.method, tt.path, nil)

			err := handler.ValidateRequest(req)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for test '%s', but got none", tt.name)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error for test '%s', but got: %v", tt.name, err)
			}
		})
	}
}
