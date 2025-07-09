package dincli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
	"github.com/ethereum/go-ethereum/core/types"
	"golang.org/x/term"
)

func readContractAddr() (string, error) {
	envContractAddr := os.Getenv("DIN_REGISTRY_CONTRACT_ADDR")
	if dinRegistryContractAddr == "" && envContractAddr == "" {
		return "", errors.New("DIN Registry contract address is required, pass it with --din-registry-contract-addr or set DIN_REGISTRY_CONTRACT_ADDR environment variable")
	}

	return getConfigValue(dinRegistryContractAddr, envContractAddr), nil
}

func readRPCURL() (string, error) {
	envRPCURL := os.Getenv("RPC_URL")
	if rpcURL == "" && envRPCURL == "" {
		return "", errors.New("a RPC URL is required, pass it with --rpc-url or set RPC_URL environment variable")
	}

	return getConfigValue(rpcURL, envRPCURL), nil
}

func readKeystorePath() (string, error) {
	envKeystorePath := os.Getenv("KEYSTORE_PATH")
	if keystorePath == "" && envKeystorePath == "" {
		return "", errors.New("keystore-path is required, pass it with --keystore-path or set KEYSTORE_PATH environment variable")
	}

	return getConfigValue(keystorePath, envKeystorePath), nil
}

func readKeystorePassword() (string, error) {
	// Create a context with timeout for security
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Channel to receive the password result
	resultChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// Get key password from stdin in a goroutine
	go func() {
		fmt.Print("Enter keystore password: ")
		password, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			errChan <- errors.New("failed to read password: " + err.Error())
			return
		}
		fmt.Println() // Add newline after password input
		resultChan <- string(password)
	}()

	// Wait for either the password or timeout
	select {
	case password := <-resultChan:
		return password, nil
	case err := <-errChan:
		return "", err
	case <-ctx.Done():
		return "", errors.New("password input timed out after 60 seconds")
	}
}

func filterProviders(providers []*din.Provider, providerName string) []*din.Provider {
	filteredProviders := make([]*din.Provider, 0)
	for _, provider := range providers {
		if strings.EqualFold(provider.Name, providerName) {
			filteredProviders = append(filteredProviders, provider)
		}
	}
	return filteredProviders
}

