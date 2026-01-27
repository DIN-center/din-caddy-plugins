package din

import (
	"context"
	"fmt"
	"os"
	"strings"

	scm "github.com/DIN-center/din-sc/apps/din-go/pkg/dinregistry/sc_models"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// DinClient is a client for interacting with the on-chain DinRegistry contract
type DinClient struct {
	handler       *scm.DinRegistryHandler
	rpcConnection *ethclient.Client
	logger        *zap.Logger
}

func NewDinClient(logger *zap.Logger, rpcEndpointUrl string, registryContractAddress string) (*DinClient, error) {
	// Create a connection to the Ethereum client
	conn, err := ethclient.Dial(rpcEndpointUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to the Ethereum client: %v", err)
	}

	// Create the registry contract instance
	registry, err := scm.NewDinRegistryHandler(common.HexToAddress(registryContractAddress), conn)
	if err != nil {
		return nil, fmt.Errorf("failed to create registry contract instance: %v", err)
	}

	return &DinClient{
		handler:       registry,
		rpcConnection: conn,
		logger:        logger,
	}, nil
}

// GetLatestBlockNumber retrieves the latest block number for the registry source.
// Returns the latest block number, or an error if the operation fails.
func (d *DinClient) GetLatestBlockNumber() (uint64, error) {
	return d.rpcConnection.BlockNumber(context.Background())
}

// GetRegistryData returns all networks, including their providers and services, in the registry
func (d *DinClient) GetRegistryData() (*DinRegistryData, error) {
	registryData := &DinRegistryData{
		Networks: make(map[string]*Network),
	}

	// Fetch all networks
	networkAddresses, err := d.handler.GetAllNetworks(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to GetAllNetworks")
	}

	for _, networkAddress := range networkAddresses {

		network, err := d.GetNetworkByAddress(networkAddress)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed fetching network from address: %s", networkAddress.Hex()))
		}

		// Add the network to the registry data
		registryData.Networks[network.Name] = network
	}
	return registryData, nil
}

// GetAllNetworks returns a list of all networks in the registry
func (d *DinClient) GetAllNetworks() ([]*Network, error) {
	//Basically we need to call the GetRegistryData function and return the list of networks
	registryData, err := d.GetRegistryData()
	if err != nil {
		return nil, errors.Wrap(err, "failed call to GetRegistryData")
	}

	//Convert the map to a slice
	var networks []*Network
	for _, network := range registryData.Networks {
		networks = append(networks, network)
	}
	return networks, nil
}

func (d *DinClient) GetNetworkByAddress(networkAddress common.Address) (*Network, error) {
	networkHandler, err := scm.NewNetworkHandler(networkAddress, d.rpcConnection)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to NewNetworkHandler")
	}

	networkName, err := networkHandler.GetName(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to GetNetworkMeta")
	}

	networkDescription, err := networkHandler.GetDescription(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to GetNetworkDescription")
	}

	networkOwnerAddress, err := networkHandler.NetworkOwner(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to GetNetworkOwner")
	}

	networkStatusSCM, err := networkHandler.GetNetworkStatus(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to GetNetworkStatus")
	}
	networkStatus, err := NetworkStatusFromCode(networkStatusSCM)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to convert network status")
	}

	capabilities, err := networkHandler.GetCapabilities(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to GetNetworkCapabilities")
	}

	networkConfigSCM, err := networkHandler.GetNetworkOperationsConfig(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to GetNetworkOperationsConfig")
	}

	methodsByName, _, err := d.getNetworkMethodsMapping(networkName)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to getNetworkMethodMappings")
	}

	networkConfig := &NetworkOperationsConfig{
		Handler:                  networkConfigSCM.Handler,
		HealthcheckIntervalSec:   networkConfigSCM.HealthcheckIntervalSec,
		HealthcheckThreshold:     networkConfigSCM.HealthcheckThreshold,
		HealthcheckTimeout:       networkConfigSCM.HealthcheckTimeout,
		BlockLagLimit:            networkConfigSCM.BlockLagLimit,
		BlockJumpLimit:           networkConfigSCM.BlockJumpLimit,
		RequestAttemptCount:      networkConfigSCM.RequestAttemptCount,
		MaxRequestPayloadSizeKb:  networkConfigSCM.MaxRequestPayloadSizeKb,
		RegistryBlockEpoch:       networkConfigSCM.RegistryBlockEpoch,
		ArchiveEnabled:           networkConfigSCM.ArchiveEnabled,
		ProviderBlockHistorySize: networkConfigSCM.ProviderBlockHistorySize,
		NetworkBlockHistorySize:  networkConfigSCM.NetworkBlockHistorySize,
		ChainId:                  networkConfigSCM.ChainId,
	}

	//Get the providers for the network
	providersAddresses, err := d.handler.GetProviders(nil, networkName)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to GetProviders on DinRegistryHandler")
	}

	providers := make(map[string]*Provider)
	for _, providerAddress := range providersAddresses {
		provider, err := d.GetProviderByAddress(providerAddress)
		if err != nil {
			return nil, errors.Wrap(err, "failed fetching provider from address")
		}

		//Filter out all network services that are not the same as the networkURI
		d.filterNetworkServices(provider, networkAddress)
		//If there are no network services, don't add the provider to the list
		if len(provider.NetworkServices) > 0 {
			//Bind the provider to the network
			providers[provider.Name] = provider
		}
	}

	network := &Network{
		Address:       networkAddress.String(),
		Owner:         networkOwnerAddress.String(),
		Name:          networkName,
		Description:   networkDescription,
		Status:        networkStatus,
		ProxyName:     convertNetworkName(networkName),
		Providers:     providers,
		MethodsByName: methodsByName,
		Capabilities:  capabilities,
		NetworkConfig: networkConfig,
	}
	return network, nil
}

