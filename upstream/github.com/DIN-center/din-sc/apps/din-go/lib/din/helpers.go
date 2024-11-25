package din

import (
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/umbracle/ethgo/jsonrpc"
)

// setupWeb3Client sets up the web3 client with the RPC provider string
func setupEthClient(rpcEndpointURL string) (*jsonrpc.Client, error) {
	rpcProvider := getRPCProvider(rpcEndpointURL)
	ethClient, err := jsonrpc.NewClient(rpcProvider)
	if err != nil {
		return nil, errors.Wrap(err, "failed call to ethclient.Dial")
	}
	return ethClient, nil
}

// getRPCProvider returns the RPC provider URL.
// Priority is given first to the function argument `rpcEndpointURL` (if provided),
// then to the environment variable `RPC_PROVIDER_URL`, and finally to a
// default URL if neither is available.
func getRPCProvider(rpcEndpointURL string) string {
	envEndpointUrl := os.Getenv("RPC_PROVIDER_URL")
	if rpcEndpointURL != "" {
		return rpcEndpointURL
	} else if envEndpointUrl != "" {
		return envEndpointUrl
	}

	// default rpc value
	return "http://localhost:8545"
}

// getDinRegistryContractAddress returns the DIN registry contract address.
// Priority is given first to the function argument `registryContractAddress` (if provided),
// then to the environment variable `DIN_REGISTRY_CONTRACT_ADDRESS`, and finally to a
// default address if neither is available.
func getDinRegistryContractAddress(registryContractAddress string) string {
	envContractAddress := os.Getenv("DIN_REGISTRY_CONTRACT_ADDRESS")
	if registryContractAddress != "" {
		return registryContractAddress
	} else if envContractAddress != "" {
		return envContractAddress

	}

	// default din contract address
	return "0xc55967876ff800d67400b6375eb5bb2592b491fa"
}

// convertNetworkName converts the network name to a format that can be used in the DIN proxy
func convertNetworkName(name string) string {
	// Special cases for ethereum and polygon
	nameMap := map[string]string{
		"ethereum://mainnet": "eth",
		"ethereum://holesky": "holesky",
		"polygon://mainnet":  "polygon",
	}

	val, ok := nameMap[name]
	if ok {
		return val
	}

	parts := strings.Split(name, "://")
	result := strings.Join(parts, "-")
	return result
}

// Printer Functions for CLI usage
func (d *DinClient) KickTheTires(network string) error {
	err := d.PrintGetAllMethodsByNetwork(network)
	if err != nil {
		return err
	}
	err = d.PrintListAllMethodsByNetwork(network)
	if err != nil {
		return err
	}
	err = d.PrintGetAllNetworks()
	if err != nil {
		return err
	}
	err = d.PrintGetNetworkCapabilities(network)
	if err != nil {
		return err
	}
	err = d.PrintGetAllProviders(network)
	if err != nil {
		return err
	}

	return nil
}
