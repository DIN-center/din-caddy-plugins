package dinregistry

import (
	"embed"
	"io/fs"

	"github.com/pkg/errors"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/jsonrpc"
)

type ProviderHandler struct {
	ContractHandler IContractHandler
}

//go:embed abi/provider.abi
var abiProvideFS embed.FS

func NewProviderHandler(ethClient *jsonrpc.Client, contractAddress string) (*ProviderHandler, error) {
	abiBytes, err := fs.ReadFile(abiProvideFS, ABIProviderPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to NewProviderHandler ReadFile")
	}
	abiStr := string(abiBytes)
	contractHandler, err := NewContractHandler(ethClient, contractAddress, abiStr)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to NewProviderHandler NewContractWrapper")
	}
	return &ProviderHandler{contractHandler}, nil
}

func (p *ProviderHandler) GetName() (string, error) {
	name, err := p.ContractHandler.Call(Name)
	if err != nil {
		return "", errors.Wrap(err, "failed call to Provider Name")
	}
	nameStruct, ok := name.(map[string]interface{})
	if !ok {
		return "", errors.New("mismatched type for nameStruct")
	}
	_, ok = nameStruct["0"]
	if !ok {
		return "", errors.New("unexpected data structure returned in Provider Name")
	}

	nameStr, ok := nameStruct["0"].(string)
	if !ok {
		return "", errors.New("mismatched type for nameStr")
	}

	return nameStr, nil
}

func (p *ProviderHandler) GetProviderOwner() (*ethgo.Address, error) {
	owner, err := p.ContractHandler.Call(ProviderOwner)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to ProviderHandler Owner")
	}
	ownerStruct, ok := owner.(map[string]interface{})
	if !ok {
		return nil, errors.New("mismatched type for ownerStruct")
	}
	_, ok = ownerStruct["0"]
	if !ok {
		return nil, errors.New("unexpected data structure returned in Provider Owner")
	}
	ownerAddress, ok := ownerStruct["0"].(ethgo.Address)
	if !ok {
		return nil, errors.New("mismatched type for ownerAddress")
	}
	return &ownerAddress, nil
}

func (p *ProviderHandler) GetProviderStatus() (string, error) {
	status, err := p.ContractHandler.Call(ProviderStatus)
	if err != nil {
		return "", errors.Wrap(err, "failed call to ProviderHandler Status")
	}
	statusStruct, ok := status.(map[string]interface{})
	if !ok {
		return "", errors.New("mismatched type for statusStruct")
	}
	_, ok = statusStruct["0"]
	if !ok {
		return "", errors.New("unexpected data structure returned in Provider Status")
	}
	statusCode, ok := statusStruct["0"].(uint8)
	if !ok {
		return "", errors.New("mismatched type for statusCode")
	}
	return getProviderStatus(statusCode)
}

func (p *ProviderHandler) GetAllNetworkServiceAddresses() ([]ethgo.Address, error) {
	allNetworkServices, err := p.ContractHandler.Call(GetAllNetworkServices)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to ProviderHandler AllServices Call")
	}

	allNetworkServicesStruct, ok := allNetworkServices.(map[string]interface{})
	if !ok {
		return nil, errors.New("mismatched type for allServicesStruct")
	}
	_, ok = allNetworkServicesStruct["allServices"]
	if !ok {
		return nil, errors.New("unexpected data structure returned in ProviderHandler AllServices")
	}

	networkServiceAddresses, ok := allNetworkServicesStruct["allServices"].([]ethgo.Address)
	if !ok {
		return nil, errors.New("mismatched type for serviceAddresses")
	}
	return networkServiceAddresses, nil
}

func (p *ProviderHandler) GetAuthConfig() (*NetworkServiceAuthConfig, error) {
	authConfig, err := p.ContractHandler.Call(AuthConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to ProviderHandler AuthConfig")
	}

	authConfigStruct, ok := authConfig.(map[string]interface{})
	if !ok {
		return nil, errors.New("mismatched type for authConfigStruct")
	}
	authTypeCode, ok := authConfigStruct["auth"].(uint8)
	if !ok {
		return nil, errors.New("mismatched type for auth")
	}

	authType, err := getProviderAuthType(authTypeCode)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to getProviderAuthType")
	}

	authUrl, ok := authConfigStruct["url"].(string)
	if !ok {
		return nil, errors.New("mismatched type for authUrl")
	}
	return &NetworkServiceAuthConfig{
		Type: authType,
		Url:  authUrl,
	}, nil
}

func getProviderStatus(providerStatusCode uint8) (string, error) {
	switch providerStatusCode {
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

	return "", errors.New("Invalid Provider Status Code")
}

func getProviderAuthType(authTypeCode uint8) (string, error) {
	switch authTypeCode {
	case 0:
		return None, nil
	case 1:
		return SIWE, nil
	default:
		return "", errors.New("invalid auth type")
	}
}
