package dincli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestProviderCommand(t *testing.T) {
	t.Run("provider command has subcommands", func(t *testing.T) {
		commands := providersCmd.Commands()
		commandNames := make([]string, len(commands))
		for i, cmd := range commands {
			commandNames[i] = cmd.Name()
		}

		assert.Contains(t, commandNames, "list")
		assert.Contains(t, commandNames, "remove")
		assert.Contains(t, commandNames, "set-status")
		assert.Contains(t, commandNames, "set-service-status")
		assert.Contains(t, commandNames, "remove-service")
	})
}

func TestListProvidersCommand(t *testing.T) {
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
		providerName = ""
	}

	t.Run("Success call, no flags - list all providers", func(t *testing.T) {
		resetListGlobalVariables()

		// set mock expectation for GetAllProviders
		mockDinClient.EXPECT().GetAllProviders().Return([]*din.Provider{}, nil).Times(1)
		// set mock expectation for FormatProviders
		mockFormatter.EXPECT().FormatProviders([]*din.Provider{}, FormatOptions{Verbose: false}).Return(nil).Times(1)

		// Execute core function being tested
		err := doListProviders()

		// Validate the result
		assert.NoError(t, err)
	})

	t.Run("Success call, network-uri flag - list providers by network", func(t *testing.T) {
		resetListGlobalVariables()

		// set CLI value for networkURI
		networkURI = "test://network"
		// set mock expectation for GetAllProvidersByNetwork
		mockDinClient.EXPECT().GetAllProvidersByNetwork("test://network").Return([]*din.Provider{}, nil).Times(1)
		mockFormatter.EXPECT().FormatProviders([]*din.Provider{}, FormatOptions{Verbose: false}).Return(nil).Times(1)

		// Execute core function being tested
		err := doListProviders()

		// Validate the result
		assert.NoError(t, err)
	})

	t.Run("Success call, name flag - list providers with name filter", func(t *testing.T) {
		resetListGlobalVariables()

		// set CLI value for providerName
		providerName = "test-provider"

		// set mock expectation for GetAllProviders
		allProviders := []*din.Provider{
			{Name: "test-provider"},
			{Name: "test-provider-2"},
		}
		mockDinClient.EXPECT().GetAllProviders().Return(allProviders, nil).Times(1)
		filteredProviders := []*din.Provider{
			{Name: "test-provider"},
		}
		mockFormatter.EXPECT().FormatProviders(filteredProviders, FormatOptions{Verbose: true}).Return(nil).Times(1)

		// Execute core function being tested
		err := doListProviders()

		// Validate the result
		assert.NoError(t, err)
	})

	t.Run("Success call, network-uri and name flags - list providers by network with name filter", func(t *testing.T) {
		resetListGlobalVariables()

		// set CLI values
		networkURI = "test://network"
		providerName = "test-provider"
		// set mock expectation for GetAllProvidersByNetwork
		mockDinClient.EXPECT().GetAllProvidersByNetwork("test://network").Return([]*din.Provider{}, nil).Times(1)
		mockFormatter.EXPECT().FormatProviders([]*din.Provider{}, FormatOptions{Verbose: true}).Return(nil).Times(1)

		// Execute core function being tested
		err := doListProviders()

		// Validate the result
		assert.NoError(t, err)
	})

	t.Run("Failure call, GetAllProviders fails", func(t *testing.T) {
		resetListGlobalVariables()

		// set mock expectation for GetAllProviders failure
		mockDinClient.EXPECT().GetAllProviders().Return(nil, errors.New("database error")).Times(1)

		// Execute core function being tested
		err := doListProviders()

		// Validate the result
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database error")
	})

	t.Run("Failure call, GetAllProvidersByNetwork fails", func(t *testing.T) {
		resetListGlobalVariables()

		// set CLI value for networkURI
		networkURI = "test://network"
		// set mock expectation for GetAllProvidersByNetwork failure
		mockDinClient.EXPECT().GetAllProvidersByNetwork("test://network").Return(nil, errors.New("network not found")).Times(1)

		// Execute core function being tested
		err := doListProviders()

		// Validate the result
		expectedError := fmt.Errorf("there is no provider registered for this network")
		assert.EqualError(t, err, expectedError.Error())
	})
}

