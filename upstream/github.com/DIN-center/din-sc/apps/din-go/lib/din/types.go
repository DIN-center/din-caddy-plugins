package din

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/pkg/errors"
)

type DinRegistryData struct {
	Networks map[string]*Network
}

type Network struct {
	Address       string                   `json:"address"`
	Owner         string                   `json:"owner"`
	Status        NetworkStatus            `json:"status"`
	Name          string                   `json:"name"`
	Description   string                   `json:"description"`
	ProxyName     string                   `json:"proxy_name"`
	MethodsByName map[string]*Method       `json:"-"` // Hide from default JSON marshaling
	Providers     map[string]*Provider     `json:"-"` // Hide from default JSON marshaling
	Capabilities  *big.Int                 `json:"capabilities"`
	NetworkConfig *NetworkOperationsConfig `json:"network_config"`
}

// MarshalJSON implements custom JSON marshaling to convert MethodsByName map to a list
func (n *Network) MarshalJSON() ([]byte, error) {
	type NetworkAlias Network // Create alias to avoid infinite recursion

	// Create a temporary struct for marshaling
	temp := struct {
		*NetworkAlias
		Methods   []*Method   `json:"methods,omitempty"`
		Providers []*Provider `json:"providers,omitempty"`
	}{
		NetworkAlias: (*NetworkAlias)(n),
		Methods:      n.GetMethodsAsSlice(),
		Providers:    n.GetProvidersAsSlice(),
	}

	return json.Marshal(temp)
}

// GetMethodsAsSlice returns the methods as a slice for JSON marshaling
func (n *Network) GetMethodsAsSlice() []*Method {
	if n.MethodsByName == nil {
		return nil
	}

	methods := make([]*Method, 0, len(n.MethodsByName))
	for _, method := range n.MethodsByName {
		methods = append(methods, method)
	}

	return methods
}

// GetProvidersAsSlice returns the providers as a slice for JSON marshaling
func (n *Network) GetProvidersAsSlice() []*Provider {
	if n.Providers == nil {
		return nil
	}

	providers := make([]*Provider, 0, len(n.Providers))
	for _, provider := range n.Providers {
		providers = append(providers, provider)
	}

	return providers
}

type Provider struct {
	Address         string                     `json:"address"`
	Name            string                     `json:"name"`
	Owner           string                     `json:"owner"`
	NetworkServices map[string]*NetworkService `json:"-"` // Hide from default JSON marshaling
	AuthConfig      *ProviderAuthConfig        `json:"auth_config"`
	Status          ProviderStatus             `json:"status"`
}

// MarshalJSON implements custom JSON marshaling to convert NetworkServices map to a list
func (p *Provider) MarshalJSON() ([]byte, error) {
	type ProviderAlias Provider // Create alias to avoid infinite recursion

	// Create a temporary struct for marshaling
	temp := struct {
		*ProviderAlias
		NetworkServices []*NetworkService `json:"services,omitempty"`
	}{
		ProviderAlias:   (*ProviderAlias)(p),
		NetworkServices: p.GetNetworkServicesAsSlice(),
	}

	return json.Marshal(temp)
}

// GetNetworkServicesAsSlice returns the network services as a slice for JSON marshaling
func (p *Provider) GetNetworkServicesAsSlice() []*NetworkService {
	if p.NetworkServices == nil {
		return nil
	}

	services := make([]*NetworkService, 0, len(p.NetworkServices))
	for _, service := range p.NetworkServices {
		services = append(services, service)
	}

	return services
}

type NetworkService struct {
	Address        string                   `json:"address"`
	Status         NetworkServiceStatus     `json:"status"`
	Url            string                   `json:"endpoint"`
	Capabilities   *big.Int                 `json:"capabilities"`
	Methods        map[string]*Method       `json:"-"` // Hide from default JSON marshaling
	Locations      []NetworkServiceLocation `json:"locations"`
	NetworkAddress string                   `json:"-"` // Hide from JSON marshaling
	NetworkName    string                   `json:"-"` // Hide from JSON marshaling
}

