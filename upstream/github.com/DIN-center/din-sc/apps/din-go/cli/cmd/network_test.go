package dincli

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestNetworkCommand(t *testing.T) {
	t.Run("network command has subcommands", func(t *testing.T) {
		commands := networkCmd.Commands()
		commandNames := make([]string, len(commands))
		for i, cmd := range commands {
			commandNames[i] = cmd.Name()
		}

		assert.Contains(t, commandNames, "list")
		assert.Contains(t, commandNames, "set-status")
		assert.Contains(t, commandNames, "set-config")
	})
}

func TestListNetworksCommand(t *testing.T) {
	// Setup mock
	mockCtrl := gomock.NewController(t)
	mockFormatter := NewMockIOutputFormatter(mockCtrl)
	mockDinClient := din.NewMockIDinClient(mockCtrl)

	// set CLI states
	dinClient = mockDinClient
	outputFormatter = mockFormatter

	// Helper function to reset global variables
	resetListGlobalVariables := func() {
		networkURI = ""
		networkListFull = false
	}

	t.Run("Success call, no flags", func(t *testing.T) {
		resetListGlobalVariables()

		// set mock expectation for GetAllNetworks
		mockDinClient.EXPECT().GetAllNetworks().Return([]*din.Network{}, nil).Times(1)
		// set mock expectation for FormatNetworks
		mockFormatter.EXPECT().FormatNetworks([]*din.Network{}, FormatOptions{}).Return(nil).Times(1)

		// Execute core function being tested
		err := doListNetworks()

		// Validate the result
		assert.NoError(t, err)
	})

	t.Run("Success call, URI flag (no full flag)", func(t *testing.T) {
		resetListGlobalVariables()

		// set CLI value for networkURI
		networkURI = "test://network"
		// set mock expectation for GetNetworkByName
		mockDinClient.EXPECT().GetNetworkByName("test://network").Return(&din.Network{}, nil).Times(1)
		mockFormatter.EXPECT().FormatNetwork(&din.Network{}, FormatOptions{}).Return(nil).Times(1)

		// Execute core function being tested
		err := doListNetworks()

		// Validate the result
		assert.NoError(t, err)
	})

	t.Run("Success call, URI flag (full flag)", func(t *testing.T) {
		resetListGlobalVariables()

		// set CLI value for networkURI
		networkURI = "test://network"
		// set CLI value for networkListFull
		networkListFull = true
		// set mock expectation for GetNetworkByName
		mockDinClient.EXPECT().GetNetworkByName("test://network").Return(&din.Network{}, nil).Times(1)
		formatOptions := FormatOptions{
			ShowMethods:   true,
			ShowProviders: true,
			Verbose:       true,
		}
		mockFormatter.EXPECT().FormatNetwork(&din.Network{}, formatOptions).Return(nil).Times(1)

		// Execute core function being tested
		err := doListNetworks()

		// Validate the result
		assert.NoError(t, err)
	})

	t.Run("Failure call, URI flag with non-existent network", func(t *testing.T) {
		resetListGlobalVariables()

		// set CLI value for networkURI
		networkURI = "test://DOES-NOT-EXIST"
		// set CLI value for networkListFull
		networkListFull = true
		// set mock expectation for GetNetworkByName
		mockDinClient.EXPECT().GetNetworkByName("test://DOES-NOT-EXIST").Return(nil, errors.New("network not found")).Times(1)

		// Execute core function being tested
		err := doListNetworks()

		// Validate the result
		expectedError := fmt.Errorf("there is no network with name '%s' in the registry. Please remove the --uri flag to see all networks available", "test://DOES-NOT-EXIST")
		assert.EqualError(t, err, expectedError.Error())
	})

	t.Run("Success call, full flag", func(t *testing.T) {
		resetListGlobalVariables()

		// set CLI value for networkListFull
		networkListFull = true
		// set mock expectation for GetAllNetworks
		mockDinClient.EXPECT().GetAllNetworks().Return([]*din.Network{}, nil).Times(1)
		formatOptions := FormatOptions{
			ShowMethods:   true,
			ShowProviders: true,
			Verbose:       true,
		}
		mockFormatter.EXPECT().FormatNetworks([]*din.Network{}, formatOptions).Return(nil).Times(1)

		// Execute core function being tested
		err := doListNetworks()

		// Validate the result
		assert.NoError(t, err)
	})
}

