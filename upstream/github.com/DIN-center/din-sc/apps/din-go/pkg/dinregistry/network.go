package dinregistry

import (
	"embed"
	"io/fs"
	"math/big"

	"github.com/pkg/errors"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/jsonrpc"
)

type NetworkHandler struct {
	ContractHandler IContractHandler
}

//go:embed abi/network.abi
var abiNetworkFS embed.FS

// NewNetwork creates a new NetworkHandler
func NewNetworkHandler(ethClient *jsonrpc.Client, contractAddress string) (*NetworkHandler, error) {
	abiBytes, err := fs.ReadFile(abiNetworkFS, ABINetworkPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to NetworkHandler ReadFile")
	}
	abiStr := string(abiBytes)
	contractHandler, err := NewContractHandler(ethClient, contractAddress, abiStr)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to NetworkHandler")
	}
	return &NetworkHandler{ContractHandler: contractHandler}, nil
}

// GetMethodId returns the method id for a given method name associated with the network
func (e *NetworkHandler) GetMethodId(name string) (uint8, error) {
	bit, err := e.ContractHandler.Call(GetMethodId, name)
	if err != nil {
		return 0, errors.Wrap(err, "failed call to NetworkHandler GetMethodId")
	}

	bitStruct, ok := bit.(map[string]interface{})
	if !ok {
		return 0, errors.New("mismatched type for bitStruct")
	}
	_, ok = bitStruct["0"]
	if !ok {
		return 0, errors.New("unexpected data structure returned in NetworkHandler GetMethodId")
	}
	methodID, ok := bitStruct["0"].(uint8)
	if !ok {
		return 0, errors.New("mismatched type for methodID")
	}
	return methodID, nil
}

// GetMethodName returns the method name for a given method id associated with the network
func (e *NetworkHandler) GetMethodName(bit uint8) (string, error) {
	name, err := e.ContractHandler.Call(GetMethodName, bit)
	if err != nil {
		return "", errors.Wrap(err, "failed call to NetworkHandler GetMethodName")
	}
	nameStruct, ok := name.(map[string]interface{})
	if !ok {
		return "", errors.New("mismatched type for nameStruct")
	}
	_, ok = nameStruct["name"]
	if !ok {
		return "", errors.New("unexpected data structure returned in NetworkHandler GetMethodName")
	}
	methodName, ok := nameStruct["name"].(string)
	if !ok {
		return "", errors.New("mismatched type for methodName")
	}
	return methodName, nil
}

// GetCapabilities returns the capabilities of the network
func (e *NetworkHandler) GetCapabilities() (*big.Int, error) {
	caps, err := e.ContractHandler.Call(GetNetworkCapabilities)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to NetworkHandler GetCapabilities")
	}
	capsStruct, ok := caps.(map[string]interface{})
	if !ok {
		return nil, errors.New("mismatched type for capsStruct")
	}
	_, ok = capsStruct["0"]
	if !ok {
		return nil, errors.New("unexpected data structure returned in NetworkHandler GetCapabilities")
	}

	cap, ok := capsStruct["0"].(*big.Int)
	if !ok {
		return nil, errors.New("mismatched type for cap")
	}
	return cap, nil
}

