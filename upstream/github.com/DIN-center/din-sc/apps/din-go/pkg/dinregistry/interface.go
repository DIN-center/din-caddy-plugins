package dinregistry

import (
	"math/big"

	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/jsonrpc"
)

type IDinRegistryHandler interface {
	GetNetworkOperationsConfig(network string) (*NetworkConfig, error)
	GetAllNetworkMethodNames(network string) ([]string, error)
	GetAllNetworkMethods(network string) ([]Method, error)
	GetNetworkAddressByName(network string) (*ethgo.Address, error)
	GetAllNetworkAddresses() ([]ethgo.Address, error)
	GetNetworkCapabilities(network string) (*big.Int, error)
	GetAllProviders() ([]ProviderHandler, error)
	GetProvidersByNetwork(network string) ([]ProviderHandler, error)
}

type INetworkHandler interface {
	GetMethodId(name string) (uint8, error)
	GetMethodName(bit uint8) (string, error)
	GetCapabilities() (*big.Int, error)
	GetAllMethods() ([]Method, error)
	IsMethodSupported(bit uint8) (bool, error)
	GetNetworkOperationsConfig() (*NetworkConfig, error)
	GetNetworkName() (string, error)
	GetNetworkOwner() (*ethgo.Address, error)
	GetNetworkStatus() (string, error)
}

type INetworkServiceHandler interface {
	GetAllMethodNames() ([]string, error)
	GetCapabilities() (*big.Int, error)
	IsMethodSupported(bit uint8) (bool, error)
	GetNetworkServiceURL() (string, error)
	GetNetworkAddress() (*ethgo.Address, error)
	GetNetworkServiceStatus() (string, error)
}

type IProviderHandler interface {
	GetName() (string, error)
	GetProviderOwner() (*ethgo.Address, error)
	GetProviderStatus() (string, error)
	GetAllNetworkServiceAddresses() ([]ethgo.Address, error)
	GetAuthConfig() (*NetworkServiceAuthConfig, error)
}

type IContractHandler interface {
	Call(method string, args ...interface{}) (interface{}, error)
	GetEthClient() *jsonrpc.Client
	GetContractAddress() *ethgo.Address
}
