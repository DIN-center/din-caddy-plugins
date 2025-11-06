package network

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestEVMHandler_GetType(t *testing.T) {
	handler := NewEVMHandler(&NetworkConfig{})

	if handler.GetType() != "evm" {
		t.Errorf("Expected type 'evm', got '%s'", handler.GetType())
	}
}

func TestEVMHandler_GetName(t *testing.T) {
	handler := NewEVMHandler(&NetworkConfig{})

	expected := "EVM JSON-RPC Handler"
	if handler.GetName() != expected {
		t.Errorf("Expected name '%s', got '%s'", expected, handler.GetName())
	}
}

func TestEVMHandler_GetRequestType(t *testing.T) {
	handler := NewEVMHandler(&NetworkConfig{})

	if handler.GetRequestType() != RequestTypeRPC {
		t.Errorf("Expected RequestTypeRPC, got %v", handler.GetRequestType())
	}
}

func TestEVMHandler_GetNamespace(t *testing.T) {
	handler := NewEVMHandler(&NetworkConfig{})

	if handler.GetNamespace() != "eip155" {
		t.Errorf("Expected namespace 'eip155', got '%s'", handler.GetNamespace())
	}
}

func TestEVMHandler_ValidateChainID(t *testing.T) {
	handler := NewEVMHandler(&NetworkConfig{})

	tests := []struct {
		name      string
		chainID   string
		shouldErr bool
	}{
		{
			name:      "valid mainnet chain ID decimal",
			chainID:   "1",
			shouldErr: false,
		},
		{
			name:      "valid mainnet chain ID hex",
			chainID:   "0x1",
			shouldErr: false,
		},
		{
			name:      "valid polygon chain ID",
			chainID:   "0x89",
			shouldErr: false,
		},
		{
			name:      "valid arbitrum chain ID",
			chainID:   "0xa4b1",
			shouldErr: false,
		},
		{
			name:      "valid - CAIP-2 format supported for backwards compatibility",
			chainID:   "eip155:1",
			shouldErr: false,
		},
		{
			name:      "valid - CAIP-2 format with hex",
			chainID:   "eip155:0x1",
			shouldErr: false,
		},
		{
			name:      "invalid - contains colon with different prefix",
			chainID:   "starknet:0x1",
			shouldErr: true,
		},
		{
			name:      "invalid - wrong CAIP-2 prefix for EVM",
			chainID:   "solana:0x1",
			shouldErr: true,
		},
		{
			name:      "empty chain ID",
			chainID:   "",
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

func TestEVMHandler_ExtractChainReference(t *testing.T) {
	handler := NewEVMHandler(&NetworkConfig{})

	tests := []struct {
		name      string
		result    interface{}
		expected  string
		shouldErr bool
	}{
		{
			name:      "valid string reference",
			result:    "0x1",
			expected:  "0x1",
			shouldErr: false,
		},
		{
			name:      "valid decimal string",
			result:    "137",
			expected:  "137",
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

func TestEVMHandler_FormatBlockHeight(t *testing.T) {
	handler := NewEVMHandler(&NetworkConfig{})

	tests := []struct {
		name     string
		blockNum int64
		expected string
	}{
		{
			name:     "zero block",
			blockNum: 0,
			expected: "0x0",
		},
		{
			name:     "block 1",
			blockNum: 1,
			expected: "0x1",
		},
		{
			name:     "block 255",
			blockNum: 255,
			expected: "0xff",
		},
		{
			name:     "block 65536",
			blockNum: 65536,
			expected: "0x10000",
		},
		{
			name:     "large block number",
			blockNum: 18500000,
			expected: "0x11a49a0",
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

func TestEVMHandler_CreateBlockRequest(t *testing.T) {
	handler := NewEVMHandler(&NetworkConfig{})

	tests := []struct {
		name                string
		method              string
		blockNum            int64
		includeTransactions bool
		expectedMethod      string
		expectedBlockParam  string
		expectedIncludeTxs  bool
	}{
		{
			name:                "getBlockByNumber without transactions",
			method:              "eth_getBlockByNumber",
			blockNum:            12345,
			includeTransactions: false,
			expectedMethod:      "eth_getBlockByNumber",
			expectedBlockParam:  "0x3039",
			expectedIncludeTxs:  false,
		},
		{
			name:                "getBlockByNumber with transactions",
			method:              "eth_getBlockByNumber",
			blockNum:            65536,
			includeTransactions: true,
			expectedMethod:      "eth_getBlockByNumber",
			expectedBlockParam:  "0x10000",
			expectedIncludeTxs:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := handler.CreateBlockRequest(tt.method, tt.blockNum, tt.includeTransactions)
			if err != nil {
				t.Errorf("CreateBlockRequest() error = %v", err)
				return
			}

			// Parse the JSON payload to verify structure
			var req map[string]interface{}
			err = json.Unmarshal(payload, &req)
			if err != nil {
				t.Errorf("Failed to unmarshal request payload: %v", err)
				return
			}

			// Check JSONRPC version
			if req["jsonrpc"] != "2.0" {
				t.Errorf("Expected jsonrpc version '2.0', got %v", req["jsonrpc"])
			}

			// Check method
			if req["method"] != tt.expectedMethod {
				t.Errorf("Expected method '%s', got %v", tt.expectedMethod, req["method"])
			}

			// Check parameters
			params, ok := req["params"].([]interface{})
			if !ok || len(params) != 2 {
				t.Errorf("Expected params to be array of length 2, got %v", req["params"])
				return
			}

			// Check block parameter
			if params[0] != tt.expectedBlockParam {
				t.Errorf("Expected block param '%s', got %v", tt.expectedBlockParam, params[0])
			}

			// Check include transactions parameter
			if params[1] != tt.expectedIncludeTxs {
				t.Errorf("Expected include transactions %v, got %v", tt.expectedIncludeTxs, params[1])
			}
		})
	}
}

func TestEVMHandler_SupportsArchiveMode(t *testing.T) {
	handler := NewEVMHandler(&NetworkConfig{})

	if !handler.SupportsArchiveMode() {
		t.Error("Expected EVM handler to support archive mode")
	}
}

func TestEVMHandler_GetArchiveMethod(t *testing.T) {
	handler := NewEVMHandler(&NetworkConfig{})

	expected := "eth_getBalance"
	if handler.GetArchiveMethod() != expected {
		t.Errorf("Expected archive method '%s', got '%s'", expected, handler.GetArchiveMethod())
	}
}

func TestEVMHandler_CreateArchivePayload(t *testing.T) {
	handler := NewEVMHandler(&NetworkConfig{})

	payload, err := handler.CreateArchivePayload("eth_getBalance", "0x3039")
	if err != nil {
		t.Errorf("CreateArchivePayload() error = %v", err)
		return
	}

	// Parse the JSON payload to verify structure
	var req map[string]interface{}
	err = json.Unmarshal(payload, &req)
	if err != nil {
		t.Errorf("Failed to unmarshal archive payload: %v", err)
		return
	}

	// Check JSONRPC version
	if req["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc version '2.0', got %v", req["jsonrpc"])
	}

	// Check method
	if req["method"] != "eth_getBalance" {
		t.Errorf("Expected method 'eth_getBalance', got %v", req["method"])
	}

	// Check parameters
	params, ok := req["params"].([]interface{})
	if !ok || len(params) != 2 {
		t.Errorf("Expected params to be array of length 2, got %v", req["params"])
		return
	}

	// Check call object
	balanceAddress, ok := params[0].(string)
	if !ok {
		t.Errorf("Expected first param to be string, got %v", params[0])
		return
	}

	if balanceAddress != "0x0000000000000000000000000000000000000000" {
		t.Errorf("Expected call input '0x0000000000000000000000000000000000000000', got %v", balanceAddress)
	}

	// Check block parameter
	if params[1] != "0x3039" {
		t.Errorf("Expected block param '0x3039', got %v", params[1])
	}
}

func TestEVMHandler_SupportsGetBlockByNumber(t *testing.T) {
	handler := NewEVMHandler(&NetworkConfig{})

	if !handler.SupportsGetBlockByNumber() {
		t.Error("Expected EVM handler to support getBlockByNumber")
	}
}

func TestEVMHandler_GetSupportedMethods(t *testing.T) {
	handler := NewEVMHandler(&NetworkConfig{})

	methods := handler.GetSupportedMethods()

	expectedMethods := []string{
		"eth_blockNumber",
		"eth_chainId",
		"eth_call",
		"eth_getBlockByNumber",
		"eth_getBlockByHash",
		"eth_getBalance",
		"eth_getTransactionByHash",
		"eth_getTransactionReceipt",
		"eth_sendRawTransaction",
		"eth_gasPrice",
		"eth_estimateGas",
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

func TestEVMHandler_GetHealthCheckMethod(t *testing.T) {
	handler := NewEVMHandler(&NetworkConfig{})

	expected := "eth_blockNumber"
	if handler.GetHealthCheckMethod() != expected {
		t.Errorf("Expected health check method '%s', got '%s'", expected, handler.GetHealthCheckMethod())
	}
}

func TestEVMHandler_GetChainIDMethod(t *testing.T) {
	handler := NewEVMHandler(&NetworkConfig{})

	expected := "eth_chainId"
	if handler.GetChainIDMethod() != expected {
		t.Errorf("Expected chain ID method '%s', got '%s'", expected, handler.GetChainIDMethod())
	}
}

func TestEVMHandler_CreateHealthCheckPayload(t *testing.T) {
	handler := NewEVMHandler(&NetworkConfig{})

	payload, err := handler.CreateHealthCheckPayload("eth_blockNumber")
	if err != nil {
		t.Errorf("CreateHealthCheckPayload() error = %v", err)
		return
	}

	// Parse the JSON payload to verify structure
	var req map[string]interface{}
	err = json.Unmarshal(payload, &req)
	if err != nil {
		t.Errorf("Failed to unmarshal health check payload: %v", err)
		return
	}

	// Check JSONRPC version
	if req["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc version '2.0', got %v", req["jsonrpc"])
	}

	// Check method
	if req["method"] != "eth_blockNumber" {
		t.Errorf("Expected method 'eth_blockNumber', got %v", req["method"])
	}

	// Check ID exists
	if req["id"] == nil {
		t.Error("Expected id field to be present")
	}
}

func TestEVMHandler_ValidateRequest(t *testing.T) {
	handler := NewEVMHandler(&NetworkConfig{})

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
			req, _ := http.NewRequest(tt.method, "/test", nil)
			req.Header.Set("Content-Type", tt.contentType)

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