func TestSetProviderStatusCommand(t *testing.T) {
	// Setup mock
	mockCtrl := gomock.NewController(t)
	mockDinClient := din.NewMockIDinClient(mockCtrl)

	// set CLI states
	dinClient = mockDinClient
	feesEstimator = &MockFeesEstimator{}

	// Helper function to reset global variables
	resetSetStatusGlobalVariables := func() {
		providerAddr = ""
		providerStatus = din.ProviderStatus("")
	}

	t.Run("Success call, set status to active", func(t *testing.T) {
		resetSetStatusGlobalVariables()

		// set CLI values
		providerAddr = "0x1234567890123456789012345678901234567890"
		providerStatus = din.ProviderStatusActive

		// Mock transactor creation
		mockTransactor := &bind.TransactOpts{}
		mockDinClient.EXPECT().CreateAuthorizedTransactor("keystorePath", gomock.Any()).Return(mockTransactor, nil).Times(1)

		// Mock SetProviderStatus call
		expectedAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
		mockDinClient.EXPECT().SetProviderStatus(mockTransactor, expectedAddr, din.ProviderStatusActive).Return(&types.Transaction{}, nil).Times(1)

		// Execute core function being tested
		tx, err := doSetProviderStatus("keystorePath", "test-password", expectedAddr)

		// Validate the result
		assert.NoError(t, err)
		assert.NotNil(t, tx)
	})

	t.Run("Success call, set status to maintenance", func(t *testing.T) {
		resetSetStatusGlobalVariables()

		// set CLI values
		providerAddr = "0x1234567890123456789012345678901234567890"
		providerStatus = din.ProviderStatusMaintenance

		// Mock transactor creation
		mockTransactor := &bind.TransactOpts{}
		mockDinClient.EXPECT().CreateAuthorizedTransactor("keystorePath", gomock.Any()).Return(mockTransactor, nil).Times(1)

		// Mock SetProviderStatus call
		expectedAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
		// Mock the transaction
		mockDinClient.EXPECT().SetProviderStatus(mockTransactor, expectedAddr, din.ProviderStatusMaintenance).Return(&types.Transaction{}, nil).Times(1)

		// Execute core function being tested
		tx, err := doSetProviderStatus("keystorePath", "test-password", expectedAddr)

		// Validate the result
		assert.NoError(t, err)
		assert.NotNil(t, tx)
	})

	t.Run("Failure call, transactor creation fails", func(t *testing.T) {
		resetSetStatusGlobalVariables()

		// set CLI values
		providerAddr = "0x1234567890123456789012345678901234567890"
		providerStatus = din.ProviderStatusActive

		// Mock transactor creation failure
		mockDinClient.EXPECT().CreateAuthorizedTransactor("keystorePath", gomock.Any()).Return(nil, errors.New("keystore error")).Times(1)

		// Execute core function being tested
		expectedAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
		tx, err := doSetProviderStatus("keystorePath", "test-password", expectedAddr)

		// Validate the result
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "keystore error")
		assert.Nil(t, tx)
	})

	t.Run("Failure call, SetProviderStatus fails", func(t *testing.T) {
		resetSetStatusGlobalVariables()

		// set CLI values
		providerAddr = "0x1234567890123456789012345678901234567890"
		providerStatus = din.ProviderStatusActive

		// Mock transactor creation
		mockTransactor := &bind.TransactOpts{}
		mockDinClient.EXPECT().CreateAuthorizedTransactor("keystorePath", gomock.Any()).Return(mockTransactor, nil).Times(1)

		// Mock SetProviderStatus failure
		expectedAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
		mockDinClient.EXPECT().SetProviderStatus(mockTransactor, expectedAddr, din.ProviderStatusActive).Return(nil, errors.New("provider not found")).Times(1)

		// Execute core function being tested
		tx, err := doSetProviderStatus("keystorePath", "test-password", expectedAddr)

		// Validate the result
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider not found")
		assert.Nil(t, tx)
	})
}

