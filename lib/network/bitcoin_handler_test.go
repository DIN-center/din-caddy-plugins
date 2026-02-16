package network

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBitcoinHandler_GetType(t *testing.T) {
	handler := NewBitcoinHandler(&NetworkConfig{})
	assert.Equal(t, BitcoinHandlerType, handler.GetType())
}

func TestBitcoinHandler_GetName(t *testing.T) {
	handler := NewBitcoinHandler(&NetworkConfig{})

	expected := "Bitcoin JSON-RPC Handler"
	if handler.GetName() != expected {
		t.Errorf("Expected name '%s', got '%s'", expected, handler.GetName())
	}
}

func TestBitcoinHandler_GetRequestType(t *testing.T) {
	handler := NewBitcoinHandler(&NetworkConfig{})

	if handler.GetRequestType() != RequestTypeRPC {
		t.Errorf("Expected RequestTypeRPC, got %v", handler.GetRequestType())
	}
}

func TestBitcoinHandler_ValidateChainID(t *testing.T) {
	handler := NewBitcoinHandler(&NetworkConfig{})

	tests := []struct {
		name    string
		chainID string
		valid   bool
	}{
		{
			name:    "valid_mainnet",
			chainID: "main",
			valid:   true,
		},
		{
			name:    "valid_testnet",
			chainID: "test",
			valid:   true,
		},
		{
			name:    "valid_regtest",
			chainID: "regtest",
			valid:   true,
		},
		{
			name:    "valid_signet",
			chainID: "signet",
			valid:   true,
		},
		{
			name:    "invalid - contains colon (old CAIP-2 format)",
			chainID: "bitcoin:main",
			valid:   false,
		},
		{
			name:    "invalid - unknown chain",
			chainID: "unknown",
			valid:   false,
		},
		{
			name:    "empty chain ID",
			chainID: "",
			valid:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.ValidateChainID(tt.chainID)
			if tt.valid && err != nil {
				t.Errorf("Expected valid chain ID '%s', but got error: %v", tt.chainID, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("Expected invalid chain ID '%s', but no error returned", tt.chainID)
			}
		})
	}
}

func TestBitcoinHandler_ValidateRequest(t *testing.T) {
	handler := NewBitcoinHandler(&NetworkConfig{})

	tests := []struct {
		name        string
		method      string
		contentType string
		expectError bool
	}{
		{
			name:        "valid_request",
			method:      "POST",
			contentType: "application/json",
			expectError: false,
		},
		{
			name:        "valid_request_with_charset",
			method:      "POST",
			contentType: "application/json; charset=utf-8",
			expectError: false,
		},
		{
			name:        "invalid_method_GET",
			method:      "GET",
			contentType: "application/json",
			expectError: true,
		},
		{
			name:        "invalid_content_type",
			method:      "POST",
			contentType: "text/plain",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(tt.method, "http://example.com", nil)
			req.Header.Set("Content-Type", tt.contentType)

			err := handler.ValidateRequest(req)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestBitcoinHandler_GetHealthCheckMethod(t *testing.T) {
	handler := NewBitcoinHandler(&NetworkConfig{})

	expected := "getblockcount"
	if handler.GetHealthCheckMethod() != expected {
		t.Errorf("Expected health check method '%s', got '%s'", expected, handler.GetHealthCheckMethod())
	}
}

func TestBitcoinHandler_GetChainIDMethod(t *testing.T) {
	handler := NewBitcoinHandler(&NetworkConfig{})

	expected := "getblockchaininfo"
	if handler.GetChainIDMethod() != expected {
		t.Errorf("Expected chain ID method '%s', got '%s'", expected, handler.GetChainIDMethod())
	}
}

func TestBitcoinHandler_CreateHealthCheckPayload(t *testing.T) {
	handler := NewBitcoinHandler(&NetworkConfig{})

	payload, err := handler.CreateHealthCheckPayload("getblockcount")
	if err != nil {
		t.Fatalf("Failed to create health check payload: %v", err)
	}

	expected := `{"jsonrpc":"2.0","method":"getblockcount","id":1}`
	if string(payload) != expected {
		t.Errorf("Expected payload '%s', got '%s'", expected, string(payload))
	}
}

func TestBitcoinHandler_ParseHealthCheckResponse(t *testing.T) {
	handler := NewBitcoinHandler(&NetworkConfig{})

	tests := []struct {
		name        string
		response    string
		expectedNum int64
		expectError bool
	}{
		{
			name:        "valid_response",
			response:    `{"jsonrpc":"2.0","result":910901,"id":1}`,
			expectedNum: 910901,
			expectError: false,
		},
		{
			name:        "invalid_json",
			response:    `invalid json`,
			expectedNum: 0,
			expectError: true,
		},
		{
			name:        "missing_result",
			response:    `{"jsonrpc":"2.0","id":1}`,
			expectedNum: 0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blockInfo, err := handler.ParseHealthCheckResponse([]byte(tt.response))
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if blockInfo.Number != tt.expectedNum {
					t.Errorf("Expected block number %d, got %d", tt.expectedNum, blockInfo.Number)
				}
			}
		})
	}
}

func TestBitcoinHandler_ParseChainIDResponse(t *testing.T) {
	handler := NewBitcoinHandler(&NetworkConfig{})

	tests := []struct {
		name        string
		response    string
		statusCode  int
		expectedID  string
		expectError bool
	}{
		{
			name: "valid_mainnet_response",
			response: `{
				"jsonrpc": "2.0",
				"result": {
					"chain": "main",
					"blocks": 910909,
					"headers": 910909,
					"bestblockhash": "00000000000000000001063d6678998c1bb56431f5b161c043bec1c8dc0d36c7"
				},
				"id": 1
			}`,
			statusCode:  http.StatusOK,
			expectedID:  "main",
			expectError: false,
		},
		{
			name: "valid_testnet_response",
			response: `{
				"jsonrpc": "2.0",
				"result": {
					"chain": "test"
				},
				"id": 1
			}`,
			statusCode:  http.StatusOK,
			expectedID:  "test",
			expectError: false,
		},
		{
			name:        "http_error",
			response:    `{}`,
			statusCode:  http.StatusInternalServerError,
			expectedID:  "",
			expectError: true,
		},
		{
			name: "json_rpc_error",
			response: `{
				"jsonrpc": "2.0",
				"error": {
					"code": -32600,
					"message": "Invalid Request"
				},
				"id": null
			}`,
			statusCode:  http.StatusOK,
			expectedID:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chainID, err := handler.ParseChainIDResponse([]byte(tt.response), tt.statusCode)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if chainID != tt.expectedID {
					t.Errorf("Expected chain ID '%s', got '%s'", tt.expectedID, chainID)
				}
			}
		})
	}
}