// Builds a NetworkOperationsConfig from a JSON string with only the fields that are provided in the JSON string
// The other fields are set to current values retrieved from the given network URI
func buildNetworkConfigFromUserInput(configJsonAsString string, dinClient din.IDinClient, networkURI string) (*din.NetworkOperationsConfig, error) {
	// Define mirror struct for the NetworkOperationsConfig struct but with pointers
	// This is necessary because the JSON string may not contain all fields, and we want to be able to set only the provided fields
	type NetworkConfigInput struct {
		HealthcheckMethod       *string `json:"health_check_method,omitempty"`
		HealthcheckIntervalSec  *uint8  `json:"health_check_interval_sec,omitempty"`
		ChainIdMethod           *string `json:"chain_id_method,omitempty"`
		GetBlockByNumberMethod  *string `json:"get_block_by_number_method,omitempty"`
		CallContractMethod      *string `json:"call_contract_method,omitempty"`
		BlockLagLimit           *uint8  `json:"block_lag_limit,omitempty"`
		BlockJumpLimit          *uint8  `json:"block_jump_limit,omitempty"`
		RequestAttemptCount     *uint8  `json:"request_attempt_count,omitempty"`
		MaxRequestPayloadSizeKb *uint16 `json:"max_request_payload_size_kb,omitempty"`
		RegistryBlockEpoch      *uint32 `json:"registry_block_epoch,omitempty"`
		ArchiveEnabled          *bool   `json:"archive_enabled,omitempty"`
		ChainId                 *string `json:"chain_id,omitempty"`
	}

	// Parse the JSON string into the temporary struct
	var configInput NetworkConfigInput
	err := json.Unmarshal([]byte(configJsonAsString), &configInput)
	if err != nil {
		return nil, errors.New("failed to parse JSON config: " + err.Error())
	}

	network, err := dinClient.GetNetworkByName(networkURI)
	if err != nil {
		return nil, errors.New("failed to get network: " + err.Error())
	}

	// Copy the current config to the new config (deep copy of the entire struct.
	newConfig := *network.NetworkConfig

	// Print which fields were provided in the JSON
	modified := false
	if configInput.HealthcheckMethod != nil {
		newConfig.HealthcheckMethod = *configInput.HealthcheckMethod
		modified = true
	}
	if configInput.HealthcheckIntervalSec != nil {
		newConfig.HealthcheckIntervalSec = *configInput.HealthcheckIntervalSec
		modified = true
	}
	if configInput.ChainIdMethod != nil {
		newConfig.ChainIdMethod = *configInput.ChainIdMethod
		modified = true
	}
	if configInput.GetBlockByNumberMethod != nil {
		newConfig.GetBlockByNumberMethod = *configInput.GetBlockByNumberMethod
		modified = true
	}
	if configInput.CallContractMethod != nil {
		newConfig.CallContractMethod = *configInput.CallContractMethod
		modified = true
	}
	if configInput.BlockLagLimit != nil {
		newConfig.BlockLagLimit = *configInput.BlockLagLimit
		modified = true
	}
	if configInput.BlockJumpLimit != nil {
		newConfig.BlockJumpLimit = *configInput.BlockJumpLimit
		modified = true
	}
	if configInput.RequestAttemptCount != nil {
		newConfig.RequestAttemptCount = *configInput.RequestAttemptCount
		modified = true
	}
	if configInput.MaxRequestPayloadSizeKb != nil {
		newConfig.MaxRequestPayloadSizeKb = *configInput.MaxRequestPayloadSizeKb
		modified = true
	}
	if configInput.RegistryBlockEpoch != nil {
		newConfig.RegistryBlockEpoch = *configInput.RegistryBlockEpoch
		modified = true
	}
	if configInput.ArchiveEnabled != nil {
		newConfig.ArchiveEnabled = *configInput.ArchiveEnabled
		modified = true
	}
	if configInput.ChainId != nil {
		newConfig.ChainId = *configInput.ChainId
		modified = true
	}

	if modified {
		return &newConfig, nil
	}

	return nil, errors.New("nothing to update. Check the provided JSON fields names")
}

func getConfigValue(flagValue, envValue string) string {
	if trimmed := strings.TrimSpace(flagValue); trimmed != "" {
		return trimmed // value from flag takes precedence over env variable
	}
	return envValue
}

func handleTxConfirmation(tx *types.Transaction, dinClient din.IDinClient) error {

	fmt.Printf("Transaction pending: 0x%x\n", tx.Hash())

	var receipt *types.Receipt
	var err error

	for i := range maxTxConfirmationInSeconds {
		time.Sleep(1 * time.Second)
		receipt, err = dinClient.GetEthereumRpcClient().TransactionReceipt(context.Background(), tx.Hash())
		if err == nil {
			break
		}
		if i == maxTxConfirmationInSeconds-1 {
			return errors.New("failed call to TransactionReceipt after max retries")
		}
	}

	if receipt == nil {
		fmt.Println("Check the transaction on a block explorer")
	} else {
		if receipt.Status == types.ReceiptStatusSuccessful {
			fmt.Printf("✅ [SUCCESS]")
			fmt.Printf("Block: %d\n", receipt.BlockNumber)
			gasUsed := receipt.GasUsed
			gasPrice := tx.GasPrice()
			totalCost := new(big.Int).Mul(new(big.Int).SetUint64(gasUsed), gasPrice)
			ethCost := new(big.Float).Quo(new(big.Float).SetInt(totalCost), new(big.Float).SetInt64(1e18))
			gweiPrice := new(big.Float).Quo(new(big.Float).SetInt(gasPrice), new(big.Float).SetInt64(1e9))
			fmt.Printf("Paid: %s ETH (%d gas * %s gwei)\n", ethCost.Text('f', 18), gasUsed, gweiPrice.Text('f', 9))
		} else {
			fmt.Printf("❌ [FAILED]")
		}
	}

	return nil
}