func TestRemoveProviderCommand(t *testing.T) {
	// Setup mock
	mockCtrl := gomock.NewController(t)
	mockDinClient := din.NewMockIDinClient(mockCtrl)

	// set CLI states
	dinClient = mockDinClient
	feesEstimator = &MockFeesEstimator{}
	// Helper function to reset global variables
	resetRemoveGlobalVariables := func() {
		providerAddr = ""
	}

	t.Run("Success call, remove provider", func(t *testing.T) {
		resetRemoveGlobalVariables()

		// set CLI values
		providerAddr = "0x1234567890123456789012345678901234567890"

		// Mock transactor creation
		mockTransactor := &bind.TransactOpts{}
		mockDinClient.EXPECT().CreateAuthorizedTransactor("keystorePath", gomock.Any()).Return(mockTransactor, nil).Times(1)

		// Mock RemoveProvider call
		expectedAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
		mockDinClient.EXPECT().RemoveProvider(mockTransactor, expectedAddr).Return(&types.Transaction{}, nil).Times(1)

		// Execute core function being tested
		tx, err := doRemoveProvider("keystorePath", "test-password", expectedAddr)

		// Validate the result
		assert.NoError(t, err)
		assert.NotNil(t, tx)
	})

	t.Run("Failure call, transactor creation fails", func(t *testing.T) {
		resetRemoveGlobalVariables()

		// set CLI values
		providerAddr = "0x1234567890123456789012345678901234567890"

		// Mock transactor creation failure
		mockDinClient.EXPECT().CreateAuthorizedTransactor("keystorePath", gomock.Any()).Return(nil, errors.New("keystore error")).Times(1)

		// Execute core function being tested
		expectedAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
		tx, err := doRemoveProvider("keystorePath", "test-password", expectedAddr)

		// Validate the result
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "keystore error")
		assert.Nil(t, tx)
	})

	t.Run("Failure call, RemoveProvider fails", func(t *testing.T) {
		resetRemoveGlobalVariables()

		// set CLI values
		providerAddr = "0x1234567890123456789012345678901234567890"

		// Mock transactor creation
		mockTransactor := &bind.TransactOpts{}
		mockDinClient.EXPECT().CreateAuthorizedTransactor("keystorePath", gomock.Any()).Return(mockTransactor, nil).Times(1)

		// Mock RemoveProvider failure
		expectedAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
		mockDinClient.EXPECT().RemoveProvider(mockTransactor, expectedAddr).Return(nil, errors.New("provider not found")).Times(1)

		// Execute core function being tested
		tx, err := doRemoveProvider("keystorePath", "test-password", expectedAddr)

		// Validate the result
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider not found")
		assert.Nil(t, tx)
	})
}

func TestRemoveNetworkServiceCommand(t *testing.T) {
	// Setup mock
	mockCtrl := gomock.NewController(t)
	mockDinClient := din.NewMockIDinClient(mockCtrl)

	// set CLI states
	dinClient = mockDinClient
	feesEstimator = &MockFeesEstimator{}
	// Helper function to reset global variables
	resetRemoveServiceGlobalVariables := func() {
		providerAddr = ""
		networkServiceAddr = ""
	}

	t.Run("Success call, remove network service", func(t *testing.T) {
		resetRemoveServiceGlobalVariables()

		// set CLI values
		providerAddr = "0x1234567890123456789012345678901234567890"
		networkServiceAddr = "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"

		// Mock transactor creation
		mockTransactor := &bind.TransactOpts{}
		mockDinClient.EXPECT().CreateAuthorizedTransactor("keystorePath", gomock.Any()).Return(mockTransactor, nil).Times(1)

		// Mock RemoveNetworkService
		expectedProviderAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
		expectedServiceAddr := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd")
		mockTx := &types.Transaction{}
		mockDinClient.EXPECT().RemoveNetworkService(mockTransactor, expectedProviderAddr, expectedServiceAddr).Return(mockTx, nil).Times(1)

		// Execute core function being tested
		tx, err := doRemoveNetworkService("keystorePath", "test-password", expectedProviderAddr, expectedServiceAddr)

		// Validate the result
		assert.NoError(t, err)
		assert.NotNil(t, tx)
	})

	t.Run("Failure call, transactor creation fails", func(t *testing.T) {
		resetRemoveServiceGlobalVariables()

		// set CLI values
		providerAddr = "0x1234567890123456789012345678901234567890"
		networkServiceAddr = "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"

		// Mock transactor creation failure
		mockDinClient.EXPECT().CreateAuthorizedTransactor("keystorePath", gomock.Any()).Return(nil, errors.New("keystore error")).Times(1)

		// Execute core function being tested
		expectedProviderAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
		expectedServiceAddr := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd")
		tx, err := doRemoveNetworkService("keystorePath", "test-password", expectedProviderAddr, expectedServiceAddr)

		// Validate the result
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "keystore error")
		assert.Nil(t, tx)
	})

	t.Run("Failure call, RemoveNetworkService fails", func(t *testing.T) {
		resetRemoveServiceGlobalVariables()

		// set CLI values
		providerAddr = "0x1234567890123456789012345678901234567890"
		networkServiceAddr = "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"

		// Mock transactor creation
		mockTransactor := &bind.TransactOpts{}
		mockDinClient.EXPECT().CreateAuthorizedTransactor("keystorePath", gomock.Any()).Return(mockTransactor, nil).Times(1)

		// Mock RemoveNetworkService failure
		expectedProviderAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
		expectedServiceAddr := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd")
		mockDinClient.EXPECT().RemoveNetworkService(mockTransactor, expectedProviderAddr, expectedServiceAddr).Return(nil, errors.New("service not found")).Times(1)

		// Execute core function being tested
		tx, err := doRemoveNetworkService("keystorePath", "test-password", expectedProviderAddr, expectedServiceAddr)

		// Validate the result
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "service not found")
		assert.Nil(t, tx)
	})
}

