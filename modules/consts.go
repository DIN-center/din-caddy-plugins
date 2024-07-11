package modules

type HealthStatus int

const (
	// Health status enums
	Healthy HealthStatus = iota
	Warning
	Unhealthy

	// Module constants
	DinUpstreamsContextKey = "din.internal.upstreams"

	// Runtime constants
	EthereumRuntime = "ethereum"
	SolanaRuntime   = "solana"
	StarknetRuntime = "starknet"
	DefaultRuntime  = EthereumRuntime

	// Health check constants
	DefaultHCThreshold   = 2
	DefaultHCInterval    = 5
	DefaultBlockLagLimit = int64(5)
)
