package dinregistry

import (
	"embed"
	"io/fs"
	"math/big"

	"github.com/pkg/errors"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/jsonrpc"
)

// // Compile time check to ensure that the DinWrapper implements the Din interface
var _ IDinRegistryHandler = &DinRegistryHandler{}
var _ IProviderHandler = &ProviderHandler{}
var _ INetworkServiceHandler = &NetworkServiceHandler{}
var _ INetworkHandler = &NetworkHandler{}

type DinRegistryHandler struct {
	ContractHandler IContractHandler
}

//go:embed abi/din_registry.abi
var abiDinRegistryFS embed.FS

func NewDinRegistryHandler(ethClient *jsonrpc.Client, contractAddress string) (*DinRegistryHandler, error) {
	abiBytes, err := fs.ReadFile(abiDinRegistryFS, ABIDinRegistryPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to NewDinRegistryHandler ReadFile")
	}

	contractHandler, err := NewContractHandler(ethClient, contractAddress, string(abiBytes))
	if err != nil {
		return nil, errors.Wrap(err, "failed call to NewDinRegistryHandler")
	}
	return &DinRegistryHandler{ContractHandler: contractHandler}, nil
}

func (d *DinRegistryHandler) GetNetworkOperationsConfig(network string) (*NetworkConfig, error) {
	networkConfigData, err := d.ContractHandler.Call(GetNetworkOperationsConfig, network)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to DinRegistryHandler GetNetworkOperationsConfig")
	}

	networkConfigStruct, ok := networkConfigData.(map[string]interface{})
	if !ok {
		return nil, errors.New("mismatched type for networkConfigMap")
	}

	_, ok = networkConfigStruct["opsConfig"]
	if !ok {
		return nil, errors.New("unexpected data structure returned in opsConfig")
	}

	networkConfigMapData, ok := networkConfigStruct["opsConfig"].(map[string]interface{})
	if !ok {
		return nil, errors.New("mismatched type for opsConfig")
	}

	return parseNetworkOperationsConfig(networkConfigMapData)
}

func (d *DinRegistryHandler) GetAllNetworkMethodNames(network string) ([]string, error) {
	allMethods, err := d.ContractHandler.Call(GetAllNetworkMethodNames, network)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to DinRegistryHandler GetAllNetworkMethodNames")
	}
	allMethodsListStruct, ok := allMethods.(map[string]interface{})
	if !ok {
		return nil, errors.New("mismatched type for allMethodsListStruct")
	}

	_, ok = allMethodsListStruct["methods"]
	if !ok {
		return nil, errors.New("unexpected data structure returned in GetAllNetworkMethodNames")
	}

	methodList, ok := allMethodsListStruct["methods"].([]string)
	if !ok {
		return nil, errors.New("mismatched type for methodList")
	}

	return methodList, nil
}

func (d *DinRegistryHandler) GetAllNetworkMethods(network string) ([]Method, error) {
	allMethods, err := d.ContractHandler.Call(GetAllNetworkMethods, network)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to DinRegistryHandler GetAllNetworkMethods")
	}

	var methods []Method
	allMethodsStruct, ok := allMethods.(map[string]interface{})
	if !ok {
		return nil, errors.New("mismatched type for allMethodsStruct")
	}

	_, ok = allMethodsStruct["methods"]
	if !ok {
		return nil, errors.New("unexpected data structure returned in GetAllNetworkMethods")
	}

	// Need to convert the raw data into an anonymous struct to access the fields.
	allMethodsStructMap, ok := allMethodsStruct["methods"].([]map[string]interface{})
	if !ok {
		return nil, errors.New("mismatched type for allMethodsStructMap")
	}
	for _, method := range allMethodsStructMap {
		name, ok := method["name"].(string)
		if !ok {
			return nil, errors.New("mismatched types for name")
		}

		bit, ok := method["bit"].(uint8)
		if !ok {
			return nil, errors.New("mismatched type for bit")
		}

		deactivated, ok := method["deactivated"].(bool)
		if !ok {
			return nil, errors.New("mismatched type for deactivated")
		}
		methods = append(methods, Method{
			Name:        name,
			Bit:         bit,
			Deactivated: deactivated,
		})
	}

	return methods, nil
}