// MarshalJSON implements custom JSON marshaling to convert Methods map to a list
func (ns *NetworkService) MarshalJSON() ([]byte, error) {
	type NetworkServiceAlias NetworkService // Create alias to avoid infinite recursion

	// Create a temporary struct for marshaling
	temp := struct {
		*NetworkServiceAlias
		Methods []*Method `json:"methods,omitempty"`
	}{
		NetworkServiceAlias: (*NetworkServiceAlias)(ns),
		Methods:             ns.GetMethodsAsSlice(),
	}

	return json.Marshal(temp)
}

// GetMethodsAsSlice returns the methods as a slice for JSON marshaling
func (ns *NetworkService) GetMethodsAsSlice() []*Method {
	if ns.Methods == nil {
		return nil
	}

	methods := make([]*Method, 0, len(ns.Methods))
	for _, method := range ns.Methods {
		methods = append(methods, method)
	}

	return methods
}

type Method struct {
	Name        string `json:"name"`
	Bit         uint8  `json:"bit"`
	Deactivated bool   `json:"deactivated"`
}

type ProviderAuthConfig struct {
	Type              ProviderAuthType `json:"type"`
	Url               string           `json:"url"`
	ApiKeyPlaceholder string           `json:"apikey_placeholder"`
	UseHeader         bool             `json:"use_header"`
}

type NetworkOperationsConfig struct {
	Handler                  string `json:"handler"`
	HealthcheckIntervalSec   uint8  `json:"health_check_interval_sec"`
	HealthcheckThreshold     uint8  `json:"health_check_threshold"`
	HealthcheckTimeout       uint16 `json:"health_check_timeout"`
	BlockLagLimit            uint8  `json:"block_lag_limit"`
	BlockJumpLimit           uint8  `json:"block_jump_limit"`
	RequestAttemptCount      uint8  `json:"request_attempt_count"`
	MaxRequestPayloadSizeKb  uint16 `json:"max_request_payload_size_kb"`
	RegistryBlockEpoch       uint32 `json:"registry_block_epoch"`
	ArchiveEnabled           bool   `json:"archive_enabled"`
	ProviderBlockHistorySize uint16 `json:"provider_block_history_size"`
	NetworkBlockHistorySize  uint16 `json:"network_block_history_size"`
	ChainId                  string `json:"chain_id"`
}

// Type-safe for network status
type NetworkStatus string

const (
	NetworkStatusNone           NetworkStatus = "None"
	NetworkStatusOnboarding     NetworkStatus = "Onboarding"
	NetworkStatusActive         NetworkStatus = "Active"
	NetworkStatusMaintenance    NetworkStatus = "Maintenance"
	NetworkStatusDecommissioned NetworkStatus = "Decommissioned"
	NetworkStatusRetired        NetworkStatus = "Retired"
)

var NetworkStatusAll = []NetworkStatus{
	NetworkStatusNone,
	NetworkStatusOnboarding,
	NetworkStatusActive,
	NetworkStatusMaintenance,
	NetworkStatusDecommissioned,
	NetworkStatusRetired,
}

func (s NetworkStatus) IsValid() bool {
	switch s {
	case NetworkStatusNone,
		NetworkStatusOnboarding,
		NetworkStatusActive,
		NetworkStatusMaintenance,
		NetworkStatusDecommissioned,
		NetworkStatusRetired:
		return true
	}
	return false
}
func (s *NetworkStatus) String() string {
	return string(*s)
}

func (s *NetworkStatus) Set(value string) error {
	status := NetworkStatus(value)
	if !status.IsValid() {
		return fmt.Errorf("invalid network status: %s. Must be one of: %s", value, strings.Join(func() []string {
			statuses := make([]string, len(NetworkStatusAll))
			for i, status := range NetworkStatusAll {
				statuses[i] = status.String()
			}
			return statuses
		}(), ", "))
	}
	*s = NetworkStatus(status)
	return nil
}

