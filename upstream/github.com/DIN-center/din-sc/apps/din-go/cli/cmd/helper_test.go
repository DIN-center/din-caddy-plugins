package dincli

import (
	"errors"
	"math/big"
	"os"
	"testing"

	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type MockFeesEstimator struct {
	fees FeeData
}

func (m *MockFeesEstimator) EstimateFees() (FeeData, error) {
	return m.fees, nil
}

func TestHelperFunctions(t *testing.T) {
	t.Run("readContractAddr fails if no address is set in environment or flag", func(t *testing.T) {
		// Clear environment
		os.Unsetenv("DIN_REGISTRY_CONTRACT_ADDR")
		dinRegistryContractAddr = ""

		result, err := readContractAddr()
		assert.Error(t, err)
		assert.Equal(t, "", result)
	})

	t.Run("readContractAddr set by flag", func(t *testing.T) {
		// Clear environment
		os.Unsetenv("DIN_REGISTRY_CONTRACT_ADDR")
		dinRegistryContractAddr = "0x1234567890123456789012345678901234567890"

		result, err := readContractAddr()
		assert.NoError(t, err)
		assert.Equal(t, "0x1234567890123456789012345678901234567890", result)

		// Reset
		dinRegistryContractAddr = ""
	})

	t.Run("readContractAddr set by environment", func(t *testing.T) {
		// Clear flag
		dinRegistryContractAddr = ""
		os.Setenv("DIN_REGISTRY_CONTRACT_ADDR", "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd")

		result, err := readContractAddr()
		assert.NoError(t, err)
		assert.Equal(t, "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd", result)

		// Cleanup
		os.Unsetenv("DIN_REGISTRY_CONTRACT_ADDR")
	})

	t.Run("readContractAddr set by flag takes precedence", func(t *testing.T) {
		os.Setenv("DIN_REGISTRY_CONTRACT_ADDR", "0xenvvalue")
		dinRegistryContractAddr = "0xflagvalue"

		result, err := readContractAddr()
		assert.NoError(t, err)
		assert.Equal(t, "0xflagvalue", result)

		// Cleanup
		os.Unsetenv("DIN_REGISTRY_CONTRACT_ADDR")
		dinRegistryContractAddr = ""
	})

	t.Run("readRPCURL fails if no URL is set in environment or flag", func(t *testing.T) {
		// Clear environment
		os.Unsetenv("RPC_URL")
		rpcURL = ""

		result, err := readRPCURL()
		assert.Error(t, err)
		assert.Equal(t, "", result)
	})

	t.Run("readRPCURL set by flag", func(t *testing.T) {
		// Clear environment
		os.Unsetenv("RPC_URL")
		rpcURL = "http://flag:8545"

		result, err := readRPCURL()
		assert.NoError(t, err)
		assert.Equal(t, "http://flag:8545", result)

		// Reset
		rpcURL = ""
	})

	t.Run("readRPCURL set by environment", func(t *testing.T) {
		// Clear flag
		rpcURL = ""
		os.Setenv("RPC_URL", "http://env:8545")

		result, err := readRPCURL()
		assert.NoError(t, err)
		assert.Equal(t, "http://env:8545", result)

		// Cleanup
		os.Unsetenv("RPC_URL")
	})

	t.Run("readRPCURL set by flag takes precedence", func(t *testing.T) {
		os.Setenv("RPC_URL", "http://env:8545")
		rpcURL = "http://flag:8545"

		result, err := readRPCURL()
		assert.NoError(t, err)
		assert.Equal(t, "http://flag:8545", result)

		// Cleanup
		os.Unsetenv("RPC_URL")
		rpcURL = ""
	})

	t.Run("readKeystorePath set by flag", func(t *testing.T) {
		// Clear environment
		os.Unsetenv("KEYSTORE_PATH")
		keystorePath = "/path/to/flag/keystore"

		result, err := readKeystorePath()
		assert.NoError(t, err)
		assert.Equal(t, "/path/to/flag/keystore", result)

		// Reset
		keystorePath = ""
	})

	t.Run("readKeystorePath with environment", func(t *testing.T) {
		// Clear flag
		keystorePath = ""
		os.Setenv("KEYSTORE_PATH", "/path/to/env/keystore")

		result, err := readKeystorePath()
		assert.NoError(t, err)
		assert.Equal(t, "/path/to/env/keystore", result)

		// Cleanup
		os.Unsetenv("KEYSTORE_PATH")
	})

	t.Run("readKeystorePath set by flag takes precedence", func(t *testing.T) {
		os.Setenv("KEYSTORE_PATH", "/path/to/env/keystore")
		keystorePath = "/path/to/flag/keystore"

		result, err := readKeystorePath()
		assert.NoError(t, err)
		assert.Equal(t, "/path/to/flag/keystore", result)

		// Cleanup
		os.Unsetenv("KEYSTORE_PATH")
		keystorePath = ""
	})

	t.Run("trim whitespace in helper functions", func(t *testing.T) {
		dinRegistryContractAddr = "  0x1234567890123456789012345678901234567890  "
		result, err := readContractAddr()
		assert.NoError(t, err)
		assert.Equal(t, "0x1234567890123456789012345678901234567890", result)

		rpcURL = "  http://test:8545  "
		result, err = readRPCURL()
		assert.NoError(t, err)
		assert.Equal(t, "http://test:8545", result)

		// Reset
		dinRegistryContractAddr = ""
		rpcURL = ""
	})

	t.Run("adjustTxOptions with fees estimator, gas price set", func(t *testing.T) {
		mockFeesEstimator := &MockFeesEstimator{fees: FeeData{GasPrice: big.NewInt(1000000000)}}
		//set global variable
		feesEstimator = mockFeesEstimator
		gasPriceInWei = 0

		//call adjustTxOptions
		txOptions := &bind.TransactOpts{}
		err := adjustTxOptions(txOptions)
		assert.NoError(t, err)

		//assert that the gas price was set
		assert.Equal(t, uint64(1000000000), txOptions.GasPrice.Uint64())
	})

	t.Run("adjustTxOptions without fees estimator, gas price flag set", func(t *testing.T) {
		mockFeesEstimator := &MockFeesEstimator{fees: FeeData{}}
		feesEstimator = mockFeesEstimator
		gasPriceInWei = 99999

		//call adjustTxOptions
		txOptions := &bind.TransactOpts{}
		err := adjustTxOptions(txOptions)
		assert.NoError(t, err)

		//assert that the gas price was set
		assert.Equal(t, uint64(99999), txOptions.GasPrice.Uint64())
	})

	t.Run("adjustTxOptions with fees estimator, gas fee cap and tip cap set", func(t *testing.T) {
		mockFeesEstimator := &MockFeesEstimator{fees: FeeData{GasTipCap: big.NewInt(99999), GasFeeCap: big.NewInt(77777)}}
		feesEstimator = mockFeesEstimator
		gasPriceInWei = 0

		//call adjustTxOptions
		txOptions := &bind.TransactOpts{}
		err := adjustTxOptions(txOptions)
		assert.NoError(t, err)

		//assert that the gas fee cap and tip cap were set
		assert.Equal(t, uint64(77777), txOptions.GasFeeCap.Uint64())
		assert.Equal(t, uint64(99999), txOptions.GasTipCap.Uint64())
	})

	t.Run("adjustTxOptions with dry run", func(t *testing.T) {
		mockFeesEstimator := &MockFeesEstimator{fees: FeeData{GasPrice: big.NewInt(1000000000)}}
		feesEstimator = mockFeesEstimator
		dryRun = true

		//call adjustTxOptions
		txOptions := &bind.TransactOpts{}
		err := adjustTxOptions(txOptions)
		assert.NoError(t, err)

		//assert that the dry run flag was set
		assert.True(t, txOptions.NoSend)
	})

	t.Run("adjustTxOptions with dry nonce", func(t *testing.T) {
		mockFeesEstimator := &MockFeesEstimator{fees: FeeData{GasPrice: big.NewInt(1000000000)}}
		feesEstimator = mockFeesEstimator
		nonce = 1984

		//call adjustTxOptions
		txOptions := &bind.TransactOpts{}
		err := adjustTxOptions(txOptions)
		assert.NoError(t, err)

		//assert that the nonce was set
		assert.Equal(t, uint64(1984), txOptions.Nonce.Uint64())
	})

}

func TestFilterProviders(t *testing.T) {
	t.Run("filter providers by name", func(t *testing.T) {
		providers := []*din.Provider{
			{Name: "Provider1"},
			{Name: "Provider2"},
			{Name: "provider1"}, // Case insensitive
		}

		filtered := filterProviders(providers, "Provider1")
		assert.Len(t, filtered, 2)
		assert.Equal(t, "Provider1", filtered[0].Name)
		assert.Equal(t, "provider1", filtered[1].Name)
	})

	t.Run("filter providers no match", func(t *testing.T) {
		providers := []*din.Provider{
			{Name: "Provider1"},
			{Name: "Provider2"},
		}

		filtered := filterProviders(providers, "Provider3")
		assert.Len(t, filtered, 0)
	})

	t.Run("filter providers empty list", func(t *testing.T) {
		providers := []*din.Provider{}

		filtered := filterProviders(providers, "Provider1")
		assert.Len(t, filtered, 0)
	})
}
func TestBuildNetworkConfigFromUserInput(t *testing.T) {
	// Setup mock
	mockCtrl := gomock.NewController(t)
	mockDinClient := din.NewMockIDinClient(mockCtrl)

	t.Run("Success call, update single field", func(t *testing.T) {
		// Mock GetNetworkByName to return existing config
		existingNetwork := &din.Network{
			NetworkConfig: &din.NetworkOperationsConfig{
				Handler:                  "evm",
				HealthcheckIntervalSec:   30,
				HealthcheckThreshold:     2,
				HealthcheckTimeout:       5,
				BlockLagLimit:            5,
				BlockJumpLimit:           100,
				RequestAttemptCount:      3,
				MaxRequestPayloadSizeKb:  512,
				RegistryBlockEpoch:       2000,
				ArchiveEnabled:           false,
				ProviderBlockHistorySize: 10,
				NetworkBlockHistorySize:  128,
				ChainId:                  "0x1",
			},
		}
		mockDinClient.EXPECT().GetNetworkByName("test://network").Return(existingNetwork, nil).Times(1)

		// Execute function being tested
		config, err := buildNetworkConfigFromUserInput(`{"handler": "starknet"}`, mockDinClient, "test://network")

		// Validate the result
		assert.NoError(t, err)
		assert.Equal(t, "starknet", config.Handler)               // Updated
		assert.Equal(t, uint8(30), config.HealthcheckIntervalSec) // Unchanged
		assert.Equal(t, "0x1", config.ChainId)                    // Unchanged
	})

	t.Run("Success call, update multiple fields", func(t *testing.T) {
		// Mock GetNetworkByName to return existing config
		existingNetwork := &din.Network{
			NetworkConfig: &din.NetworkOperationsConfig{
				Handler:                  "evm",
				HealthcheckIntervalSec:   30,
				HealthcheckThreshold:     2,
				HealthcheckTimeout:       5,
				BlockLagLimit:            5,
				BlockJumpLimit:           100,
				RequestAttemptCount:      3,
				MaxRequestPayloadSizeKb:  512,
				RegistryBlockEpoch:       2000,
				ArchiveEnabled:           false,
				ProviderBlockHistorySize: 10,
				NetworkBlockHistorySize:  128,
				ChainId:                  "0x1",
			},
		}
		mockDinClient.EXPECT().GetNetworkByName("test://network").Return(existingNetwork, nil).Times(1)

		// Execute function being tested
		config, err := buildNetworkConfigFromUserInput(`{"handler": "starknet", "health_check_interval_sec": 60, "chain_id": "0x5"}`, mockDinClient, "test://network")

		// Validate the result
		assert.NoError(t, err)
		assert.Equal(t, "starknet", config.Handler)               // Updated
		assert.Equal(t, uint8(60), config.HealthcheckIntervalSec) // Updated
		assert.Equal(t, "0x5", config.ChainId)                    // Updated
		assert.Equal(t, uint8(2), config.HealthcheckThreshold)    // Unchanged
	})

	t.Run("Failure call, invalid JSON", func(t *testing.T) {
		// Execute function being tested with invalid JSON
		_, err := buildNetworkConfigFromUserInput(`{"handler": "starknet", invalid json}`, mockDinClient, "test://network")

		// Validate the result
		assert.Error(t, err)
	})

	t.Run("Failure call, GetNetworkByName fails", func(t *testing.T) {
		// Mock GetNetworkByName failure
		mockDinClient.EXPECT().GetNetworkByName("test://network").Return(nil, errors.New("network not found")).Times(1)

		// Execute function being tested
		_, err := buildNetworkConfigFromUserInput(`{"handler": "starknet"}`, mockDinClient, "test://network")

		// Validate the result
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "network not found")
	})

	t.Run("Failure call, empty JSON object", func(t *testing.T) {
		// Mock GetNetworkByName to return existing config
		existingNetwork := &din.Network{
			NetworkConfig: &din.NetworkOperationsConfig{
				Handler:                  "evm",
				HealthcheckIntervalSec:   30,
				HealthcheckThreshold:     2,
				HealthcheckTimeout:       5,
				BlockLagLimit:            5,
				BlockJumpLimit:           100,
				RequestAttemptCount:      3,
				MaxRequestPayloadSizeKb:  512,
				RegistryBlockEpoch:       2000,
				ArchiveEnabled:           false,
				ProviderBlockHistorySize: 10,
				NetworkBlockHistorySize:  128,
				ChainId:                  "0x1",
			},
		}
		mockDinClient.EXPECT().GetNetworkByName("test://network").Return(existingNetwork, nil).Times(1)

		// Execute function being tested
		_, err := buildNetworkConfigFromUserInput(`{}`, mockDinClient, "test://network")

		// Validate the result
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nothing to update")
	})
}