func (d *DinClient) getNetworkMethodsMapping(networkName string) (map[string]*Method, map[uint8]*Method, error) {
	methodsSCM, err := d.handler.GetAllNetworkMethods(nil, networkName)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed call to GetAllMethodsByEndpoint")
	}

	methodsByName := make(map[string]*Method)
	methodsByBit := make(map[uint8]*Method)
	for _, methodSCM := range methodsSCM {
		method := Method{
			Name:        methodSCM.Name,
			Bit:         methodSCM.Bit,
			Deactivated: methodSCM.Deactivated,
		}
		methodsByName[method.Name] = &method
		methodsByBit[method.Bit] = &method
	}

	return methodsByName, methodsByBit, nil
}

func (d *DinClient) GetProviderByAddress(providerAddress common.Address) (*Provider, error) {
	providerHandler, err := scm.NewProviderHandler(providerAddress, d.rpcConnection)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to NewProviderHandler")
	}

	name, err := providerHandler.Name(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to Provider Name")
	}

	owner, err := providerHandler.ProviderOwner(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to Provider Owner")
	}

	authConfig, err := providerHandler.AuthConfig(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to GetAuthConfig")
	}

	authConfigType, err := ProviderAuthTypeFromCode(authConfig.Auth)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to ProviderAuthTypeFromCode")
	}

	statusSCM, err := providerHandler.ProviderStatus(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to ProviderStatus")
	}
	status, err := ProviderStatusFromCode(statusSCM)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to getProviderStatus")
	}
	networkServiceAddresses, err := providerHandler.GetAllNetworkServices(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to AllServices")
	}

	networkServices := make(map[string]*NetworkService)
	for _, networkServiceAddress := range networkServiceAddresses {

		networkServiceData, err := d.GetNetworkServiceByAddress(networkServiceAddress)
		if err != nil {
			return nil, errors.Wrap(err, "failed fetching network service from address")
		}
		networkServices[networkServiceData.Url] = networkServiceData

	}
	newProvider := &Provider{
		Address:         providerAddress.String(),
		Name:            name,
		Owner:           owner.String(),
		NetworkServices: networkServices,
		AuthConfig: &ProviderAuthConfig{
			Type:              authConfigType,
			Url:               authConfig.Url,
			ApiKeyPlaceholder: authConfig.ApiKeyPlaceholder,
			UseHeader:         authConfig.UseHeader,
		},
		Status: status,
	}
	return newProvider, nil
}