func (s *NetworkStatus) Type() string {
	return "NetworkStatus"
}

func NetworkStatusFromCode(networkStatusCode uint8) (NetworkStatus, error) {
	switch networkStatusCode {
	case 0:
		return NetworkStatusNone, nil
	case 1:
		return NetworkStatusOnboarding, nil
	case 2:
		return NetworkStatusActive, nil
	case 3:
		return NetworkStatusMaintenance, nil
	case 4:
		return NetworkStatusDecommissioned, nil
	case 5:
		return NetworkStatusRetired, nil
	}
	return "", errors.New("Invalid Network Status Code")
}

func (s NetworkStatus) ToCode() (uint8, error) {
	switch s {
	case NetworkStatusNone:
		return 0, nil
	case NetworkStatusOnboarding:
		return 1, nil
	case NetworkStatusActive:
		return 2, nil
	case NetworkStatusMaintenance:
		return 3, nil
	case NetworkStatusDecommissioned:
		return 4, nil
	case NetworkStatusRetired:
		return 5, nil
	}
	return 0, errors.New("Invalid Network Status")
}

// Type-safe for provider status
type ProviderStatus string

const (
	ProviderStatusNone        ProviderStatus = "None"
	ProviderStatusOnboarding  ProviderStatus = "Onboarding"
	ProviderStatusActive      ProviderStatus = "Active"
	ProviderStatusMaintenance ProviderStatus = "Maintenance"
	ProviderStatusRetired     ProviderStatus = "Retired"
)

var ProviderStatusAll = []ProviderStatus{
	ProviderStatusNone,
	ProviderStatusOnboarding,
	ProviderStatusActive,
	ProviderStatusMaintenance,
	ProviderStatusRetired,
}

func (s ProviderStatus) IsValid() bool {
	switch s {
	case ProviderStatusNone,
		ProviderStatusOnboarding,
		ProviderStatusActive,
		ProviderStatusMaintenance,
		ProviderStatusRetired:
		return true
	}
	return false
}
func (s *ProviderStatus) String() string {
	return string(*s)
}

func (s *ProviderStatus) Set(value string) error {
	status := ProviderStatus(value)
	if !status.IsValid() {
		return fmt.Errorf("invalid provider status: %s. Must be one of: %s", value, strings.Join(func() []string {
			statuses := make([]string, len(ProviderStatusAll))
			for i, status := range ProviderStatusAll {
				statuses[i] = status.String()
			}
			return statuses
		}(), ", "))
	}
	*s = ProviderStatus(status)
	return nil
}

func (s ProviderStatus) Type() string {
	return "ProviderStatus"
}

func ProviderStatusFromCode(providerStatusCode uint8) (ProviderStatus, error) {
	switch providerStatusCode {
	case 0:
		return ProviderStatusNone, nil
	case 1:
		return ProviderStatusOnboarding, nil
	case 2:
		return ProviderStatusActive, nil
	case 3:
		return ProviderStatusMaintenance, nil
	case 4:
		return ProviderStatusRetired, nil
	}

	return "", errors.New("Invalid Provider Status Code")
}

func (s ProviderStatus) ToCode() (uint8, error) {
	switch s {
	case ProviderStatusNone:
		return 0, nil
	case ProviderStatusOnboarding:
		return 1, nil
	case ProviderStatusActive:
		return 2, nil
	case ProviderStatusMaintenance:
		return 3, nil
	case ProviderStatusRetired:
		return 4, nil
	}
	return 0, errors.New("invalid provider status")
}

// Type-safe for network service status
type NetworkServiceStatus string

const (
	NetworkServiceStatusNone        NetworkServiceStatus = "None"
	NetworkServiceStatusOnboarding  NetworkServiceStatus = "Onboarding"
	NetworkServiceStatusActive      NetworkServiceStatus = "Active"
	NetworkServiceStatusMaintenance NetworkServiceStatus = "Maintenance"
	NetworkServiceStatusRetired     NetworkServiceStatus = "Retired"
)