func (d *DinRegistryHandler) GetNetworkAddressByName(network string) (*ethgo.Address, error) {
	addressData, err := d.ContractHandler.Call(GetNetworkFromName, network)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to DinRegistryHandler GetNetworkByName")
	}

	addressStruct, ok := addressData.(map[string]interface{})
	if !ok {
		return nil, errors.New("mismatched type for addressStruct GetNetworkByName")
	}

	_, ok = addressStruct["0"]
	if !ok {
		return nil, errors.New("unexpected data structure returned in DinRegistry GetNetworkByName")
	}

	address, ok := addressStruct["0"].(ethgo.Address)
	if !ok {
		return nil, errors.New("mismatched type for serviceAddress")
	}

	return &address, nil
}

func (d *DinRegistryHandler) GetAllNetworkAddresses() ([]ethgo.Address, error) {
	networks, err := d.ContractHandler.Call(GetAllNetworks)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to DinRegistryHandler GetAllNetworks")
	}

	networksStruct, ok := networks.(map[string]interface{})
	if !ok {
		return nil, errors.New("mismatched types for networksStruct")
	}

	_, ok = networksStruct["0"]
	if !ok {
		return nil, errors.New("unexpected data structure returned in GetAllNetworks")
	}

	addressList, ok := networksStruct["0"].([]ethgo.Address)
	if !ok {
		return nil, errors.New("mismatched type for networksList")
	}

	return addressList, nil
}

func (d *DinRegistryHandler) GetNetworkCapabilities(network string) (*big.Int, error) {
	capabilities, err := d.ContractHandler.Call(GetRegNetworkCapabilities, network)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to DinRegistryHandler GetNetworkCapabilities")
	}

	capabilitiesStruct, ok := capabilities.(map[string]interface{})
	if !ok {
		return nil, errors.New("mismatched type for capabilitiesStruct")
	}
	_, ok = capabilitiesStruct["caps"]
	if !ok {
		return nil, errors.New("unexpected data structure returned in GetNetworkCapabilities")
	}

	cap, ok := capabilitiesStruct["caps"].(*big.Int)
	if !ok {
		return nil, errors.New("mismatched type for cap")
	}

	return cap, nil
}

func (d *DinRegistryHandler) GetAllProviders() ([]ProviderHandler, error) {
	allProviders, err := d.ContractHandler.Call(GetAllProviders)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to DinRegistryHandler GetAllProviders Call")
	}

	allProvidersStruct, ok := allProviders.(map[string]interface{})
	if !ok {
		return nil, errors.New("mismatched type for allProvidersStruct")
	}
	_, ok = allProvidersStruct["0"]
	if !ok {
		return nil, errors.New("unexpected data structure returned in GetAllProviders")
	}

	var providers []ProviderHandler
	addressList, ok := allProvidersStruct["0"].([]ethgo.Address)
	if !ok {
		return nil, errors.New("mismatched type for addressList")
	}
	for _, address := range addressList {
		provider, err := NewProviderHandler(d.ContractHandler.GetEthClient(), address.String())
		if err != nil {
			return nil, errors.Wrap(err, "failed call to DinRegistryHandler GetAllProviders NewProvider")
		}
		providers = append(providers, *provider)
	}
	return providers, nil
}

func (d *DinRegistryHandler) GetProvidersByNetwork(network string) ([]ProviderHandler, error) {
	allProviders, err := d.ContractHandler.Call(GetProviders, network)
	if err != nil {
		return nil, err
	}

	allProvidersStruct, ok := allProviders.(map[string]interface{})
	if !ok {
		return nil, errors.New("mismatched type for allProvidersStruct")
	}

	_, ok = allProvidersStruct["0"]
	if !ok {
		return nil, errors.New("unexpected data structure returned in GetAllProviders")
	}

	var providers []ProviderHandler
	addressList, ok := allProvidersStruct["0"].([]ethgo.Address)
	if !ok {
		return nil, errors.New("mismatched type	for addressList")
	}
	for _, address := range addressList {
		provider, err := NewProviderHandler(d.ContractHandler.GetEthClient(), address.String())
		if err != nil {
			return nil, err
		}
		providers = append(providers, *provider)
	}
	return providers, nil
}
