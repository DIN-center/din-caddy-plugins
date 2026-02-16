package network

// HandlerType represents the type of network handler
type HandlerType string

const (
	// Handler types
	EVMHandlerType            HandlerType = "evm"
	BeaconHandlerType         HandlerType = "beacon-chain"
	StarknetHandlerType       HandlerType = "starknet"
	SolanaHandlerType         HandlerType = "solana"
	StellarRpcHandlerType     HandlerType = "stellar-rpc"
	BitcoinHandlerType        HandlerType = "bitcoin"
	BitcoinEsploraHandlerType HandlerType = "bitcoin-esplora"
	TronHandlerType           HandlerType = "tron-full-node"
)
