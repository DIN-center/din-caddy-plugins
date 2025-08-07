package web3

import (
	"context"

	"github.com/ethereum/go-ethereum/ethclient"
)

type EVMClient struct {
	client *ethclient.Client
}

func NewEVMClient(client *ethclient.Client) *EVMClient {
	return &EVMClient{client: client}
}

func (c *EVMClient) LatestBlockNumber() (uint64, error) {
	return c.client.BlockNumber(context.Background())
}