func TestBitcoinHandler_CreateBlockRequest(t *testing.T) {
	handler := NewBitcoinHandler(&NetworkConfig{})

	tests := []struct {
		name                string
		method              string
		blockNum            int64
		includeTransactions bool
		expectedPayload     string
		expectError         bool
	}{
		{
			name:                "getblockhash_request",
			method:              "getblockhash",
			blockNum:            910905,
			includeTransactions: false,
			expectedPayload:     `{"jsonrpc":"2.0","method":"getblockhash","params":[910905],"id":1}`,
			expectError:         false,
		},
		{
			name:                "getblock_requires_hash",
			method:              "getblock",
			blockNum:            910905,
			includeTransactions: false,
			expectedPayload:     "",
			expectError:         true,
		},
		{
			name:                "unsupported_method",
			method:              "unsupported",
			blockNum:            910905,
			includeTransactions: false,
			expectedPayload:     "",
			expectError:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := handler.CreateBlockRequest(tt.method, tt.blockNum, tt.includeTransactions)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if string(payload) != tt.expectedPayload {
					t.Errorf("Expected payload '%s', got '%s'", tt.expectedPayload, string(payload))
				}
			}
		})
	}
}

func TestBitcoinHandler_CreateBlockRequestWithHash(t *testing.T) {
	handler := NewBitcoinHandler(&NetworkConfig{})

	tests := []struct {
		name                string
		blockHash           string
		includeTransactions bool
		expectedPayload     string
	}{
		{
			name:                "without_transactions",
			blockHash:           "00000000000000000000badb5c06bab4232d2f5a3e975b7abe4747d2edd85253",
			includeTransactions: false,
			expectedPayload:     `{"jsonrpc":"2.0","method":"getblock","params":["00000000000000000000badb5c06bab4232d2f5a3e975b7abe4747d2edd85253",1],"id":1}`,
		},
		{
			name:                "with_transactions",
			blockHash:           "00000000000000000000badb5c06bab4232d2f5a3e975b7abe4747d2edd85253",
			includeTransactions: true,
			expectedPayload:     `{"jsonrpc":"2.0","method":"getblock","params":["00000000000000000000badb5c06bab4232d2f5a3e975b7abe4747d2edd85253",2],"id":1}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := handler.CreateBlockRequestWithHash(tt.blockHash, tt.includeTransactions)
			if err != nil {
				t.Fatalf("Failed to create block request with hash: %v", err)
			}
			if string(payload) != tt.expectedPayload {
				t.Errorf("Expected payload '%s', got '%s'", tt.expectedPayload, string(payload))
			}
		})
	}
}

func TestBitcoinHandler_ExtractBlockHash(t *testing.T) {
	handler := NewBitcoinHandler(&NetworkConfig{})

	tests := []struct {
		name         string
		blockData    interface{}
		expectedHash string
	}{
		{
			name:         "string_hash",
			blockData:    "00000000000000000000badb5c06bab4232d2f5a3e975b7abe4747d2edd85253",
			expectedHash: "00000000000000000000badb5c06bab4232d2f5a3e975b7abe4747d2edd85253",
		},
		{
			name: "block_object_with_hash",
			blockData: map[string]interface{}{
				"hash":   "00000000000000000000badb5c06bab4232d2f5a3e975b7abe4747d2edd85253",
				"height": 910905,
			},
			expectedHash: "00000000000000000000badb5c06bab4232d2f5a3e975b7abe4747d2edd85253",
		},
		{
			name:         "raw_message_string",
			blockData:    json.RawMessage(`"00000000000000000000badb5c06bab4232d2f5a3e975b7abe4747d2edd85253"`),
			expectedHash: "00000000000000000000badb5c06bab4232d2f5a3e975b7abe4747d2edd85253",
		},
		{
			name:         "raw_message_object",
			blockData:    json.RawMessage(`{"hash":"00000000000000000000badb5c06bab4232d2f5a3e975b7abe4747d2edd85253"}`),
			expectedHash: "00000000000000000000badb5c06bab4232d2f5a3e975b7abe4747d2edd85253",
		},
		{
			name:         "invalid_data",
			blockData:    123,
			expectedHash: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := handler.ExtractBlockHash(tt.blockData)
			if hash != tt.expectedHash {
				t.Errorf("Expected hash '%s', got '%s'", tt.expectedHash, hash)
			}
		})
	}
}

func TestBitcoinHandler_GetSupportedMethods(t *testing.T) {
	handler := NewBitcoinHandler(&NetworkConfig{})

	methods := handler.GetSupportedMethods()

	// Check that critical methods are included
	expectedMethods := []string{
		"getblockcount",
		"getblockhash",
		"getblock",
		"getblockchaininfo",
	}

	for _, expected := range expectedMethods {
		found := false
		for _, method := range methods {
			if method == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected method '%s' not found in supported methods", expected)
		}
	}
}

func TestBitcoinHandler_ParseBlockNumberResponse(t *testing.T) {
	handler := NewBitcoinHandler(&NetworkConfig{})

	tests := []struct {
		name        string
		response    string
		statusCode  int
		expectedNum int64
		expectError bool
	}{
		{
			name:        "valid_response",
			response:    `{"jsonrpc":"2.0","result":910901,"id":1}`,
			statusCode:  http.StatusOK,
			expectedNum: 910901,
			expectError: false,
		},
		{
			name:        "rate_limit_error",
			response:    `{}`,
			statusCode:  429,
			expectedNum: 0,
			expectError: true,
		},
		{
			name:        "http_error",
			response:    `{}`,
			statusCode:  500,
			expectedNum: 0,
			expectError: true,
		},
		{
			name: "json_rpc_error",
			response: `{
				"jsonrpc": "2.0",
				"error": {
					"code": -32601,
					"message": "Method not found"
				},
				"id": 1
			}`,
			statusCode:  http.StatusOK,
			expectedNum: 0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blockNum, err := handler.ParseBlockNumberResponse([]byte(tt.response), tt.statusCode)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if blockNum != tt.expectedNum {
					t.Errorf("Expected block number %d, got %d", tt.expectedNum, blockNum)
				}
			}
		})
	}
}

func TestBitcoinHandler_SupportsGetBlockByNumber(t *testing.T) {
	handler := NewBitcoinHandler(&NetworkConfig{})

	if !handler.SupportsGetBlockByNumber() {
		t.Errorf("Expected Bitcoin handler to support get block by number")
	}
}

func TestBitcoinHandler_SupportsArchiveMode(t *testing.T) {
	handler := NewBitcoinHandler(&NetworkConfig{})

	if handler.SupportsArchiveMode() {
		t.Errorf("Expected Bitcoin handler to not support archive mode")
	}
}
