package din

import (
	"encoding/json"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
)

// ApiDingoClient is an implementation of IDinReader that interacts with a DIN registry via its HTTP API.
type ApiDingoClient struct {
	baseURL    string
	httpClient *http.Client
}

type apiServicesResponse struct {
	Ok    bool         `json:"ok"`
	Count int          `json:"count"`
	Skip  int          `json:"skip"`
	Limit int          `json:"limit"`
	Data  []apiService `json:"data"`
}

type apiService struct {
	ServiceID string            `json:"service_id"`
	Name      string            `json:"name"`
	Status    string            `json:"status"`
	Route     string            `json:"route"`
	Config    apiServiceConfig  `json:"config"`
	Paths     []apiServicePath  `json:"paths"`
	Providers []apiServiceOwner `json:"providers"`
}

type apiServiceConfig struct {
	Handler                  string      `json:"handler"`
	HealthcheckIntervalSec   json.Number `json:"health_check_interval_sec"`
	HealthcheckThreshold     json.Number `json:"health_check_threshold"`
	HealthcheckTimeout       json.Number `json:"health_check_timeout"`
	BlockLagLimit            json.Number `json:"block_lag_limit"`
	BlockJumpLimit           json.Number `json:"block_jump_limit"`
	RequestAttemptCount      json.Number `json:"request_attempt_count"`
	MaxRequestPayloadSizeKb  json.Number `json:"max_request_payload_size_kb"`
	RegistryBlockEpoch       json.Number `json:"registry_block_epoch"`
	ArchiveEnabled           *bool       `json:"archive_enabled"`
	ProviderBlockHistorySize json.Number `json:"provider_block_history_size"`
	NetworkBlockHistorySize  json.Number `json:"network_block_history_size"`
	ChainId                  string      `json:"chain_id"`
}

type apiServicePath struct {
	Name        string `json:"name"`
	Bit         *uint8 `json:"bit"`
	Deactivated *bool  `json:"deactivated"`
}

type apiServiceOwner struct {
	ProviderID string             `json:"provider_id"`
	Name       string             `json:"name"`
	Status     string             `json:"status"`
	AuthConfig apiProviderAuth    `json:"auth_config"`
	Endpoints  []apiServiceTarget `json:"endpoints"`
}

type apiProviderAuth struct {
	Type string `json:"type"`
	Url  string `json:"url"`
}

type apiServiceTarget struct {
	Url    string `json:"url"`
	Status string `json:"status"`
}

// NewApiDingoClient creates a new ApiDingoClient with the specified base URL and optional HTTP client.
func NewApiDingoClient(baseURL string, httpClient *http.Client) *ApiDingoClient {
	client := httpClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	return &ApiDingoClient{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: client,
	}
}

// GetLatestBlockNumber retrieves the latest block number for the registry source.
// Returns a unix timestamp for now, or an error if the operation fails.
func (c *ApiDingoClient) GetLatestBlockNumber() (uint64, error) {
	return uint64(time.Now().Unix()), nil
}

// GetRegistryData retrieves all registry data including networks and providers.
// Returns a DinRegistryData struct containing the complete registry state, or an error if the operation fails.
func (c *ApiDingoClient) GetRegistryData() (*DinRegistryData, error) {
	services, err := c.fetchServices()
	if err != nil {
		return nil, err
	}

	registryData := &DinRegistryData{
		Networks: make(map[string]*Network),
	}

	for _, service := range services.Data {
		name := strings.TrimSpace(service.Name)
		if name == "" {
			name = service.ServiceID
		}

		networkStatus, err := parseNetworkStatus(service.Status)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse network status")
		}

		networkConfig, err := mapNetworkConfig(service.Config)
		if err != nil {
			return nil, errors.Wrap(err, "failed to map network config")
		}

		methods := make(map[string]*Method)
		for _, path := range service.Paths {
			bit := uint8(0)
			if path.Bit != nil {
				bit = *path.Bit
			}
			method := &Method{
				Name:        path.Name,
				Bit:         bit,
				Deactivated: path.Deactivated != nil && *path.Deactivated,
			}
			methods[method.Name] = method
		}

		providers := make(map[string]*Provider)
		for _, provider := range service.Providers {
			providerStatus, err := parseProviderStatus(provider.Status)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse provider status")
			}

			authType, err := parseProviderAuthType(provider.AuthConfig.Type)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse provider auth type")
			}

			networkServices := make(map[string]*NetworkService)
			for _, endpoint := range provider.Endpoints {
				endpointStatus, err := parseNetworkServiceStatus(endpoint.Status)
				if err != nil {
					return nil, errors.Wrap(err, "failed to parse network service status")
				}

				networkService := &NetworkService{
					Address:        endpoint.Url,
					Url:            endpoint.Url,
					Capabilities:   big.NewInt(0), // Capabilities are not exposed by the API.
					Status:         endpointStatus,
					Locations:      nil,
					NetworkAddress: service.ServiceID,
					NetworkName:    name,
					Methods:        nil,
				}
				networkServices[endpoint.Url] = networkService
			}

			newProvider := &Provider{
				Address:         provider.ProviderID,
				Name:            provider.Name,
				Owner:           "",
				NetworkServices: networkServices,
				AuthConfig: &ProviderAuthConfig{
					Type:              authType,
					Url:               provider.AuthConfig.Url,
					ApiKeyPlaceholder: "",
					UseHeader:         false,
				},
				Status: providerStatus,
			}
			providers[newProvider.Name] = newProvider
		}

		network := &Network{
			Address:       service.ServiceID,
			Owner:         "",
			Status:        networkStatus,
			Name:          name,
			Description:   "",
			ProxyName:     strings.TrimSpace(firstNonEmpty(service.Route, convertNetworkName(service.ServiceID))),
			MethodsByName: methods,
			Providers:     providers,
			Capabilities:  big.NewInt(0), // Capabilities are not exposed by the API.
			NetworkConfig: networkConfig,
		}

		registryData.Networks[network.Name] = network
	}

	return registryData, nil
}