var NetworkServiceStatusAll = []NetworkServiceStatus{
	NetworkServiceStatusNone,
	NetworkServiceStatusOnboarding,
	NetworkServiceStatusActive,
	NetworkServiceStatusMaintenance,
	NetworkServiceStatusRetired,
}

func (s NetworkServiceStatus) IsValid() bool {
	switch s {
	case NetworkServiceStatusNone,
		NetworkServiceStatusOnboarding,
		NetworkServiceStatusActive,
		NetworkServiceStatusMaintenance,
		NetworkServiceStatusRetired:
		return true
	}
	return false
}

func (s *NetworkServiceStatus) String() string {
	return string(*s)
}

func (s *NetworkServiceStatus) Set(value string) error {
	status := NetworkServiceStatus(value)
	if !status.IsValid() {
		return fmt.Errorf("invalid network service status: %s. Must be one of: %s", value, strings.Join(func() []string {
			statuses := make([]string, len(NetworkServiceStatusAll))
			for i, status := range NetworkServiceStatusAll {
				statuses[i] = status.String()
			}
			return statuses
		}(), ", "))
	}
	*s = NetworkServiceStatus(status)
	return nil
}

func (s NetworkServiceStatus) Type() string {
	return "NetworkServiceStatus"
}

func NetworkServiceStatusFromCode(networkServiceStatusCode uint8) (NetworkServiceStatus, error) {
	switch networkServiceStatusCode {
	case 0:
		return NetworkServiceStatusNone, nil
	case 1:
		return NetworkServiceStatusOnboarding, nil
	case 2:
		return NetworkServiceStatusActive, nil
	case 3:
		return NetworkServiceStatusMaintenance, nil
	case 4:
		return NetworkServiceStatusRetired, nil
	}
	return "", errors.New("Invalid Network Service Status Code")
}

func (s NetworkServiceStatus) ToCode() (uint8, error) {
	switch s {
	case NetworkServiceStatusNone:
		return 0, nil
	case NetworkServiceStatusOnboarding:
		return 1, nil
	case NetworkServiceStatusActive:
		return 2, nil
	case NetworkServiceStatusMaintenance:
		return 3, nil
	case NetworkServiceStatusRetired:
		return 4, nil
	}
	return 0, errors.New("invalid network service status")
}

// Type-safe for network service location
type NetworkServiceLocation string

const (
	NetworkServiceLocationNone             NetworkServiceLocation = "None"
	NetworkServiceLocationNorthAmerica     NetworkServiceLocation = "NorthAmerica"
	NetworkServiceLocationLatam            NetworkServiceLocation = "Latam"
	NetworkServiceLocationEurope           NetworkServiceLocation = "Europe"
	NetworkServiceLocationMiddleEastAfrica NetworkServiceLocation = "MiddleEastAfrica"
	NetworkServiceLocationAsiaPacific      NetworkServiceLocation = "AsiaPacific"
)

var NetworkServiceLocationAll = []NetworkServiceLocation{
	NetworkServiceLocationNone,
	NetworkServiceLocationNorthAmerica,
	NetworkServiceLocationLatam,
	NetworkServiceLocationEurope,
	NetworkServiceLocationMiddleEastAfrica,
	NetworkServiceLocationAsiaPacific,
}

func (s NetworkServiceLocation) IsValid() bool {
	switch s {
	case NetworkServiceLocationNone,
		NetworkServiceLocationNorthAmerica,
		NetworkServiceLocationLatam,
		NetworkServiceLocationEurope,
		NetworkServiceLocationMiddleEastAfrica,
		NetworkServiceLocationAsiaPacific:
		return true
	}
	return false
}

func (s *NetworkServiceLocation) String() string {
	return string(*s)
}

