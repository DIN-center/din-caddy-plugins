package web3

// Minimal interface for a web3 client.
type Web3Client interface {
	// LatestBlockNumber  provides access to the current block number.
	LatestBlockNumber() (uint64, error)
}