func TestSetNetworkStatusCommand(t *testing.T) {
	// Setup mock
	mockCtrl := gomock.NewController(t)
	mockDinClient := din.NewMockIDinClient(mockCtrl)

	// set CLI states
	dinClient = mockDinClient

	// Helper function to reset global variables
	resetSetStatusGlobalVariables := func() {
		networkURI = ""
		networkStatus = din.NetworkStatus("")
		keystorePath = ""
	}

	t.Run("Success call, set status to active", func(t *testing.T) {
		resetSetStatusGlobalVariables()

		// set CLI values
		networkURI = "test://network"
		networkStatus = din.NetworkStatusActive

		// Mock transactor creation
		mockTransactor := &bind.TransactOpts{}
		mockDinClient.EXPECT().CreateAuthorizedTransactor("keystorePath", gomock.Any()).Return(mockTransactor, nil).Times(1)

		// Mock SetNetworkStatus call - returns transaction and error
		mockTx := &types.Transaction{}
		mockDinClient.EXPECT().SetNetworkStatus(mockTransactor, "test://network", din.NetworkStatusActive).Return(mockTx, nil).Times(1)

		// Execute core function being tested
		tx, err := doSetNetworkStatus("keystorePath", "test-password")

		// Validate the result
		assert.NoError(t, err)
		assert.NotNil(t, tx)
	})

	t.Run("Success call, set status to maintenance", func(t *testing.T) {
		resetSetStatusGlobalVariables()

		// set CLI values
		networkURI = "test://network"
		networkStatus = din.NetworkStatusMaintenance

		// Mock transactor creation
		mockTransactor := &bind.TransactOpts{}
		mockDinClient.EXPECT().CreateAuthorizedTransactor("keystorePath", gomock.Any()).Return(mockTransactor, nil).Times(1)

		// Mock SetNetworkStatus call - returns transaction and error
		mockTx := &types.Transaction{}
		mockDinClient.EXPECT().SetNetworkStatus(mockTransactor, "test://network", din.NetworkStatusMaintenance).Return(mockTx, nil).Times(1)

		// Execute core function being tested
		tx, err := doSetNetworkStatus("keystorePath", "test-password")

		// Validate the result
		assert.NoError(t, err)
		assert.NotNil(t, tx)
	})

	t.Run("Failure call, transactor creation fails", func(t *testing.T) {
		resetSetStatusGlobalVariables()

		// set CLI values
		networkURI = "test://network"
		networkStatus = din.NetworkStatusActive

		// Mock transactor creation failure
		mockDinClient.EXPECT().CreateAuthorizedTransactor("keystorePath", gomock.Any()).Return(nil, errors.New("keystore error")).Times(1)

		// Execute core function being tested
		tx, err := doSetNetworkStatus("keystorePath", "test-password")

		// Validate the result
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "keystore error")
		assert.Nil(t, tx)
	})

	t.Run("Failure call, SetNetworkStatus fails", func(t *testing.T) {
		resetSetStatusGlobalVariables()

		// set CLI values
		networkURI = "test://network"
		networkStatus = din.NetworkStatusActive

		// Mock transactor creation
		mockTransactor := &bind.TransactOpts{}
		mockDinClient.EXPECT().CreateAuthorizedTransactor("keystorePath", gomock.Any()).Return(mockTransactor, nil).Times(1)

		// Mock SetNetworkStatus failure
		mockDinClient.EXPECT().SetNetworkStatus(mockTransactor, "test://network", din.NetworkStatusActive).Return(nil, errors.New("network not found")).Times(1)

		// Execute core function being tested
		tx, err := doSetNetworkStatus("keystorePath", "test-password")

		// Validate the result
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "network not found")
		assert.Nil(t, tx)
	})
}

