package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStarknetHandler_GetType(t *testing.T) {
	handler := NewStarknetHandler(&NetworkConfig{})

	assert.Equal(t, StarknetHandlerType, handler.GetType())
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
		name      string
		chainID   string
		shouldErr bool
	}{
		{
			name:      "valid mainnet chain ID",
			chainID:   "0x534e5f4d41494e",
			shouldErr: false,
		},
		{
			name:      "valid sepolia chain ID",
			chainID:   "0x534e5f5345504f4c4941",
			shouldErr: false,
		},
		{
			name:      "invalid - contains colon (old CAIP-2 format)",
			chainID:   "starknet:0x534e5f4d41494e",
			shouldErr: true,
		},
		{
			name:      "invalid - contains colon with different prefix",
			chainID:   "ethereum:0x1",
			shouldErr: true,
		},
		{
			name:      "missing 0x prefix",
			chainID:   "534e5f4d41494e",
			shouldErr: true,
		},
		{
			name:      "empty hex part",
			chainID:   "0x",
			shouldErr: true,
		},
		{
			name:      "invalid hex characters",
			chainID:   "0xghi",
			shouldErr: true,
		},
		{
			name:      "uppercase hex",
			chainID:   "0x534E5F4D41494E",
			shouldErr: false,
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

func TestStarknetHandler_GetDefaultMethods(t *testing.T) {
	handler := NewStarknetHandler(&NetworkConfig{})

	methods := handler.GetDefaultMethods()

	expectedMethods := []string{
		"starknet_blockNumber",
		"starknet_chainId",
		"starknet_call",
		"starknet_getBlockWithTxHashes",
		"starknet_getBlockWithTxs",
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