func (d *DinClient) GetNetworkServiceByAddress(networkServiceAddress common.Address) (*NetworkService, error) {
	networkServiceHandler, err := scm.NewNetworkServiceHandler(networkServiceAddress, d.rpcConnection)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to NewNetworkServiceHandler")
	}

	networkAddress, err := networkServiceHandler.Inetwork(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to GetNetworkAddress")
	}

	url, err := networkServiceHandler.ServiceUrl(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to GetNetworkServiceURL")
	}

	capabilities, err := networkServiceHandler.Capabilities(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to GetCapabilities")
	}

	networkServiceStatusCode, err := networkServiceHandler.GetStatus(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to GetNetworkServiceStatus")
	}
	networkServiceStatus, err := NetworkServiceStatusFromCode(networkServiceStatusCode)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to convert network service status")
	}

	locations, err := networkServiceHandler.GetLocations(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to GetNetworkServiceLocations")
	}

	locationsName, err := mapRawLocationsCodeToName(locations)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to mapRawLocationsCodeToName")
	}

	networkHandler, err := scm.NewNetworkHandler(networkAddress, d.rpcConnection)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to NewNetworkHandler")
	}

	// Retrieve the network name from the network address to enrich the network service
	networkName, err := networkHandler.GetName(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to GetNetworkMeta")
	}

	// Retrieve the methods for the network so we can filter the network service methods
	methodsByName, _, err := d.getNetworkMethodsMapping(networkName)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to getNetworkMethodsMapping")
	}

	// Only methods that are available for the network service (but we have only the string names)
	serviceMethodNames, err := networkServiceHandler.GetAllMethodNames(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to GetMethods")
	}

	//Create a map of only the methods that are available for the network service
	serviceMethods := make(map[string]*Method)
	for _, methodName := range serviceMethodNames {
		serviceMethods[methodName] = methodsByName[methodName]
	}

	networkServiceData := &NetworkService{
		Address:        networkServiceAddress.String(),
		Url:            url,
		Capabilities:   capabilities,
		Status:         networkServiceStatus,
		Locations:      locationsName,
		NetworkAddress: networkAddress.String(),
		NetworkName:    networkName,
		Methods:        serviceMethods,
	}
	return networkServiceData, nil
}

func (d *DinClient) GetNetworkByName(networkName string) (*Network, error) {
	// Fetch all networks
	networkAddress, err := d.handler.GetNetworkFromName(nil, networkName)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to get network from name: %s", networkName))
	}
	network, err := d.GetNetworkByAddress(networkAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to getNetworkFromAddress")
	}
	return network, nil
}

func (d *DinClient) SetNetworkStatus(auth *bind.TransactOpts, networkURI string, networkStatus NetworkStatus) (tx *types.Transaction, err error) {

	networkStatusCode, err := networkStatus.ToCode()
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to convert network status: %s to code", networkStatus))
	}

	// Check if the network exists
	_, err = d.handler.GetNetworkFromName(nil, networkURI)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("network %s does not exist", networkURI))
	}

	tx, err = d.handler.SetNetworkStatus(auth, networkURI, networkStatusCode)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to set network status: %s", networkStatus))
	}

	return tx, nil
}

func (d *DinClient) SetProviderStatus(authTransactor *bind.TransactOpts, providerAddr common.Address, providerStatus ProviderStatus) (tx *types.Transaction, err error) {
	providerStatusCode, err := providerStatus.ToCode()
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to convert provider status %s to code", providerStatus))
	}

	providerHandler, err := scm.NewProviderHandler(providerAddr, d.rpcConnection)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("no provider found for this address: %s", providerAddr.String()))
	}

	tx, err = providerHandler.SetProviderStatus(authTransactor, providerStatusCode)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to set provider status: %s", providerStatus))
	}

	return tx, nil
}

func (d *DinClient) CreateAuthorizedTransactor(keystorePath string, keyPassword string) (*bind.TransactOpts, error) {
	// Get the chain ID
	chainID, err := d.rpcConnection.ChainID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %v", err)
	}

	// Read the key file
	keyFileContent, err := os.ReadFile(keystorePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %v", err)
	}

	// Create the auth
	auth, err := bind.NewTransactorWithChainID(strings.NewReader(string(keyFileContent)), keyPassword, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth: %v", err)
	}

	return auth, nil
}

func (d *DinClient) GetAllProvidersByNetwork(networkURI string) ([]*Provider, error) {
	providersAddres, err := d.handler.GetProviders(nil, networkURI)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to GetProviders")
	}

	networkAddress, err := d.handler.GetNetworkFromName(nil, networkURI)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to GetNetworkFromName")
	}

	providers := make([]*Provider, len(providersAddres))
	for i, providerAddress := range providersAddres {
		provider, err := d.GetProviderByAddress(providerAddress)
		if err != nil {
			return nil, errors.Wrap(err, "failed call to getProviderFromAddress")
		}
		d.filterNetworkServices(provider, networkAddress)
		//If there are no network services, don't add the provider to the list
		if len(provider.NetworkServices) > 0 {
			providers[i] = provider
		}
	}
	return providers, nil
}

