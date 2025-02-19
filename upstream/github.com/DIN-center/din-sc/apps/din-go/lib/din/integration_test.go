package din

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/DIN-center/din-sc/apps/din-go/pkg/dinregistry"
)

var dinClient *DinClient

func setup(t *testing.T) {
	rpcURL := "http://localhost:8545"

	// Get the chain ID
	chainID, err := getEthChainId(rpcURL)
	if err != nil {
		log.Fatalf("Error getting chain ID: %v", err)
	}

	// Get the contract address from the anvil output
	anvilOutputPath := fmt.Sprintf("../../../din-sc/broadcast/Deploy.s.sol/%v/run-latest.json", *chainID)
	contractAddress, err := getContractAddressFromAnvil(anvilOutputPath)
	if err != nil {
		t.Fatalf("Error getting contract address from anvil output: %v", err)
	}

	// Create a new DinClient
	dinClient, err = NewDinClient(nil, rpcURL, contractAddress)
	if err != nil {
		t.Fatalf("Error creating DinClient: %v", err)
	}
}

// getEthChainId gets the chain ID of the Ethereum network
func getEthChainId(rpcURL string) (*int64, error) {
	// Create the JSON-RPC request
	requestBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_chainId",
		"params":  []interface{}{},
		"id":      1,
	}

	// Convert request to JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("Error marshalling JSON: %v", err)
	}

	// Make the HTTP POST request
	resp, err := http.Post(rpcURL, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("Error making HTTP POST request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading response body: %v", err)
	}

	// Parse the response
	var rpcResponse map[string]interface{}
	err = json.Unmarshal(body, &rpcResponse)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshalling JSON: %v", err)
	}

	// Check for errors in the response
	if rpcResponse["error"] != nil {
		return nil, fmt.Errorf("Error in JSON response: %v", rpcResponse["error"])
	}

	// Get the chain ID hexadecimal
	chainId, ok := rpcResponse["result"].(string)
	if !ok {
		return nil, fmt.Errorf("Error getting result from JSON")
	}

	// Convert the hexadecimal string to an int64
	chainIdInt, err := strconv.ParseInt(chainId[2:], 16, 64)
	if err != nil {
		return nil, fmt.Errorf("Error converting chain ID to int: %v", err)
	}

	return &chainIdInt, nil
}