func TestSetNetworkConfigCommand(t *testing.T) {
	// Setup mock
	mockCtrl := gomock.NewController(t)
	mockDinClient := din.NewMockIDinClient(mockCtrl)
	mockFeesEstimator := NewMockIFeesEstimator(mockCtrl)

	// set CLI states
	dinClient = mockDinClient
	feesEstimator = mockFeesEstimator

	// Helper function to reset global variables
	resetSetConfigGlobalVariables := func() {
		networkURI = ""
		networkConfigJsonAsString = ""
		keystorePath = ""
	}

	t.Run("Success call, update healthcheck method", func(t *testing.T) {
		resetSetConfigGlobalVariables()

		// set CLI values
		networkURI = "test://network"
		networkConfigJsonAsString = `{"handler": "starknet"}`
		keystorePath = "keystorePath"

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

		// Mock fees estimator to return proper fee data
		mockFeeData := FeeData{
			GasPrice:  big.NewInt(1000000000), // 1 gwei
			GasFeeCap: big.NewInt(2000000000), // 2 gwei
			GasTipCap: big.NewInt(1000000000), // 1 gwei
		}
		mockFeesEstimator.EXPECT().EstimateFees().Return(mockFeeData, nil).Times(1)

		// Mock transactor creation
		mockTransactor := &bind.TransactOpts{}
		mockDinClient.EXPECT().CreateAuthorizedTransactor("keystorePath", gomock.Any()).Return(mockTransactor, nil).Times(1)

		// Mock SetNetworkConfig call with updated config
		expectedConfig := din.NetworkOperationsConfig{
			Handler:                  "starknet", // Updated
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
		}
		mockTx := &types.Transaction{}
		mockDinClient.EXPECT().SetNetworkConfig(mockTransactor, "test://network", expectedConfig).Return(mockTx, nil).Times(1)

		// Execute core function being tested
		tx, err := doSetNetworkConfig("keystorePath", "test-password")

		// Validate the result
		assert.NoError(t, err)
		assert.NotNil(t, tx)
	})

	t.Run("Success call, update multiple fields", func(t *testing.T) {
		resetSetConfigGlobalVariables()

		// set CLI values
		networkURI = "test://network"
		networkConfigJsonAsString = `{"handler": "starknet", "health_check_interval_sec": 60, "chain_id": "0x5"}`

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

		// Mock fees estimator to return proper fee data
		mockFeeData := FeeData{
			GasPrice:  big.NewInt(1000000000), // 1 gwei
			GasFeeCap: big.NewInt(2000000000), // 2 gwei
			GasTipCap: big.NewInt(1000000000), // 1 gwei
		}
		mockFeesEstimator.EXPECT().EstimateFees().Return(mockFeeData, nil).Times(1)

		// Mock transactor creation
		mockTransactor := &bind.TransactOpts{}
		mockDinClient.EXPECT().CreateAuthorizedTransactor("keystorePath", gomock.Any()).Return(mockTransactor, nil).Times(1)

		// Mock SetNetworkConfig call with updated config
		expectedConfig := din.NetworkOperationsConfig{
			Handler:                  "starknet", // Updated
			HealthcheckIntervalSec:   60,         // Updated
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
			ChainId:                  "0x5", // Updated
		}
		mockTx := &types.Transaction{}
		mockDinClient.EXPECT().SetNetworkConfig(mockTransactor, "test://network", expectedConfig).Return(mockTx, nil).Times(1)

		// Execute core function being tested
		tx, err := doSetNetworkConfig("keystorePath", "test-password")

		// Validate the result
		assert.NoError(t, err)
		assert.NotNil(t, tx)
	})

	t.Run("Failure call, GetNetworkByName fails", func(t *testing.T) {
		resetSetConfigGlobalVariables()

		// set CLI values
		networkURI = "test://DOES-NOT-EXIST"
		networkConfigJsonAsString = `{"handler": "starknet"}`

		// Mock GetNetworkByName failure
		mockDinClient.EXPECT().GetNetworkByName("test://DOES-NOT-EXIST").Return(nil, errors.New("network not found")).Times(1)

		// Execute core function being tested
		tx, err := doSetNetworkConfig("keystorePath", "test-password")

		// Validate the result
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "network not found")
		assert.Nil(t, tx)
	})

	t.Run("Failure call, transactor creation fails", func(t *testing.T) {
		resetSetConfigGlobalVariables()

		// set CLI values
		networkURI = "test://network"
		networkConfigJsonAsString = `{"handler": "starknet"}`

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

		// Mock transactor creation failure
		mockDinClient.EXPECT().CreateAuthorizedTransactor("keystorePath", gomock.Any()).Return(nil, errors.New("keystore error")).Times(1)

		// Execute core function being tested
		tx, err := doSetNetworkConfig("keystorePath", "test-password")

		// Validate the result
		assert.Error(t, err)
		assert.Nil(t, tx)
		assert.Contains(t, err.Error(), "keystore error")
	})

	t.Run("Failure call, SetNetworkConfig fails", func(t *testing.T) {
		resetSetConfigGlobalVariables()

		// set CLI values
		networkURI = "test://network"
		networkConfigJsonAsString = `{"handler": "starknet"}`

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

		// Mock fees estimator to return proper fee data
		mockFeeData := FeeData{
			GasPrice:  big.NewInt(1000000000), // 1 gwei
			GasFeeCap: big.NewInt(2000000000), // 2 gwei
			GasTipCap: big.NewInt(1000000000), // 1 gwei
		}
		mockFeesEstimator.EXPECT().EstimateFees().Return(mockFeeData, nil).Times(1)

		// Mock transactor creation
		mockTransactor := &bind.TransactOpts{}
		mockDinClient.EXPECT().CreateAuthorizedTransactor("keystorePath", gomock.Any()).Return(mockTransactor, nil).Times(1)

		// Mock SetNetworkConfig failure
		expectedConfig := din.NetworkOperationsConfig{
			Handler:                  "starknet", // Updated
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
		}
		mockDinClient.EXPECT().SetNetworkConfig(mockTransactor, "test://network", expectedConfig).Return(nil, errors.New("transaction failed")).Times(1)

		// Execute core function being tested
		tx, err := doSetNetworkConfig("keystorePath", "test-password")

		// Validate the result
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "transaction failed")
		assert.Nil(t, tx)
	})
}