// GetAllNetworks retrieves all networks registered in the DIN registry.
func (c *ApiDingoClient) GetAllNetworks() ([]*Network, error) {
	registryData, err := c.GetRegistryData()
	if err != nil {
		return nil, err
	}

	networks := make([]*Network, 0, len(registryData.Networks))
	for _, network := range registryData.Networks {
		networks = append(networks, network)
	}
	return networks, nil
}

// GetNetworkByAddress retrieves a specific network by its contract address.
func (c *ApiDingoClient) GetNetworkByAddress(networkAddress common.Address) (*Network, error) {
	registryData, err := c.GetRegistryData()
	if err != nil {
		return nil, err
	}

	for _, network := range registryData.Networks {
		if strings.EqualFold(network.Address, networkAddress.Hex()) {
			return network, nil
		}
	}

	return nil, errors.New("network not found")
}

// GetProviderByAddress retrieves a specific provider by its contract address.
func (c *ApiDingoClient) GetProviderByAddress(providerAddress common.Address) (*Provider, error) {
	registryData, err := c.GetRegistryData()
	if err != nil {
		return nil, err
	}

	for _, network := range registryData.Networks {
		for _, provider := range network.Providers {
			if strings.EqualFold(provider.Address, providerAddress.Hex()) {
				return provider, nil
			}
		}
	}

	return nil, errors.New("provider not found")
}

// GetNetworkByName retrieves a specific network by its URI.
func (c *ApiDingoClient) GetNetworkByName(networkName string) (*Network, error) {
	registryData, err := c.GetRegistryData()
	if err != nil {
		return nil, err
	}

	network, ok := registryData.Networks[networkName]
	if ok {
		return network, nil
	}

	for _, candidate := range registryData.Networks {
		if strings.EqualFold(candidate.Name, networkName) {
			return candidate, nil
		}
	}

	return nil, errors.New("network not found")
}

// GetNetworkServiceByAddress retrieves a specific network service by its contract address.
func (c *ApiDingoClient) GetNetworkServiceByAddress(networkServiceAddress common.Address) (*NetworkService, error) {
	registryData, err := c.GetRegistryData()
	if err != nil {
		return nil, err
	}

	for _, network := range registryData.Networks {
		for _, provider := range network.Providers {
			for _, service := range provider.NetworkServices {
				if strings.EqualFold(service.Address, networkServiceAddress.Hex()) {
					return service, nil
				}
			}
		}
	}

	return nil, errors.New("network service not found")
}

// GetAllProvidersByNetwork retrieves all providers that serve a specific network.
func (c *ApiDingoClient) GetAllProvidersByNetwork(networkURI string) ([]*Provider, error) {
	network, err := c.GetNetworkByName(networkURI)
	if err != nil {
		return nil, err
	}

	providers := make([]*Provider, 0, len(network.Providers))
	for _, provider := range network.Providers {
		providers = append(providers, provider)
	}
	return providers, nil
}

// GetAllProviders retrieves all providers registered in the DIN registry.
func (c *ApiDingoClient) GetAllProviders() ([]*Provider, error) {
	registryData, err := c.GetRegistryData()
	if err != nil {
		return nil, err
	}

	providersByAddress := make(map[string]*Provider)
	for _, network := range registryData.Networks {
		for _, provider := range network.Providers {
			providersByAddress[strings.ToLower(provider.Address)] = provider
		}
	}

	providers := make([]*Provider, 0, len(providersByAddress))
	for _, provider := range providersByAddress {
		providers = append(providers, provider)
	}

	return providers, nil
}

func (c *ApiDingoClient) fetchServices() (*apiServicesResponse, error) {
	requestURL := c.baseURL + "/api/v1/services"
	response, err := c.httpClient.Get(requestURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch services")
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, errors.Errorf("unexpected status code: %d", response.StatusCode)
	}

	decoder := json.NewDecoder(response.Body)
	decoder.UseNumber()

	var services apiServicesResponse
	if err := decoder.Decode(&services); err != nil {
		return nil, errors.Wrap(err, "failed to decode services response")
	}

	return &services, nil
}

