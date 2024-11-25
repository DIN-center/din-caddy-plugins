package din

import "github.com/umbracle/ethgo/jsonrpc"

type IDingoClient interface {
	GetRegistryData() (*DinRegistryData, error)
	GetNetworkServiceMethods(networkServiceAddress string) ([]*string, error)
	GetNetworkMethodNameByBit(networkName string, bit uint8) (string, error)
	GetEthClient() *jsonrpc.Client
	GetLatestBlockNumber() (uint64, error)
}
