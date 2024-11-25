package din

import (
	"strconv"

	din "github.com/DIN-center/din-sc/apps/din-go/pkg/dinregistry"
	"github.com/pkg/errors"
	"github.com/umbracle/ethgo/jsonrpc"
	"go.uber.org/zap"
)

type DinClient struct {
	DinRegistry din.IDinRegistryHandler
	ethClient   *jsonrpc.Client
	logger      *zap.Logger
}

func NewDinClient(logger *zap.Logger, rpcEndpointUrl string, registryContractAddress string) (*DinClient, error) {
	ethClient, err := setupEthClient(rpcEndpointUrl)
	if err != nil {
		return nil, errors.Wrap(err, "error calling setupWeb3Client")
	}
	dinContractAddress := getDinRegistryContractAddress(registryContractAddress)
	dinRegistry, err := din.NewDinRegistryHandler(ethClient, dinContractAddress)
	if err != nil {
		return nil, errors.Wrap(err, "error creating DinRegistry")
	}
	return &DinClient{
		DinRegistry: dinRegistry,
		ethClient:   ethClient,
	}, nil
}

func (d *DinClient) GetRegistryData() (*DinRegistryData, error) {
	registryData := &DinRegistryData{
		Networks: make(map[string]*Network),
	}

	// Get A list of Network Addresses
	networkAddresses, err := d.DinRegistry.GetAllNetworkAddresses()
	if err != nil {
		return nil, errors.Wrap(err, "failed call to GetAllNetworks")
	}

	for _, networkAddress := range networkAddresses {

		networkHandler, err := din.NewNetworkHandler(d.ethClient, networkAddress.String())
		if err != nil {
			return nil, errors.Wrap(err, "failed call to NewNetworkHandler")
		}

		networkName, err := networkHandler.GetNetworkName()
		if err != nil {
			return nil, errors.Wrap(err, "failed call to GetNetworkMeta")
		}

		networkStatus, err := networkHandler.GetNetworkStatus()
		if err != nil {
			return nil, errors.Wrap(err, "failed call to GetNetworkStatus")
		}

		capabilities, err := d.DinRegistry.GetNetworkCapabilities(networkName)
		if err != nil {
			return nil, errors.Wrap(err, "failed call to GetNetworkCapabilities")
		}

		networkConfig, err := d.DinRegistry.GetNetworkOperationsConfig(networkName)
		if err != nil {
			return nil, errors.Wrap(err, "failed call to GetNetworkOperationsConfig")
		}

		registryData.Networks[networkName] = &Network{
			Address:       networkAddress.String(),
			Name:          networkName,
			Status:        networkStatus,
			ProxyName:     convertNetworkName(networkName),
			Providers:     make(map[string]*Provider),
			Methods:       make(map[string]*din.Method),
			Capabilities:  capabilities,
			NetworkConfig: networkConfig,
		}

		methods, err := d.DinRegistry.GetAllNetworkMethods(networkName)
		if err != nil {
			return nil, errors.Wrap(err, "failed call to GetAllMethodsByEndpoint")
		}

		for _, method := range methods {
			registryData.Networks[networkName].Methods[method.Name] = &method
		}

		providers, err := d.DinRegistry.GetProvidersByNetwork(networkName)
		if err != nil {
			return nil, errors.Wrap(err, "failed call to GetProvidersByNetwork")
		}

		for _, provider := range providers {
			name, err := provider.GetName()
			if err != nil {
				return nil, errors.Wrap(err, "failed call to Provider Name")
			}

			owner, err := provider.GetProviderOwner()
			if err != nil {
				return nil, errors.Wrap(err, "failed call to Provider Owner")
			}

			authConfig, err := provider.GetAuthConfig()
			if err != nil {
				return nil, errors.Wrap(err, "failed call to GetAuthConfig")
			}

			providerAddress := provider.ContractHandler.GetContractAddress()

			newProvider := &Provider{
				Address:         providerAddress.String(),
				Name:            name,
				Owner:           owner.String(),
				NetworkServices: make(map[string]*NetworkService),
				AuthConfig:      authConfig,
			}

			registryData.Networks[networkName].Providers[name] = newProvider

			networkServiceAddresses, err := provider.GetAllNetworkServiceAddresses()
			if err != nil {
				return nil, errors.Wrap(err, "failed call to AllServices")
			}

			for _, networkServiceAddress := range networkServiceAddresses {
				networkServiceHandler, err := din.NewNetworkServiceHandler(d.ethClient, networkServiceAddress.String())
				if err != nil {
					return nil, errors.Wrap(err, "failed call to NewNetworkServiceHandler")
				}

				networkAddress, err := networkServiceHandler.GetNetworkAddress()
				if err != nil {
					return nil, errors.Wrap(err, "failed call to GetNetworkAddress")
				}

				// Check if the network services's network address is the same as the network address we are looking for
				if networkHandler.ContractHandler.GetContractAddress().String() != networkAddress.String() {
					continue
				}

				url, err := networkServiceHandler.GetNetworkServiceURL()
				if err != nil {
					return nil, errors.Wrap(err, "failed call to GetNetworkServiceURL")
				}

				capabilities, err := networkServiceHandler.GetCapabilities()
				if err != nil {
					return nil, errors.Wrap(err, "failed call to GetCapabilities")
				}

				networkServiceStatus, err := networkServiceHandler.GetNetworkServiceStatus()
				if err != nil {
					return nil, errors.Wrap(err, "failed call to GetNetworkServiceStatus")
				}
				networkServiceData := &NetworkService{
					Address:      networkServiceAddress.String(),
					Url:          url,
					Capabilities: capabilities,
					Status:       networkServiceStatus,
					// Methods: make(map[string]*din.Method),
				}
				registryData.Networks[networkName].Providers[name].NetworkServices[networkServiceData.Url] = networkServiceData
				// TODO: will include for method based routing
				// serviceHandler, err := din.NewNetworkServiceHandler(d.EthClient, service.Address)
				// if err != nil {
				// 	return nil, errors.Wrap(err, "failed call to NewNetworkServiceHandler")
				// }

				// serviceMethods, err := serviceHandler.AllMethods()
				// if err != nil {
				// 	return nil, errors.Wrap(err, "failed call to AllMethods")
				// }

				// for _, method := range serviceMethods {
				// 	registryData.Networks[network.Name].Methods[method.Name] = method
				// }
			}
		}
	}
	return registryData, nil
}