func mapNetworkConfig(config apiServiceConfig) (*NetworkOperationsConfig, error) {
	return &NetworkOperationsConfig{
		Handler:                  config.Handler,
		HealthcheckIntervalSec:   parseUint8(config.HealthcheckIntervalSec),
		HealthcheckThreshold:     parseUint8(config.HealthcheckThreshold),
		HealthcheckTimeout:       parseUint16(config.HealthcheckTimeout),
		BlockLagLimit:            parseUint8(config.BlockLagLimit),
		BlockJumpLimit:           parseUint8(config.BlockJumpLimit),
		RequestAttemptCount:      parseUint8(config.RequestAttemptCount),
		MaxRequestPayloadSizeKb:  parseUint16(config.MaxRequestPayloadSizeKb),
		RegistryBlockEpoch:       parseUint32(config.RegistryBlockEpoch),
		ArchiveEnabled:           config.ArchiveEnabled != nil && *config.ArchiveEnabled,
		ProviderBlockHistorySize: parseUint16(config.ProviderBlockHistorySize),
		NetworkBlockHistorySize:  parseUint16(config.NetworkBlockHistorySize),
		ChainId:                  config.ChainId,
	}, nil
}

func parseNetworkStatus(status string) (NetworkStatus, error) {
	trimmed := strings.TrimSpace(status)
	if trimmed == "" {
		return NetworkStatusNone, nil
	}

	switch strings.ToLower(trimmed) {
	case "none":
		return NetworkStatusNone, nil
	case "onboarding":
		return NetworkStatusOnboarding, nil
	case "active":
		return NetworkStatusActive, nil
	case "maintenance":
		return NetworkStatusMaintenance, nil
	case "decommissioned":
		return NetworkStatusDecommissioned, nil
	case "retired":
		return NetworkStatusRetired, nil
	default:
		return "", errors.Errorf("unknown network status: %s", status)
	}
}

func parseProviderStatus(status string) (ProviderStatus, error) {
	trimmed := strings.TrimSpace(status)
	if trimmed == "" {
		return ProviderStatusNone, nil
	}

	switch strings.ToLower(trimmed) {
	case "none":
		return ProviderStatusNone, nil
	case "onboarding":
		return ProviderStatusOnboarding, nil
	case "active":
		return ProviderStatusActive, nil
	case "maintenance":
		return ProviderStatusMaintenance, nil
	case "retired":
		return ProviderStatusRetired, nil
	default:
		return "", errors.Errorf("unknown provider status: %s", status)
	}
}

func parseNetworkServiceStatus(status string) (NetworkServiceStatus, error) {
	trimmed := strings.TrimSpace(status)
	if trimmed == "" {
		return NetworkServiceStatusNone, nil
	}

	switch strings.ToLower(trimmed) {
	case "none":
		return NetworkServiceStatusNone, nil
	case "onboarding":
		return NetworkServiceStatusOnboarding, nil
	case "active":
		return NetworkServiceStatusActive, nil
	case "maintenance":
		return NetworkServiceStatusMaintenance, nil
	case "retired":
		return NetworkServiceStatusRetired, nil
	default:
		return "", errors.Errorf("unknown network service status: %s", status)
	}
}

func parseProviderAuthType(authType string) (ProviderAuthType, error) {
	trimmed := strings.TrimSpace(authType)
	if trimmed == "" {
		return ProviderAuthTypeNone, nil
	}

	switch strings.ToLower(trimmed) {
	case "none":
		return ProviderAuthTypeNone, nil
	case "siwe":
		return ProviderAuthTypeSIWE, nil
	case "apikey", "api_key", "api-key":
		return ProviderAuthTypeAPIKEY, nil
	default:
		return "", errors.Errorf("unknown provider auth type: %s", authType)
	}
}

// Several JSON type conversion methods follow

func parseUint8(value json.Number) uint8 {
	parsed, err := value.Int64()
	if err != nil || parsed < 0 {
		return 0
	}
	if parsed > 255 {
		return 255
	}
	return uint8(parsed)
}

func parseUint16(value json.Number) uint16 {
	parsed, err := value.Int64()
	if err != nil || parsed < 0 {
		return 0
	}
	if parsed > 65535 {
		return 65535
	}
	return uint16(parsed)
}

func parseUint32(value json.Number) uint32 {
	parsed, err := value.Int64()
	if err != nil || parsed < 0 {
		return 0
	}
	if parsed > 4294967295 {
		return 4294967295
	}
	return uint32(parsed)
}

func collectPathNames(paths []apiServicePath) []string {
	methodNames := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.TrimSpace(path.Name) == "" {
			continue
		}
		methodNames = append(methodNames, path.Name)
	}
	return methodNames
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
