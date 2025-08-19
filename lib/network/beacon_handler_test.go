package network

import (
	"net/http"
	"strings"
	"testing"
)

func TestBeaconChainHandler_GetType(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	if handler.GetType() != "beacon-chain" {
		t.Errorf("Expected type 'beacon-chain', got '%s'", handler.GetType())
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
			chainID:   "1",
			shouldErr: false,
		},
		{
			name:      "valid sepolia chain ID",
			chainID:   "11155111",
			shouldErr: false,
		},
		{
			name:      "valid goerli chain ID",
			chainID:   "5",
			shouldErr: false,
		},
		{
			name:      "invalid - contains colon (old CAIP-2 format)",
			chainID:   "beacon:1",
			shouldErr: true,
		},
		{
			name:      "invalid - contains colon with different prefix",
			chainID:   "starknet:0x1",
			shouldErr: true,
		},
		{
			name:      "invalid - not a number",
			chainID:   "abc",
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
		"/eth/v2/beacon/blocks/head",
		"/eth/v2/beacon/blocks/{block_id}",
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

	expected := "/eth/v2/beacon/blocks/head"
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

func TestBeaconChainHandler_GetBlockInfoMethod(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	expected := "/eth/v2/beacon/blocks/head"
	if handler.GetBlockInfoMethod() != expected {
		t.Errorf("Expected block info method '%s', got '%s'", expected, handler.GetBlockInfoMethod())
	}
}

func TestBeaconChainHandler_CreateHealthCheckPayload(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	payload, err := handler.CreateHealthCheckPayload("/eth/v1/beacon/headers")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
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
				ExecutionOptimistic: false,
				Finalized:           true,
				Data: []struct {
					Root      string `json:"root"`
					Canonical bool   `json:"canonical"`
					Header    struct {
						Message struct {
							Slot          string `json:"slot"`
							ProposerIndex string `json:"proposer_index"`
							ParentRoot    string `json:"parent_root"`
							StateRoot     string `json:"state_root"`
							BodyRoot      string `json:"body_root"`
						} `json:"message"`
						Signature string `json:"signature"`
					} `json:"header"`
				}{
					{
						Root:      "0x1234567890abcdef",
						Canonical: true,
						Header: struct {
							Message struct {
								Slot          string `json:"slot"`
								ProposerIndex string `json:"proposer_index"`
								ParentRoot    string `json:"parent_root"`
								StateRoot     string `json:"state_root"`
								BodyRoot      string `json:"body_root"`
							} `json:"message"`
							Signature string `json:"signature"`
						}{
							Message: struct {
								Slot          string `json:"slot"`
								ProposerIndex string `json:"proposer_index"`
								ParentRoot    string `json:"parent_root"`
								StateRoot     string `json:"state_root"`
								BodyRoot      string `json:"body_root"`
							}{
								Slot: "12345",
							},
						},
					},
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
				"execution_optimistic": false,
				"finalized": true,
				"data": [
					{
						"root": "0x1234567890abcdef",
						"canonical": true,
						"header": {
							"message": {
								"slot": "7300000",
								"proposer_index": "1",
								"parent_root": "0xabcd",
								"state_root": "0xefgh",
								"body_root": "0xijkl"
							},
							"signature": "0x12345678"
						}
					}
				]
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
				"execution_optimistic": false,
				"finalized": true,
				"data": [
					{
						"root": "0x1234567890abcdef",
						"canonical": true,
						"header": {
							"message": {},
							"signature": "0x12345678"
						}
					}
				]
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
			path:        "/eth/v1/beacon/headers",
			expectError: false,
		},
		{
			name:        "valid POST request",
			method:      "POST",
			path:        "/eth/v1/beacon/headers",
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

func TestBeaconChainHandler_ParseHealthCheckResponse_NewFormat(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	t.Run("Valid beacon head response with v2 blocks format", func(t *testing.T) {
		responseBody := `{
			"version": "phase0",
			"execution_optimistic": false,
			"finalized": true,
			"data": {
				"message": {
					"slot": "12345",
					"proposer_index": "1",
					"parent_root": "0xabcd1234",
					"state_root": "0xefgh5678",
					"body": {}
				},
				"signature": "0x1234567890abcdef"
			}
		}`

		blockInfo, err := handler.ParseHealthCheckResponse([]byte(responseBody))

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if blockInfo == nil {
			t.Fatal("Expected blockInfo to be non-nil")
		}

		if blockInfo.Number != 12345 {
			t.Errorf("Expected block number 12345, got %d", blockInfo.Number)
		}

		// Check metadata for slot
		expectedSlot := int64(12345)
		if slot := getInt64FromMetadata(blockInfo.Metadata, "slot"); slot != expectedSlot {
			t.Errorf("Expected slot 12345, got %d", slot)
		}

		// Check metadata for epoch
		expectedEpoch := int64(385) // 12345 / 32 = 385
		if epoch := getInt64FromMetadata(blockInfo.Metadata, "epoch"); epoch != expectedEpoch {
			t.Errorf("Expected epoch 385, got %d", epoch)
		}

		if blockInfo.Hash != "0xabcd1234" {
			t.Errorf("Expected hash to be parent_root '0xabcd1234', got %s", blockInfo.Hash)
		}

		// Check metadata for finalized
		if finalized := getBoolFromMetadata(blockInfo.Metadata, "finalized"); !finalized {
			t.Error("Expected finalized to be true")
		}

		// Check metadata for execution_optimistic
		if execOptimistic := getBoolFromMetadata(blockInfo.Metadata, "execution_optimistic"); execOptimistic {
			t.Error("Expected execution_optimistic to be false")
		}
	})

	t.Run("Missing slot field", func(t *testing.T) {
		responseBody := `{
			"execution_optimistic": false,
			"finalized": true,
			"data": {
				"message": {
					"proposer_index": "1",
					"parent_root": "0xabcd1234",
					"state_root": "0xefgh5678",
					"body": {}
				},
				"signature": "0x1234567890abcdef"
			}
		}`

		_, err := handler.ParseHealthCheckResponse([]byte(responseBody))

		if err == nil {
			t.Error("Expected error for missing slot field")
		}

		if !strings.Contains(err.Error(), "failed to parse slot") {
			t.Errorf("Expected error about parsing slot, got: %v", err)
		}
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		responseBody := `{invalid json`

		_, err := handler.ParseHealthCheckResponse([]byte(responseBody))

		if err == nil {
			t.Error("Expected error for invalid JSON")
		}
	})
}

func TestBeaconChainHandler_ParseHealthCheckResponse_RealData(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	t.Run("Real beacon chain response v2 blocks format", func(t *testing.T) {
		// This simulates response data from /eth/v2/beacon/blocks/head
		responseBody := `{
			"version": "deneb",
			"execution_optimistic": false,
			"finalized": false,
			"data": {
				"message": {
					"slot": "12170038",
					"proposer_index": "1527972",
					"parent_root": "0xc0b76a4d9893b6421cf7860ab3c39a88b9ae4e461780f0b14fa87f4af3bb29b7",
					"state_root": "0xcc6c1d4cb2cca1835f3d8c3c1ecd7e38ae2e29ca5029fce25e80376abf651d2d",
					"body": {}
				},
				"signature": "0xa10a8090622b91cf1e30e7ce35374c3365be00b039612169e0ed581598613dd2807b40dcafc23a561817254c7b9c10ce1977b9ffcd3ebebec55dfd5e005e988517eff7c32c6bd7ce675362a1115ce530b4bc14d543352ddca0d7e99316d80cfb"
			}
		}`

		blockInfo, err := handler.ParseHealthCheckResponse([]byte(responseBody))

		if err != nil {
			t.Fatalf("Expected no error parsing real beacon data, got: %v", err)
		}

		if blockInfo == nil {
			t.Fatal("Expected blockInfo to be non-nil")
		}

		// Verify the slot number (12170038)
		expectedSlot := int64(12170038)
		if blockInfo.Number != expectedSlot {
			t.Errorf("Expected block number %d, got %d", expectedSlot, blockInfo.Number)
		}

		// Check metadata for slot
		if slot := getInt64FromMetadata(blockInfo.Metadata, "slot"); slot != expectedSlot {
			t.Errorf("Expected slot %d, got %d", expectedSlot, slot)
		}

		// Verify epoch calculation (slot / 32)
		expectedEpoch := expectedSlot / 32 // 12170038 / 32 = 380313
		if epoch := getInt64FromMetadata(blockInfo.Metadata, "epoch"); epoch != expectedEpoch {
			t.Errorf("Expected epoch %d, got %d", expectedEpoch, epoch)
		}

		// Verify the parent root hash (v2 blocks uses parent_root as hash)
		expectedHash := "0xc0b76a4d9893b6421cf7860ab3c39a88b9ae4e461780f0b14fa87f4af3bb29b7"
		if blockInfo.Hash != expectedHash {
			t.Errorf("Expected hash %s, got %s", expectedHash, blockInfo.Hash)
		}

		// Verify beacon-specific fields
		if execOptimistic := getBoolFromMetadata(blockInfo.Metadata, "execution_optimistic"); execOptimistic {
			t.Error("Expected execution_optimistic to be false")
		}

		if finalized := getBoolFromMetadata(blockInfo.Metadata, "finalized"); finalized {
			t.Error("Expected finalized to be false")
		}

		slot := getInt64FromMetadata(blockInfo.Metadata, "slot")
		epoch := getInt64FromMetadata(blockInfo.Metadata, "epoch")
		t.Logf("Successfully parsed real beacon data: slot=%d, epoch=%d, hash=%s",
			slot, epoch, blockInfo.Hash)
	})

	t.Run("Latest real beacon chain response v2 blocks format", func(t *testing.T) {
		// This simulates response data from /eth/v2/beacon/blocks/head
		responseBody := `{
			"version": "deneb",
			"execution_optimistic": false,
			"finalized": false,
			"data": {
				"message": {
					"slot": "12170151",
					"proposer_index": "568631",
					"parent_root": "0xb771076ff61241f2c6b176f98f662bc3a2df659dfa6cf60bf099a1ed67fb7438",
					"state_root": "0x88930925964a327cc9896780b8769bde8d4bcbea4e1bdd9ff6ff5894f966e151",
					"body": {}
				},
				"signature": "0xb1aa51a5b01c44f2fe22624be813992f2e765c7fe18c6c7f714ba100b5eb04908db5a4ad58bee036ebd43a15ef0a098514c9b6c82c800e8b11b20ce547b3eb59b1d938926782daf7ca2be88334d0d330a6335a31202e1ea20f8b35ffbe5cf160"
			}
		}`

		blockInfo, err := handler.ParseHealthCheckResponse([]byte(responseBody))

		if err != nil {
			t.Fatalf("Expected no error parsing latest beacon data, got: %v", err)
		}

		if blockInfo == nil {
			t.Fatal("Expected blockInfo to be non-nil")
		}

		// Verify the slot number (12170151)
		expectedSlot := int64(12170151)
		if blockInfo.Number != expectedSlot {
			t.Errorf("Expected block number %d, got %d", expectedSlot, blockInfo.Number)
		}

		// Check metadata for slot
		if slot := getInt64FromMetadata(blockInfo.Metadata, "slot"); slot != expectedSlot {
			t.Errorf("Expected slot %d, got %d", expectedSlot, slot)
		}

		// Verify epoch calculation (slot / 32)
		expectedEpoch := expectedSlot / 32 // 12170151 / 32 = 380317
		if epoch := getInt64FromMetadata(blockInfo.Metadata, "epoch"); epoch != expectedEpoch {
			t.Errorf("Expected epoch %d, got %d", expectedEpoch, epoch)
		}

		// Verify the parent root hash (v2 blocks uses parent_root as hash)
		expectedHash := "0xb771076ff61241f2c6b176f98f662bc3a2df659dfa6cf60bf099a1ed67fb7438"
		if blockInfo.Hash != expectedHash {
			t.Errorf("Expected hash %s, got %s", expectedHash, blockInfo.Hash)
		}

		// Verify beacon-specific fields
		if execOptimistic := getBoolFromMetadata(blockInfo.Metadata, "execution_optimistic"); execOptimistic {
			t.Error("Expected execution_optimistic to be false")
		}

		if finalized := getBoolFromMetadata(blockInfo.Metadata, "finalized"); finalized {
			t.Error("Expected finalized to be false")
		}

		slot := getInt64FromMetadata(blockInfo.Metadata, "slot")
		epoch := getInt64FromMetadata(blockInfo.Metadata, "epoch")
		t.Logf("Successfully parsed latest beacon data: slot=%d, epoch=%d, hash=%s",
			slot, epoch, blockInfo.Hash)
	})
}

func TestBeaconChainHandler_ParseChainIDResponse_RealData(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	t.Run("Real config/spec response from Rivet", func(t *testing.T) {
		// This is actual response data from eth.beacon.rivet.cloud/eth/v1/config/spec
		responseBody := `{
			"data": {
				"CONFIG_NAME": "mainnet",
				"PRESET_BASE": "mainnet",
				"DEPOSIT_CHAIN_ID": "1",
				"DEPOSIT_NETWORK_ID": "1",
				"DEPOSIT_CONTRACT_ADDRESS": "0x00000000219ab540356cbb839cbe05303d7705fa",
				"TERMINAL_TOTAL_DIFFICULTY": "58750000000000000000000",
				"MIN_GENESIS_ACTIVE_VALIDATOR_COUNT": "16384",
				"MIN_GENESIS_TIME": "1606824000",
				"SECONDS_PER_SLOT": "12",
				"SLOTS_PER_EPOCH": "32"
			}
		}`

		chainID, err := handler.ParseChainIDResponse([]byte(responseBody), 200)

		if err != nil {
			t.Fatalf("Expected no error parsing real config data, got: %v", err)
		}

		// Should return just "1" for mainnet (no prefix)
		expectedChainID := "1"
		if chainID != expectedChainID {
			t.Errorf("Expected chain ID %s, got %s", expectedChainID, chainID)
		}

		t.Logf("Successfully parsed real config data: chain_id=%s", chainID)
	})

	t.Run("Error response handling", func(t *testing.T) {
		// Test error status code
		_, err := handler.ParseChainIDResponse([]byte(`{"error": "not found"}`), 404)
		if err == nil {
			t.Error("Expected error for 404 status code")
		}
	})

	t.Run("Missing DEPOSIT_CHAIN_ID", func(t *testing.T) {
		// Test response without DEPOSIT_CHAIN_ID
		responseBody := `{
			"data": {
				"CONFIG_NAME": "mainnet",
				"PRESET_BASE": "mainnet"
			}
		}`

		_, err := handler.ParseChainIDResponse([]byte(responseBody), 200)
		if err == nil {
			t.Error("Expected error when DEPOSIT_CHAIN_ID is missing")
		}
	})

	t.Run("Array format response (Chainstack format)", func(t *testing.T) {
		// Some providers like Chainstack return data as an array
		responseBody := `{
			"data": [
				{
					"DEPOSIT_CHAIN_ID": "1",
					"PRESET_BASE": "mainnet"
				}
			]
		}`

		chainID, err := handler.ParseChainIDResponse([]byte(responseBody), 200)
		if err != nil {
			t.Fatalf("Failed to parse array format response: %v", err)
		}

		if chainID != "1" {
			t.Errorf("Expected chain_id '1', got '%s'", chainID)
		}

		t.Logf("Successfully parsed array format config data: chain_id=%s", chainID)
	})

	t.Run("Array format with numeric DEPOSIT_CHAIN_ID", func(t *testing.T) {
		// Test numeric format in array
		responseBody := `{
			"data": [
				{
					"DEPOSIT_CHAIN_ID": 1,
					"PRESET_BASE": "mainnet"
				}
			]
		}`

		chainID, err := handler.ParseChainIDResponse([]byte(responseBody), 200)
		if err != nil {
			t.Fatalf("Failed to parse array format with numeric ID: %v", err)
		}

		if chainID != "1" {
			t.Errorf("Expected chain_id '1', got '%s'", chainID)
		}
	})

	t.Run("Chainstack format with mixed types including BLOB_SCHEDULE", func(t *testing.T) {
		// Actual Chainstack response format with BLOB_SCHEDULE array
		responseBody := `{
			"data": {
				"DEPOSIT_CHAIN_ID": "1",
				"PRESET_BASE": "mainnet",
				"DEPOSIT_CONTRACT_ADDRESS": "0x00000000219ab540356cbb839cbe05303d7705fa",
				"BLOB_SCHEDULE": [
					{
						"EPOCH": "269568",
						"MAX_BLOBS_PER_BLOCK": "6"
					},
					{
						"EPOCH": "364032",
						"MAX_BLOBS_PER_BLOCK": "9"
					}
				],
				"SECONDS_PER_SLOT": "12"
			}
		}`

		chainID, err := handler.ParseChainIDResponse([]byte(responseBody), 200)
		if err != nil {
			t.Fatalf("Failed to parse Chainstack format with mixed types: %v", err)
		}

		if chainID != "1" {
			t.Errorf("Expected chain_id '1', got '%s'", chainID)
		}

		t.Logf("Successfully parsed Chainstack format with BLOB_SCHEDULE: chain_id=%s", chainID)
	})
}

func TestBeaconChainHandler_EndpointConfiguration(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	t.Run("Verify endpoint and HTTP method configuration", func(t *testing.T) {
		// Test the endpoint configuration
		endpoint := handler.GetBlockInfoMethod()
		if endpoint != "/eth/v2/beacon/blocks/head" {
			t.Errorf("Expected endpoint '/eth/v2/beacon/blocks/head', got '%s'", endpoint)
		}

		httpMethod := handler.GetHealthCheckHTTPMethod()
		if httpMethod != "GET" {
			t.Errorf("Expected HTTP method 'GET', got '%s'", httpMethod)
		}

		t.Logf("Beacon chain configuration verified: endpoint=%s, method=%s", endpoint, httpMethod)
	})
}

func TestBeaconChainHandler_HealthCheckResponse_RealData(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	t.Run("Real health check response - ready (200)", func(t *testing.T) {
		// Real health check endpoints typically return empty bodies with status codes
		// 200 = ready, 206 = syncing, 503 = not ready
		emptyBody := []byte("")

		blockInfo, err := handler.ParseHealthCheckResponse(emptyBody)

		if err != nil {
			t.Fatalf("Expected no error for empty health check response, got: %v", err)
		}

		if blockInfo == nil {
			t.Fatal("Expected blockInfo to be non-nil")
		}

		// Should return placeholder values for health-only responses
		if blockInfo.Number != -1 {
			t.Errorf("Expected block number -1 for health-only response, got %d", blockInfo.Number)
		}

		// Check metadata for slot
		if slot := getInt64FromMetadata(blockInfo.Metadata, "slot"); slot != -1 {
			t.Errorf("Expected slot -1 for health-only response, got %d", slot)
		}

		if blockInfo.Hash != "" {
			t.Errorf("Expected empty hash for health-only response, got %s", blockInfo.Hash)
		}

		t.Logf("Successfully handled empty health check response")
	})

	t.Run("Health check with JSON error response", func(t *testing.T) {
		// Some providers might return JSON error responses
		errorBody := []byte(`{"code": 503, "message": "Beacon node is syncing"}`)

		// This should still try to parse as beacon head response and fail gracefully
		_, err := handler.ParseHealthCheckResponse(errorBody)

		// Should return an error since it's not valid beacon head format
		if err == nil {
			t.Error("Expected error for JSON error response")
		}

		t.Logf("Correctly handled error response: %v", err)
	})
}

func TestGetInt64FromMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]interface{}
		key      string
		expected int64
	}{
		{
			name:     "valid int64 value",
			metadata: map[string]interface{}{"slot": int64(12345)},
			key:      "slot",
			expected: 12345,
		},
		{
			name:     "missing key",
			metadata: map[string]interface{}{"other": int64(999)},
			key:      "slot",
			expected: 0,
		},
		{
			name:     "nil metadata",
			metadata: nil,
			key:      "slot",
			expected: 0,
		},
		{
			name:     "wrong type",
			metadata: map[string]interface{}{"slot": "not-a-number"},
			key:      "slot",
			expected: 0,
		},
		{
			name:     "empty metadata",
			metadata: map[string]interface{}{},
			key:      "slot",
			expected: 0,
		},
		{
			name:     "zero value",
			metadata: map[string]interface{}{"slot": int64(0)},
			key:      "slot",
			expected: 0,
		},
		{
			name:     "negative value",
			metadata: map[string]interface{}{"epoch": int64(-1)},
			key:      "epoch",
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getInt64FromMetadata(tt.metadata, tt.key)
			if result != tt.expected {
				t.Errorf("getInt64FromMetadata(%v, %s) = %d, expected %d", tt.metadata, tt.key, result, tt.expected)
			}
		})
	}
}

func TestGetBoolFromMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]interface{}
		key      string
		expected bool
	}{
		{
			name:     "valid true value",
			metadata: map[string]interface{}{"finalized": true},
			key:      "finalized",
			expected: true,
		},
		{
			name:     "valid false value",
			metadata: map[string]interface{}{"execution_optimistic": false},
			key:      "execution_optimistic",
			expected: false,
		},
		{
			name:     "missing key",
			metadata: map[string]interface{}{"other": true},
			key:      "finalized",
			expected: false,
		},
		{
			name:     "nil metadata",
			metadata: nil,
			key:      "finalized",
			expected: false,
		},
		{
			name:     "wrong type",
			metadata: map[string]interface{}{"finalized": "true"},
			key:      "finalized",
			expected: false,
		},
		{
			name:     "empty metadata",
			metadata: map[string]interface{}{},
			key:      "finalized",
			expected: false,
		},
		{
			name:     "int value instead of bool",
			metadata: map[string]interface{}{"finalized": 1},
			key:      "finalized",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getBoolFromMetadata(tt.metadata, tt.key)
			if result != tt.expected {
				t.Errorf("getBoolFromMetadata(%v, %s) = %t, expected %t", tt.metadata, tt.key, result, tt.expected)
			}
		})
	}
}

func TestBeaconChainHandler_ParseHealthCheckResponse_WithMetadata(t *testing.T) {
	handler := NewBeaconChainHandler(&NetworkConfig{})

	t.Run("Verify metadata structure in parsed response", func(t *testing.T) {
		responseBody := `{
			"version": "phase0",
			"execution_optimistic": true,
			"finalized": false,
			"data": {
				"message": {
					"slot": "7890",
					"proposer_index": "1",
					"parent_root": "0xtest1234",
					"state_root": "0xtest5678",
					"body": {}
				},
				"signature": "0xsignature"
			}
		}`

		blockInfo, err := handler.ParseHealthCheckResponse([]byte(responseBody))

		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		// Verify metadata is not nil
		if blockInfo.Metadata == nil {
			t.Fatal("Expected metadata to be non-nil")
		}

		// Verify all expected metadata keys exist
		expectedKeys := []string{"slot", "epoch", "execution_optimistic", "finalized"}
		for _, key := range expectedKeys {
			if _, exists := blockInfo.Metadata[key]; !exists {
				t.Errorf("Expected metadata key '%s' to exist", key)
			}
		}

		// Verify metadata values using helper functions
		if slot := getInt64FromMetadata(blockInfo.Metadata, "slot"); slot != 7890 {
			t.Errorf("Expected metadata slot 7890, got %d", slot)
		}

		expectedEpoch := int64(7890) / 32 // 246
		if epoch := getInt64FromMetadata(blockInfo.Metadata, "epoch"); epoch != expectedEpoch {
			t.Errorf("Expected metadata epoch %d, got %d", expectedEpoch, epoch)
		}

		if execOptimistic := getBoolFromMetadata(blockInfo.Metadata, "execution_optimistic"); !execOptimistic {
			t.Error("Expected metadata execution_optimistic to be true")
		}

		if finalized := getBoolFromMetadata(blockInfo.Metadata, "finalized"); finalized {
			t.Error("Expected metadata finalized to be false")
		}
	})

	t.Run("Empty body creates metadata with placeholder values", func(t *testing.T) {
		blockInfo, err := handler.ParseHealthCheckResponse([]byte(""))

		if err != nil {
			t.Fatalf("Expected no error for empty body, got: %v", err)
		}

		// Verify metadata exists and has placeholder values
		if blockInfo.Metadata == nil {
			t.Fatal("Expected metadata to be non-nil even for empty response")
		}

		if slot := getInt64FromMetadata(blockInfo.Metadata, "slot"); slot != -1 {
			t.Errorf("Expected metadata slot -1 for empty response, got %d", slot)
		}

		if epoch := getInt64FromMetadata(blockInfo.Metadata, "epoch"); epoch != -1 {
			t.Errorf("Expected metadata epoch -1 for empty response, got %d", epoch)
		}
	})
}