func (s *NetworkServiceLocation) Set(value string) error {
	location := NetworkServiceLocation(value)
	if !location.IsValid() {
		return fmt.Errorf("invalid network service location: %s. Must be one of: %s", value, strings.Join(func() []string {
			locations := make([]string, len(NetworkServiceLocationAll))
			for i, location := range NetworkServiceLocationAll {
				locations[i] = location.String()
			}
			return locations
		}(), ", "))
	}
	*s = NetworkServiceLocation(location)
	return nil
}

func (s NetworkServiceLocation) Type() string {
	return "NetworkServiceLocation"
}

func NetworkServiceLocationFromCode(networkServiceLocationCode uint8) (NetworkServiceLocation, error) {
	switch networkServiceLocationCode {
	case 0:
		return NetworkServiceLocationNone, nil
	case 1:
		return NetworkServiceLocationNorthAmerica, nil
	case 2:
		return NetworkServiceLocationLatam, nil
	case 3:
		return NetworkServiceLocationEurope, nil
	case 4:
		return NetworkServiceLocationMiddleEastAfrica, nil
	case 5:
		return NetworkServiceLocationAsiaPacific, nil
	}
	return "", errors.New("Invalid Network Service Location Code")
}

func (s NetworkServiceLocation) ToCode() (uint8, error) {
	switch s {
	case NetworkServiceLocationNone:
		return 0, nil
	case NetworkServiceLocationNorthAmerica:
		return 1, nil
	case NetworkServiceLocationLatam:
		return 2, nil
	case NetworkServiceLocationEurope:
		return 3, nil
	case NetworkServiceLocationMiddleEastAfrica:
		return 4, nil
	case NetworkServiceLocationAsiaPacific:
		return 5, nil
	}
	return 0, errors.New("invalid network service location")
}

// Auth Type for Network Service
type ProviderAuthType string

const (
	ProviderAuthTypeNone   ProviderAuthType = "None"
	ProviderAuthTypeSIWE   ProviderAuthType = "SiWE"
	ProviderAuthTypeAPIKEY ProviderAuthType = "ApiKey"
)

var ProviderAuthTypeAll = []ProviderAuthType{
	ProviderAuthTypeNone,
	ProviderAuthTypeSIWE,
	ProviderAuthTypeAPIKEY,
}

func (s ProviderAuthType) IsValid() bool {
	switch s {
	case ProviderAuthTypeNone,
		ProviderAuthTypeSIWE,
		ProviderAuthTypeAPIKEY:
		return true
	}
	return false
}

func (s *ProviderAuthType) String() string {
	return string(*s)
}

func (s *ProviderAuthType) Set(value string) error {
	authType := ProviderAuthType(value)
	if !authType.IsValid() {
		return fmt.Errorf("invalid provider auth type: %s. Must be one of: %s", value, strings.Join(func() []string {
			authTypes := make([]string, len(ProviderAuthTypeAll))
			for i, authType := range ProviderAuthTypeAll {
				authTypes[i] = authType.String()
			}
			return authTypes
		}(), ", "))
	}
	*s = ProviderAuthType(authType)
	return nil
}

func (s ProviderAuthType) Type() string {
	return "ProviderAuthType"
}

func ProviderAuthTypeFromCode(providerAuthTypeCode uint8) (ProviderAuthType, error) {
	switch providerAuthTypeCode {
	case 0:
		return ProviderAuthTypeNone, nil
	case 1:
		return ProviderAuthTypeSIWE, nil
	case 2:
		return ProviderAuthTypeAPIKEY, nil
	}
	return "", errors.New("invalid provider auth type")
}

func (s ProviderAuthType) ToCode() (uint8, error) {
	switch s {
	case ProviderAuthTypeNone:
		return 0, nil
	case ProviderAuthTypeSIWE:
		return 1, nil
	case ProviderAuthTypeAPIKEY:
		return 2, nil
	}
	return 0, errors.New("invalid provider auth type")
}
