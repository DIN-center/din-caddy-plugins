package dinregistry

const (
	// Contract calls
	// Shared
	IsMethodSupported = "isMethodSupported"

	// Provider Functions
	Name                  = "name"
	ProviderOwner         = "providerOwner"
	ProviderStatus        = "providerStatus"
	GetAllNetworkServices = "getAllNetworkServices"
	AuthConfig            = "authConfig"

	// Network Functions
	GetMethodId                = "getMethodId"
	GetMethodName              = "getMethodName"
	GetAllMethods              = "allMethods"
	GetNetworkOperationsConfig = "getNetworkOperationsConfig"
	GetNetworkName             = "getName"
	GetNetworkOwnerAddress     = "networkOwner"
	GetNetworkCapabilities     = "getCapabilities"
	GetNetworkStatus           = "getNetworkStatus"

	// Network Service Functions
	GetAllMethodNames             = "getAllMethodNames"
	ServiceURL                    = "serviceUrl"
	GetNetworkAddress             = "inetwork"
	GetNetworkServiceStatus       = "getStatus"
	GetNetworkServiceCapabilities = "capabilities"

	// DinRegistry Functions
	GetAllNetworks            = "getAllNetworks"
	GetAllNetworkMethods      = "getAllNetworkMethods"
	GetAllNetworkMethodNames  = "getAllNetworkMethodNames"
	GetRegNetworkCapabilities = "getNetworkCapabilities"
	GetAllProviders           = "getAllProviders"
	GetProviders              = "getProviders"
	GetNetworkFromName        = "getNetworkFromName"

	// ABI Paths
	ABINetworkPath     = "abi/network.abi"
	ABIProviderPath    = "abi/provider.abi"
	ABIServicePath     = "abi/network_service.abi"
	ABIDinRegistryPath = "abi/din_registry.abi"

	// Provider, Network and Network Service Statuses
	None           = "None"
	Onboarding     = "Onboarding"
	Active         = "Active"
	Maintenance    = "Maintenance"
	Decommissioned = "Decommissioned"
	Retired        = "Retired"

	// Auth Type for Network Service
	SIWE = "siwe"
)

// Method corresponds to the `struct Method` in the ABI.
type Method struct {
	Name        string `json:"name"`
	Bit         uint8  `json:"bit"`
	Deactivated bool   `json:"deactivated,omitempty"`
}

type Network struct {
	Name          string
	Description   string
	OwnerAddress  string
	NetworkStatus string
}

type NetworkConfig struct {
	HealthcheckMethodBit    uint8
	HealthcheckIntervalSec  uint8
	BlockLagLimit           uint8
	RequestAttemptCount     uint8
	MaxRequestPayloadSizeKb uint16
}

type NetworkServiceAuthConfig struct {
	Type string
	Url  string
}