// AllMethods returns a list of methods supported by the network
func (e *NetworkHandler) GetAllMethods() ([]Method, error) {
	allMethods, err := e.ContractHandler.Call(GetAllMethods)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to NetworkHandler AllMethods")
	}

	allMethodsStruct, ok := allMethods.(map[string]interface{})
	if !ok {
		return nil, errors.New("mismatched type for allMethodsStruct")
	}
	_, ok = allMethodsStruct["0"]
	if !ok {
		return nil, errors.New("unexpected data structure returned in NetworkHandler AllMethods")
	}

	var methods []Method
	methodList, ok := allMethodsStruct["0"].([]map[string]interface{})
	if !ok {
		return nil, errors.New("mismatched type for methodList")
	}
	// Need to convert the raw data into an anonymous struct to access the fields.
	for _, method := range methodList {
		name, ok := method["name"].(string)
		if !ok {
			return nil, errors.New("mismatched type for name")
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

// IsMethodSupported returns whether a method is supported by the network
func (e *NetworkHandler) IsMethodSupported(bit uint8) (bool, error) {
	supported, err := e.ContractHandler.Call(IsMethodSupported, bit)
	if err != nil {
		return false, errors.Wrap(err, "failed call to NetworkHandler IsMethodSupported")
	}
	supportedStruct, ok := supported.(map[string]interface{})
	if !ok {
		return false, errors.New("mismatched type for supportedStruct")
	}
	_, ok = supportedStruct["supported"]
	if !ok {
		return false, errors.New("unexpected data structure returned in NetworkHandler IsMethodSupported")
	}

	isMethodSupported, ok := supportedStruct["supported"].(bool)
	if !ok {
		return false, errors.New("mismatched type for isMethodSupported")
	}
	return isMethodSupported, nil
}

// GetNetworkOperationsConfig returns the network operations configuration
func (e *NetworkHandler) GetNetworkOperationsConfig() (*NetworkConfig, error) {
	networkConfigData, err := e.ContractHandler.Call(GetNetworkOperationsConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to DinRegistryHandler GetNetworkOperationsConfig")
	}
	networkConfigStruct, ok := networkConfigData.(map[string]interface{})
	if !ok {
		return nil, errors.New("mismatched type for networkConfigMap")
	}

	_, ok = networkConfigStruct["config"]
	if !ok {
		return nil, errors.New("unexpected data structure returned in config")
	}

	networkConfigMapData, ok := networkConfigStruct["config"].(map[string]interface{})
	if !ok {
		return nil, errors.New("mismatched type for config")
	}

	return parseNetworkOperationsConfig(networkConfigMapData)
}

// GetNetworkMeta returns the network metadata
func (e *NetworkHandler) GetNetworkName() (string, error) {
	name, err := e.ContractHandler.Call(GetNetworkName)
	if err != nil {
		return "", errors.Wrap(err, "failed call to NetworkHandler Owner")
	}
	nameStruct, ok := name.(map[string]interface{})
	if !ok {
		return "", errors.New("mismatched type for nameStruct")
	}
	_, ok = nameStruct["0"]
	if !ok {
		return "", errors.New("unexpected data structure returned in Network Owner")
	}
	nameValue, ok := nameStruct["0"].(string)
	if !ok {
		return "", errors.New("mismatched type for ownerAddress")
	}
	return nameValue, nil
}

func (e *NetworkHandler) GetNetworkOwner() (*ethgo.Address, error) {
	owner, err := e.ContractHandler.Call(GetNetworkOwnerAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to NetworkHandler Owner")
	}
	ownerStruct, ok := owner.(map[string]interface{})
	if !ok {
		return nil, errors.New("mismatched type for ownerStruct")
	}
	_, ok = ownerStruct["0"]
	if !ok {
		return nil, errors.New("unexpected data structure returned in Network Owner")
	}
	ownerAddress, ok := ownerStruct["0"].(ethgo.Address)
	if !ok {
		return nil, errors.New("mismatched type for ownerAddress")
	}
	return &ownerAddress, nil
}

func parseNetworkOperationsConfig(networkConfigMapData map[string]interface{}) (*NetworkConfig, error) {
	healthcheckMethodBit, ok := networkConfigMapData["healthcheckMethodBit"].(uint8)
	if !ok {
		return nil, errors.New("mismatched type for healthcheckMethodBit")
	}

	healthcheckIntervalSec, ok := networkConfigMapData["healthcheckIntervalSec"].(uint8)
	if !ok {
		return nil, errors.New("mismatched type for healthcheckIntervalSec")
	}

	blockLagLimit, ok := networkConfigMapData["blockLagLimit"].(uint8)
	if !ok {
		return nil, errors.New("mismatched type for blockLagLimit")
	}

	requestAttemptCount, ok := networkConfigMapData["requestAttemptCount"].(uint8)
	if !ok {
		return nil, errors.New("mismatched type for requestAttemptCount")
	}

	maxRequestPayloadSizeKb, ok := networkConfigMapData["maxRequestPayloadSizeKb"].(uint16)
	if !ok {
		return nil, errors.New("mismatched type for maxRequestPayloadSizeKb")
	}

	return &NetworkConfig{
		HealthcheckMethodBit:    healthcheckMethodBit,
		HealthcheckIntervalSec:  healthcheckIntervalSec,
		BlockLagLimit:           blockLagLimit,
		RequestAttemptCount:     requestAttemptCount,
		MaxRequestPayloadSizeKb: maxRequestPayloadSizeKb,
	}, nil
}

// GetNetworkStatus return the network status
func (e *NetworkHandler) GetNetworkStatus() (string, error) {
	statusCode, err := e.ContractHandler.Call(GetNetworkStatus)
	if err != nil {
		return "", errors.Wrap(err, "failed call to NetworkHandler GetNetworkStatus")
	}
	statusStruct, ok := statusCode.(map[string]interface{})
	if !ok {
		return "", errors.New("mismatched type for statusStruct")
	}
	_, ok = statusStruct["status"]
	if !ok {
		return "", errors.New("unexpected data structure returned in NetworkHandler GetNetworkStatus")
	}

	networkStatusCode, ok := statusStruct["status"].(uint8)
	if !ok {
		return "", errors.New("mismatched type for networkStatusTypeCode")
	}

	return getNetworkStatus(networkStatusCode)
}

func getNetworkStatus(networkStatusCode uint8) (string, error) {
	switch networkStatusCode {
	case 0:
		return None, nil
	case 1:
		return Onboarding, nil
	case 2:
		return Active, nil
	case 3:
		return Maintenance, nil
	case 4:
		return Decommissioned, nil
	case 5:
		return Retired, nil
	}
	return "", errors.New("Invalid Network Status Code")
}