func TestSetNetworkServiceStatusCommand(t *testing.T) {
	// Setup mock
	mockCtrl := gomock.NewController(t)
	mockDinClient := din.NewMockIDinClient(mockCtrl)

	// set CLI states
	dinClient = mockDinClient

	// Helper function to reset global variables
	resetSetServiceStatusGlobalVariables := func() {
		networkServiceAddr = ""
		networkServiceStatus = din.NetworkServiceStatusNone
	}

	t.Run("Success call, set network service status", func(t *testing.T) {
		resetSetServiceStatusGlobalVariables()

		// set CLI values
		networkServiceAddr = "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"
		networkServiceStatus = din.NetworkServiceStatusActive

		// Mock transactor creation
		mockTransactor := &bind.TransactOpts{}
		mockDinClient.EXPECT().CreateAuthorizedTransactor("keystorePath", gomock.Any()).Return(mockTransactor, nil).Times(1)

		// Mock SetNetworkServiceStatus
		expectedServiceAddr := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd")
		mockTx := &types.Transaction{}
		mockDinClient.EXPECT().SetNetworkServiceStatus(mockTransactor, expectedServiceAddr, din.NetworkServiceStatusActive).Return(mockTx, nil).Times(1)

		// Execute core function being tested
		tx, err := doSetNetworkServiceStatus("keystorePath", "test-password", expectedServiceAddr)

		// Validate the result
		assert.NoError(t, err)
		assert.NotNil(t, tx)
	})

	t.Run("Failure call, transactor creation fails", func(t *testing.T) {
		resetSetServiceStatusGlobalVariables()

		// set CLI values
		networkServiceAddr = "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"
		networkServiceStatus = din.NetworkServiceStatusActive

		// Mock transactor creation failure
		mockDinClient.EXPECT().CreateAuthorizedTransactor("keystorePath", gomock.Any()).Return(nil, errors.New("keystore error")).Times(1)

		// Execute core function being tested
		expectedServiceAddr := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd")
		tx, err := doSetNetworkServiceStatus("keystorePath", "test-password", expectedServiceAddr)

		// Validate the result
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "keystore error")
		assert.Nil(t, tx)
	})

	t.Run("Failure call, SetNetworkServiceStatus fails", func(t *testing.T) {
		resetSetServiceStatusGlobalVariables()

		// set CLI values
		networkServiceAddr = "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"
		networkServiceStatus = din.NetworkServiceStatusActive

		// Mock transactor creation
		mockTransactor := &bind.TransactOpts{}
		mockDinClient.EXPECT().CreateAuthorizedTransactor("keystorePath", gomock.Any()).Return(mockTransactor, nil).Times(1)

		// Mock SetNetworkServiceStatus failure
		expectedServiceAddr := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd")
		mockDinClient.EXPECT().SetNetworkServiceStatus(mockTransactor, expectedServiceAddr, din.NetworkServiceStatusActive).Return(nil, errors.New("status update failed")).Times(1)

		// Execute core function being tested
		tx, err := doSetNetworkServiceStatus("keystorePath", "test-password", expectedServiceAddr)

		// Validate the result
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "status update failed")
		assert.Nil(t, tx)
	})
}