// getContractAddressFromAnvil gets the contract address from the Anvil output
func getContractAddressFromAnvil(dinRegistryDataPath string) (string, error) {
	var dinRegistryData map[string]interface{}

	// Open the JSON file
	file, err := os.Open(dinRegistryDataPath)
	if err != nil {
		return "", fmt.Errorf("Error opening JSON file: %v", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)

	err = decoder.Decode(&dinRegistryData)
	if err != nil {
		return "", fmt.Errorf("Error decoding JSON file: %v", err)
	}

	// Get the transactions
	transactions, ok := dinRegistryData["transactions"].([]interface{})
	if !ok {
		return "", fmt.Errorf("Error getting transactions from JSON")
	}

	// Check if any transactions were found
	if len(transactions) == 0 {
		return "", fmt.Errorf("No transactions found in JSON")
	}

	// Get the first transaction
	transaction, ok := transactions[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("Error getting transaction from JSON")
	}

	// Get the contract address
	contractAddress, ok := transaction["contractAddress"].(string)
	if !ok {
		return "", fmt.Errorf("Error getting contract address from JSON")
	}

	return contractAddress, nil
}

// TestGetRegistryData tests the DinGo library with deployed contracts
func TestGetRegistryData(t *testing.T) {
	setup(t) // Call setup to initialize dinClient

	// get registry data
	registryData, err := dinClient.GetRegistryData()
	if err != nil {
		t.Fatalf("Error getting registry data: %v", err)
	}

	validateRegistryData(t, registryData)

	// get network data
	for _, network := range registryData.Networks {
		validateNetworkData(t, network)

		for _, provider := range network.Providers {
			validateProviderData(t, provider)

			for _, networkService := range provider.NetworkServices {
				validateNetworkService(t, networkService)
			}
		}
	}
}

func validateRegistryData(t *testing.T, registryData *DinRegistryData) {
	if registryData == nil {
		t.Fatalf("Registry data is nil")
	}

	if len(registryData.Networks) < 1 {
		t.Fatalf("No networks found in registry")
	}

	if registryData.Networks == nil {
		t.Fatalf("Networks is nil")
	}
}

func validateNetworkData(t *testing.T, network *Network) {
	if network == nil {
		t.Fatalf("Network is nil")
	}

	if network.Address == "" {
		t.Fatalf("Network address is empty")
	}

	if network.Status == "" {
		t.Fatalf("Network status is empty")
	}

	if network.ProxyName == "" {
		t.Fatalf("Network proxy name is empty")
	}

	if network.Capabilities == nil {
		t.Fatalf("Network capabilities is nil")
	}

	if network.NetworkConfig == nil {
		t.Fatalf("Network config is nil")
	}

	if network.NetworkConfig.HealthcheckMethodBit == 0 {
		t.Fatalf("Healthcheck method bit is 0")
	}

	if network.NetworkConfig.HealthcheckIntervalSec == 0 {
		t.Fatalf("Healthcheck interval seconds is 0")
	}

	if network.NetworkConfig.CallContractMethodBit == 0 {
		t.Fatalf("Call contract method bit is 0")
	}

	if network.NetworkConfig.ChainIdMethodBit == 0 {
		t.Fatalf("Chain ID method bit is 0")
	}

	if network.NetworkConfig.ChainId == "" {
		t.Fatalf("Chain ID is empty")
	}

	if network.NetworkConfig.BlockJumpLimit == 0 {
		t.Fatalf("Block jump limit is 0")
	}

	if network.NetworkConfig.BlockLagLimit == 0 {
		t.Fatalf("Block lag limit is 0")
	}

	if network.NetworkConfig.RequestAttemptCount == 0 {
		t.Fatalf("Request attempt count is 0")
	}

	if network.NetworkConfig.MaxRequestPayloadSizeKb == 0 {
		t.Fatalf("Max request payload size is 0")
	}

	if len(network.Providers) < 1 {
		t.Fatalf("No providers found in network %v", network.Name)
	}
}

func validateProviderData(t *testing.T, provider *Provider) {
	if provider == nil {
		t.Fatalf("Provider is nil")
	}

	if provider.Name == "" {
		t.Fatalf("Provider name is empty")
	}

	if provider.Address == "" {
		t.Fatalf("Provider address is empty")
	}

	if provider.Owner == "" {
		t.Fatalf("Provider owner is empty")
	}

	if provider.AuthConfig == nil {
		t.Fatalf("Provider auth config is nil")
	}

	if provider.AuthConfig.Type == "" {
		t.Fatalf("Provider auth config type is empty")
	}

	if provider.AuthConfig.Type != dinregistry.None && provider.AuthConfig.Url == "" {
		t.Fatalf("Provider auth config URL is empty")
	}

	if len(provider.NetworkServices) < 1 {
		t.Fatalf("No network services found in provider %v", provider.Name)
	}

}

func validateNetworkService(t *testing.T, networkService *NetworkService) {
	if networkService == nil {
		t.Fatalf("Network service is nil")
	}

	if networkService.Address == "" {
		t.Fatalf("Network service address is empty")
	}

	if networkService.Status == "" {
		t.Fatalf("Network service status is empty")
	}

	if networkService.Url == "" {
		t.Fatalf("Network service URL is empty")
	}

	if len(networkService.Locations) < 1 {
		t.Fatalf("No locations found in network service %v", networkService.Url)
	}

	if networkService.Capabilities == nil {
		t.Fatalf("Network service capabilities is nil")
	}
}

func TestGetAllNetworks(t *testing.T) {
	setup(t) // Call setup to initialize dinClient

	// get registry data
	networks, err := dinClient.GetAllNetworks()
	if err != nil {
		t.Fatalf("Error getting registry data: %v", err)
	}

	if len(networks) < 1 {
		t.Fatalf("No networks found in registry")
	}

	for _, network := range networks {
		if network.Address == "" {
			t.Fatalf("Network address is empty")
		}

		if network.Status == "" {
			t.Fatalf("Network status is empty")
		}

		if network.ProxyName == "" {
			t.Fatalf("Network proxy name is empty")
		}

		if network.Name == "" {
			t.Fatalf("Network name is empty")
		}

		if network.Capabilities == nil {
			t.Fatalf("Network capabilities is nil")
		}

		if network.NetworkConfig == nil {
			t.Fatalf("Network config is nil")
		}

		if network.NetworkConfig.HealthcheckMethodBit == 0 {
			t.Fatalf("Healthcheck method bit is 0")
		}

		if network.NetworkConfig.HealthcheckIntervalSec == 0 {
			t.Fatalf("Healthcheck interval seconds is 0")
		}

		if network.NetworkConfig.CallContractMethodBit == 0 {
			t.Fatalf("Call contract method bit is 0")
		}

		if network.NetworkConfig.ChainIdMethodBit == 0 {
			t.Fatalf("Chain ID method bit is 0")
		}

		if network.NetworkConfig.ChainId == "" {
			t.Fatalf("Chain ID is empty")
		}

		if network.NetworkConfig.BlockJumpLimit == 0 {
			t.Fatalf("Block jump limit is 0")
		}

		if network.NetworkConfig.BlockLagLimit == 0 {
			t.Fatalf("Block lag limit is 0")
		}

		if network.NetworkConfig.RequestAttemptCount == 0 {
			t.Fatalf("Request attempt count is 0")
		}

		if network.NetworkConfig.MaxRequestPayloadSizeKb == 0 {
			t.Fatalf("Max request payload size is 0")
		}
	}
}
