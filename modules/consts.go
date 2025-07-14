package modules

type HealthStatus int

const (
	// Health status enums
	Healthy HealthStatus = iota
	Warning
	Unhealthy

	LineaMainnet = "linea-mainnet"
	LineaSepolia = "linea-sepolia"

	// Module Context Key constants
	DinUpstreamsContextKey          = "din.internal.upstreams"
	RequestProviderKey              = "request_provider"
	RequestBodyKey                  = "request_body"
	RequestMethodKey                = "request_method"
	RequestContextKey               = "request_context"
	HealthStatusKey                 = "health_status"
	BlockNumberKey                  = "block_number"
	DefaultProviderBlockHistorySize = 10
	DefaultNetworkBlockHistorySize  = 128

	// Health check constants
	DefaultHCMethod                = "eth_blockNumber"
	DefaultChainIdMethod           = "eth_chainId"
	DefaultCallContractMethod      = "eth_call"
	DefaultGetBlockByNumberMethod  = "eth_getBlockByNumber"
	DefaultHCThreshold             = 2
	DefaultHCTimeout               = 5
	DefaultHCInterval              = 5
	DefaultBlockLagLimit           = int64(5)
	DefaultBlockJumpLimit          = int64(100)
	DefaultMaxRequestPayloadSizeKB = int64(4096)
	DefaultRequestAttemptCount     = 5
	DefaultArchiveEnabled          = false
	// Registry constants
	DefaultRegistryBlockCheckIntervalSec = uint64(60)
	DefaultRegistryBlockEpoch            = uint64(2000)
	DefaultRegistryPriority              = 0
	DefaultPort                          = "8000"
	// Additional Status Codes
	StatusOriginUnreachable = 523

	// Request/Response Header Keys
	DinProviderInfo = "din-provider-info"

	// Upstream/Selector Constants
	MaxPriority = 9

	// Chain ID Namespace Constants
	// Namespace:Referece = Chain ID https://chainagnostic.org/CAIPs/caip-2
	// EVM
	EVMNamespace = "eip155"
	// Bitcoin
	BitcoinNamespace = "bip122"
	// Solana
	SolanaNamespace = "solana"
	// Starknet
	StarknetNamespace     = "starknet"
	StarknetArchiveMethod = "starknet_getBlockWithTxs"
)

// String method to convert MyEnum to string
func (h HealthStatus) String() string {
	switch h {
	case Healthy:
		return "Healthy"
	case Warning:
		return "Warning"
	case Unhealthy:
		return "Unhealthy"
	default:
		return "Unknown"
	}
}