func (d *DinClient) GetAllProviders() ([]*Provider, error) {
	providersAddres, err := d.handler.GetAllProviders(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to GetAllProviders")
	}

	providers := make([]*Provider, len(providersAddres))
	for i, providerAddress := range providersAddres {
		provider, err := d.GetProviderByAddress(providerAddress)
		if err != nil {
			return nil, errors.Wrap(err, "failed call to getProviderFromAddress")
		}
		providers[i] = provider
	}
	return providers, nil
}

// Filter out all network services off the provider that are not in the given network
func (d *DinClient) filterNetworkServices(provider *Provider, networkAddress common.Address) {
	for _, networkService := range provider.NetworkServices {
		if !strings.EqualFold(networkService.NetworkAddress, networkAddress.String()) {
			delete(provider.NetworkServices, networkService.Url)
		}
	}
}

func (d *DinClient) RemoveProvider(auth *bind.TransactOpts, providerAddr common.Address) (tx *types.Transaction, err error) {
	tx, err = d.handler.UnregisterProvider(auth, providerAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to RemoveProvider")
	}

	return tx, nil
}

func (d *DinClient) SetNetworkConfig(authTransactor *bind.TransactOpts, networkURI string, newConfig NetworkOperationsConfig) (tx *types.Transaction, err error) {

	newConfigSCM := scm.NetworkOperationsConfig{
		Handler:                  newConfig.Handler,
		HealthcheckIntervalSec:   newConfig.HealthcheckIntervalSec,
		HealthcheckThreshold:     newConfig.HealthcheckThreshold,
		HealthcheckTimeout:       newConfig.HealthcheckTimeout,
		BlockLagLimit:            newConfig.BlockLagLimit,
		BlockJumpLimit:           newConfig.BlockJumpLimit,
		RequestAttemptCount:      newConfig.RequestAttemptCount,
		MaxRequestPayloadSizeKb:  newConfig.MaxRequestPayloadSizeKb,
		RegistryBlockEpoch:       newConfig.RegistryBlockEpoch,
		ArchiveEnabled:           newConfig.ArchiveEnabled,
		ProviderBlockHistorySize: newConfig.ProviderBlockHistorySize,
		NetworkBlockHistorySize:  newConfig.NetworkBlockHistorySize,
		ChainId:                  newConfig.ChainId,
	}

	tx, err = d.handler.SetNetworkOperationsConfig(authTransactor, networkURI, newConfigSCM)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to SetNetworkOperationsConfig")
	}

	return tx, nil
}

func (d *DinClient) RemoveNetworkService(authTransactor *bind.TransactOpts, providerAddr common.Address, networkServiceAddr common.Address) (tx *types.Transaction, err error) {
	// The removal of the network service is done by the provider, and using the network address
	// This is a current limitation of DIN Registry model that accepts only a single service per network
	networkServiceHandler, err := scm.NewNetworkServiceHandler(networkServiceAddr, d.rpcConnection)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed call to NewNetworkServiceHandler for address: %s", networkServiceAddr.String()))
	}
	// Retrieve the network address from the network service address
	networkAddress, err := networkServiceHandler.Inetwork(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to GetNetworkAddress")
	}

	// Ask the provider to remove the network service by using the network address
	providerHandler, err := scm.NewProviderHandler(providerAddr, d.rpcConnection)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed call to NewProviderHandler for address: %s", providerAddr.String()))
	}

	tx, err = providerHandler.RemoveNetworkService(authTransactor, networkAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to RemoveNetworkService")
	}
	return tx, nil
}

func (d *DinClient) SetNetworkServiceStatus(authTransactor *bind.TransactOpts, networkServiceAddr common.Address, networkServiceStatus NetworkServiceStatus) (tx *types.Transaction, err error) {
	networkServiceStatusCode, err := networkServiceStatus.ToCode()
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to convert network service status: %s to code", networkServiceStatus))
	}

	networkServiceHandler, err := scm.NewNetworkServiceHandler(networkServiceAddr, d.rpcConnection)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed call to NewNetworkServiceHandler for address: %s", networkServiceAddr.String()))
	}

	tx, err = networkServiceHandler.SetStatus(authTransactor, networkServiceStatusCode)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to SetNetworkServiceStatus")
	}

	return tx, nil
}

func (d *DinClient) GetEthereumRpcClient() *ethclient.Client {
	return d.rpcConnection
}
