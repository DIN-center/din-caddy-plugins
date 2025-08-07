package din

import (
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

type IDinClient interface {
	// GetRegistryData retrieves all registry data including networks and providers.
	// Returns a DinRegistryData struct containing the complete registry state, or an error if the operation fails.
	GetRegistryData() (*DinRegistryData, error)

	// GetAllNetworks retrieves all networks registered in the DIN registry.
	// Returns a slice of Network pointers, or an error if the operation fails.
	GetAllNetworks() ([]*Network, error)

	// GetNetworkByAddress retrieves a specific network by its contract address.
	// Returns a Network pointer if found, or an error if the network doesn't exist or the operation fails.
	GetNetworkByAddress(networkAddress common.Address) (*Network, error)

	// GetProviderByAddress retrieves a specific provider by its contract address.
	// Returns a Provider pointer if found, or an error if the provider doesn't exist or the operation fails.
	GetProviderByAddress(providerAddress common.Address) (*Provider, error)

	// GetNetworkByName retrieves a specific network by its URI.
	// Returns a Network pointer if found, or an error if the network doesn't exist or the operation fails.
	GetNetworkByName(networkURI string) (*Network, error)

	// GetNetworkServiceByAddress retrieves a specific network service by its contract address.
	// Returns a NetworkService pointer if found, or an error if the network service doesn't exist or the operation fails.
	GetNetworkServiceByAddress(networkServiceAddress common.Address) (*NetworkService, error)

	// SetNetworkStatus updates the status of a network in the registry.
	// Returns the submitted transaction so caller can wait for confirmation, or an error if the operation fails.
	SetNetworkStatus(auth *bind.TransactOpts, networkURI string, networkStatus NetworkStatus) (tx *types.Transaction, err error)

	// SetProviderStatus updates the status of a provider in the registry.
	// Returns the submitted transaction so caller can wait for confirmation, or an error if the operation fails.
	SetProviderStatus(authTransactor *bind.TransactOpts, providerAddr common.Address, providerStatus ProviderStatus) (tx *types.Transaction, err error)

	// CreateAuthorizedTransactor creates an authorized transaction signer from keystore credentials.
	// Returns a TransactOpts pointer for signing transactions, or an error if authentication fails.
	CreateAuthorizedTransactor(keystorePath, password string) (*bind.TransactOpts, error)

	// GetAllProvidersByNetwork retrieves all providers that serve a specific network.
	// Returns a slice of Provider pointers filtered by network, or an error if the operation fails.
	GetAllProvidersByNetwork(networkURI string) ([]*Provider, error)

	// GetAllProviders retrieves all providers registered in the DIN registry.
	// Returns a slice of Provider pointers, or an error if the operation fails.
	GetAllProviders() ([]*Provider, error)

	// RemoveProvider removes a provider from the registry by its contract address.
	// Returns the submitted transaction so caller can wait for confirmation, or an error if the operation fails.
	RemoveProvider(auth *bind.TransactOpts, providerAddr common.Address) (tx *types.Transaction, err error)

	// SetNetworkConfig updates the operational configuration of a network in the registry.
	// Returns the submitted transaction so caller can wait for confirmation, or an error if the operation fails.
	SetNetworkConfig(authTransactor *bind.TransactOpts, networkURI string, newConfig NetworkOperationsConfig) (tx *types.Transaction, err error)

	// RemoveNetworkService removes a network service from the provider's list of network services.
	// Returns the submitted transaction so caller can wait for confirmation, or an error if the operation fails.
	RemoveNetworkService(auth *bind.TransactOpts, providerAddr common.Address, networkServiceAddr common.Address) (tx *types.Transaction, err error)

	// SetNetworkServiceStatus updates the status of a network service in the registry.
	// Returns the submitted transaction so caller can wait for confirmation, or an error if the operation fails.
	SetNetworkServiceStatus(auth *bind.TransactOpts, networkServiceAddr common.Address, networkServiceStatus NetworkServiceStatus) (tx *types.Transaction, err error)

	// GetEthereumRpcClient returns the Ethereum RPC client.
	GetEthereumRpcClient() *ethclient.Client
}