func (d *DinClient) GetNetworkMethodNameByBit(networkName string, bit uint8) (string, error) {
	networkAddress, err := d.DinRegistry.GetNetworkAddressByName(networkName)
	if err != nil {
		return "", errors.Wrap(err, "failed call to GetNetworkAddressByName")
	}

	networkHandler, err := din.NewNetworkHandler(d.ethClient, networkAddress.String())
	if err != nil {
		return "", errors.Wrap(err, "failed call to NewNetworkHandler")
	}

	methodName, err := networkHandler.GetMethodName(bit)
	if err != nil {
		return "", errors.Wrap(err, "failed call to GetMethodName")
	}

	return methodName, nil
}

func (d *DinClient) GetNetworkServiceMethods(networkServiceAddress string) ([]*string, error) {
	networkServiceHandler, err := din.NewNetworkServiceHandler(d.ethClient, networkServiceAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to NewNetworkServiceHandler")
	}

	methodNames, err := networkServiceHandler.GetAllMethodNames()
	if err != nil {
		return nil, errors.Wrap(err, "failed call to GetAllMethodNames")
	}

	var methods []*string
	for _, methodName := range methodNames {
		methods = append(methods, &methodName)
	}
	return methods, nil
}

func (d *DinClient) GetEthClient() *jsonrpc.Client {
	return d.ethClient
}

// GetLatestBlockNumber returns the latest block number for the registry
func (d *DinClient) GetLatestBlockNumber() (uint64, error) {
	var hex string
	err := d.ethClient.Call("eth_blockNumber", &hex)
	if err != nil {
		return 0, errors.Wrap(err, "failed call to eth_blockNumber")
	}

	// Convert the hexadecimal string to an int64
	blockNumber, err := strconv.ParseUint(hex[2:], 16, 64)
	if err != nil {
		return 0, errors.Wrap(err, "Error converting block number")
	}
	return blockNumber, nil
}
