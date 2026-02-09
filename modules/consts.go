package modules

import "time"

type HealthStatus int

// HandlerType represents the type of network handler
type HandlerType string

const (
	// Handler types
	EVMHandler            HandlerType = "evm"
	BeaconHandler         HandlerType = "beacon-chain"
	StarknetHandler       HandlerType = "starknet"
	SolanaHandler         HandlerType = "solana"
	BitcoinHandler        HandlerType = "bitcoin"
	BitcoinEsploraHandler HandlerType = "bitcoin-esplora"
	TronHandler           HandlerType = "tron-full-node"
)

const (
	// Health status enums
	Healthy HealthStatus = iota
	Warning
	Unhealthy
)

const (
	LineaMainnet = "linea-mainnet"
	LineaSepolia = "linea-sepolia"

	// Module Context Key constants
	DinUpstreamsContextKey          = "din.internal.upstreams"
	DinExcludedProvidersContextKey  = "din.internal.excluded_providers"
	RequestProviderKey              = "request_provider"
	RequestProviderPriorityKey      = "request_provider_priority"
	RequestBodyKey                  = "request_body"
	RequestMethodKey                = "request_method"
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
	DefaultBlockLagLimit    = int64(15)
	DefaultBlockLagPeriodMs = 13000 // 13 seconds in milliseconds for dynamic block lag calculation
	DefaultBlockJumpLimit   = int64(100)
	DefaultMaxRequestPayloadSizeKB = int64(4096)
	DefaultRequestAttemptCount     = 5
	DefaultArchiveEnabled          = false

	// Registry constants
	DefaultRegistryBlockCheckIntervalSec = int64(60)
	DefaultRegistryBlockEpoch            = uint64(2000)
	DefaultRegistryPriority              = 0
	DefaultRegistryRetryMaxAttempts      = 3
	DefaultRegistryRetryDelay            = 2 * time.Second  // Fixed delay between retries
	DefaultRegistryPanicRecoveryDelay    = 30 * time.Second // Delay before restarting after panic

	// General constants
	DefaultPort = "8000"

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

	// Watcher score based dynamic load balancing constants
	DinScoreBasedLoadBalancingContextKey     = "din.internal.score_based_load_balancing"
	DinScoreBasedSelectionCaddyModuleKey     = "http.reverse_proxy.selection_policies.din_score_based_selector"
	ScoreBasedSelectionProviderDefaultWeight = 50
	StaleScoreGracePeriod                    = 60 * time.Minute
	StaleScoreConvergencePeriod              = 60 * time.Minute
	WatcherScoreUpdateInterval               = 1 * time.Minute
	ScoreBasedSelectionWeightBase            = 100
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
