package modules

var (
	// Module constants
	DinUpstreamsContextKey = "din.internal.upstreams"

	// Runtime constants
	EthereumRuntime = "ethereum"
	SolanaRuntime   = "solana"
	StarknetRuntime = "starknet"
	DefaultRuntime  = EthereumRuntime

	// Health check constants
	DefaultHCThreshold = 2
	DefaultHCInterval  = 5
	DefaultBlockLagLimit = 5
)
