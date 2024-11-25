package dinregistry

import (
	"embed"
	"io/fs"
	"math/big"

	"github.com/pkg/errors"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/jsonrpc"
)

type NetworkServiceHandler struct {
	ContractHandler IContractHandler
}

//go:embed abi/network_service.abi
var abiNetworkServiceFS embed.FS

// NewNetworkServiceHandler creates a new ServiceHandler service struct
func NewNetworkServiceHandler(ethClient *jsonrpc.Client, contractAddress string) (*NetworkServiceHandler, error) {
	abiBytes, err := fs.ReadFile(abiNetworkServiceFS, ABIServicePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to NewService ReadFile")
	}

	contractHandler, err := NewContractHandler(ethClient, contractAddress, string(abiBytes))
	if err != nil {
		return nil, errors.Wrap(err, "failed call to NewService")
	}
	return &NetworkServiceHandler{contractHandler}, nil
}

// GetAllNetworkServiceMethodNames returns a list of method names supported by the network service
func (n *NetworkServiceHandler) GetAllMethodNames() ([]string, error) {
	methods, err := n.ContractHandler.Call(GetAllMethodNames)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to ServiceHandler GetAllMethodNames")
	}

	methodsStruct, ok := methods.(map[string]interface{})
	if !ok {
		return nil, errors.New("mismatched type for methodsStruct")
	}
	_, ok = methodsStruct["methods"]
	if !ok {
		return nil, errors.New("unexpected data structure returned in ServiceHandler ListMethods")
	}
	methodList, ok := methodsStruct["methods"].([]string)
	if !ok {
		return nil, errors.New("mismatched type for methodList")
	}
	return methodList, nil
}

// GetCapabilities returns the capabilities of the service
func (n *NetworkServiceHandler) GetCapabilities() (*big.Int, error) {
	caps, err := n.ContractHandler.Call(GetNetworkServiceCapabilities)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to ServiceHandler GetCapabilities")
	}

	capsStruct, ok := caps.(map[string]interface{})
	if !ok {
		return nil, errors.New("mismatched type for capsStruct")
	}
	_, ok = capsStruct["0"]
	if !ok {
		return nil, errors.New("unexpected data structure returned in ServiceHandler GetCapabilities")
	}

	cap, ok := capsStruct["0"].(*big.Int)
	if !ok {
		return nil, errors.New("mismatched type for cap")
	}
	return cap, nil
}

// IsMethodSupported returns whether a method is supported by the network service
func (n *NetworkServiceHandler) IsMethodSupported(bit uint8) (bool, error) {
	supported, err := n.ContractHandler.Call(IsMethodSupported, bit)
	if err != nil {
		return false, errors.Wrap(err, "failed call to ServiceHandler IsMethodSupported")
	}
	supportedStruct, ok := supported.(map[string]interface{})
	if !ok {
		return false, errors.New("mismatched type for supportedStruct")
	}
	_, ok = supportedStruct["0"]
	if !ok {
		return false, errors.New("unexpected data structure returned in ServiceHandler IsMethodSupported")
	}
	isMethodSupportBool, ok := supportedStruct["0"].(bool)
	if !ok {
		return false, errors.New("mismatched type for isMethodSupportBool")
	}
	return isMethodSupportBool, nil
}

// GetServiceURL returns the URL of the service
func (n *NetworkServiceHandler) GetNetworkServiceURL() (string, error) {
	urlData, err := n.ContractHandler.Call(ServiceURL)
	if err != nil {
		return "", errors.Wrap(err, "failed call to NetworkService GetServiceURL")
	}

	urlStruct, ok := urlData.(map[string]interface{})
	if !ok {
		return "", errors.New("mismatched type for urlStruct")
	}
	_, ok = urlStruct["0"]
	if !ok {
		return "", errors.New("unexpected data structure returned in ServiceHandler GetServiceURL")
	}
	serviceURL, ok := urlStruct["0"].(string)
	if !ok {
		return "", errors.New("mismatched type for serviceURL")
	}
	return serviceURL, nil
}

// GetNetworkAddress returns the address of the network that the network service is on
func (n *NetworkServiceHandler) GetNetworkAddress() (*ethgo.Address, error) {
	nameData, err := n.ContractHandler.Call(GetNetworkAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to NetworkService GetNetworkAddress")
	}

	nameStruct, ok := nameData.(map[string]interface{})
	if !ok {
		return nil, errors.New("mismatched type for nameStruct")
	}

	_, ok = nameStruct["0"]
	if !ok {
		return nil, errors.New("unexpected data structure returned in NetworkService GetNetworkAddress")
	}
	serviceAddress, ok := nameStruct["0"].(ethgo.Address)
	if !ok {
		return nil, errors.New("mismatched type for serviceAddress")
	}
	return &serviceAddress, nil
}

// GetNetworkServiceStatus returns the status of the network service
func (n *NetworkServiceHandler) GetNetworkServiceStatus() (string, error) {
	statusData, err := n.ContractHandler.Call(GetNetworkServiceStatus)
	if err != nil {
		return "", errors.Wrap(err, "failed call to NetworkService GetNetworkServiceStatus")
	}

	statusStruct, ok := statusData.(map[string]interface{})
	if !ok {
		return "", errors.New("mismatched type for statusStruct")
	}

	_, ok = statusStruct["status"]
	if !ok {
		return "", errors.New("unexpected data structure returned in NetworkService GetNetworkServiceStatus")
	}

	status, ok := statusStruct["status"].(uint8)
	if !ok {
		return "", errors.New("mismatched type for status")
	}

	return getNetworkServiceStatus(status)
}

func getNetworkServiceStatus(networkServiceStatusCode uint8) (string, error) {
	switch networkServiceStatusCode {
	case 0:
		return None, nil
	case 1:
		return Onboarding, nil
	case 2:
		return Active, nil
	case 3:
		return Maintenance, nil
	case 4:
		return Retired, nil
	}

	return "", errors.New("Invalid Network Service Status Code")
}
