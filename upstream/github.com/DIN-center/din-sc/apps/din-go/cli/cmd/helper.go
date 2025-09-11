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
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/spf13/cobra"
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
		Handler                  *string `json:"handler,omitempty"`
		HealthcheckIntervalSec   *uint8  `json:"health_check_interval_sec,omitempty"`
		HealthcheckThreshold     *uint8  `json:"health_check_threshold,omitempty"`
		HealthcheckTimeout       *uint16 `json:"health_check_timeout,omitempty"`
		BlockLagLimit            *uint8  `json:"block_lag_limit,omitempty"`
		BlockJumpLimit           *uint8  `json:"block_jump_limit,omitempty"`
		RequestAttemptCount      *uint8  `json:"request_attempt_count,omitempty"`
		MaxRequestPayloadSizeKb  *uint16 `json:"max_request_payload_size_kb,omitempty"`
		RegistryBlockEpoch       *uint32 `json:"registry_block_epoch,omitempty"`
		ArchiveEnabled           *bool   `json:"archive_enabled,omitempty"`
		ProviderBlockHistorySize *uint16 `json:"provider_block_history_size,omitempty"`
		NetworkBlockHistorySize  *uint16 `json:"network_block_history_size,omitempty"`
		ChainId                  *string `json:"chain_id,omitempty"`
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
	if configInput.Handler != nil {
		newConfig.Handler = *configInput.Handler
		modified = true
	}
	if configInput.HealthcheckIntervalSec != nil {
		newConfig.HealthcheckIntervalSec = *configInput.HealthcheckIntervalSec
		modified = true
	}
	if configInput.HealthcheckThreshold != nil {
		newConfig.HealthcheckThreshold = *configInput.HealthcheckThreshold
		modified = true
	}
	if configInput.HealthcheckTimeout != nil {
		newConfig.HealthcheckTimeout = *configInput.HealthcheckTimeout
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
	if configInput.ProviderBlockHistorySize != nil {
		newConfig.ProviderBlockHistorySize = *configInput.ProviderBlockHistorySize
		modified = true
	}
	if configInput.NetworkBlockHistorySize != nil {
		newConfig.NetworkBlockHistorySize = *configInput.NetworkBlockHistorySize
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

	if !dryRun {
		fmt.Printf("⌛ Transaction pending: 0x%x\n", tx.Hash())
		var receipt *types.Receipt
		rpc_client := dinClient.GetEthereumRpcClient()
		trials := 0
		for trials < maxTxConfirmationInSeconds {
			time.Sleep(1 * time.Second)
			receipt, _ = rpc_client.TransactionReceipt(context.Background(), tx.Hash())
			trials++
			// Found the receipt, break the loop
			if receipt != nil {
				break
			}
		}

		if receipt == nil {
			fmt.Println("👀 Please check the transaction status on a block explorer")
		} else {
			if receipt.Status == types.ReceiptStatusSuccessful {
				fmt.Printf("✅ [SUCCESS]")
				fmt.Printf("Block: %d\n", receipt.BlockNumber)
				gasUsed := receipt.GasUsed
				effectiveGasPrice := receipt.EffectiveGasPrice
				totalCost := new(big.Int).Mul(new(big.Int).SetUint64(gasUsed), effectiveGasPrice)
				ethCost := new(big.Float).Quo(new(big.Float).SetInt(totalCost), new(big.Float).SetInt64(1e18))
				effectiveGweiPrice := new(big.Float).Quo(new(big.Float).SetInt(effectiveGasPrice), new(big.Float).SetInt64(1e9))
				fmt.Printf("Paid: %s ETH (%d gas * %s gwei)\n", ethCost.Text('f', 18), gasUsed, effectiveGweiPrice.Text('f', 9))
			} else {
				fmt.Printf("❌ [FAILED]")
			}
		}
	} else {
		fmt.Printf("🧪 Not sending transaction: 0x%x (dry-run mode)\n", tx.Hash())

		if tx.Type() < types.DynamicFeeTxType { // Legacy tx type
			ethCost := new(big.Float).Quo(new(big.Float).SetInt(tx.Cost()), new(big.Float).SetInt64(1e18))
			gweiPrice := new(big.Float).Quo(new(big.Float).SetInt(tx.GasPrice()), new(big.Float).SetInt64(1e9))
			fmt.Printf("(Legacy)Transaction would have cost at max: %s ETH (%d gas * %s gwei)\n", ethCost.Text('f', 18), tx.Gas(), gweiPrice.Text('f', 9))
		} else {
			totalGasPrice := new(big.Int).Add(tx.GasFeeCap(), tx.GasTipCap())
			txMaxGas := tx.Gas()
			totalGasCost := new(big.Int).Mul(totalGasPrice, big.NewInt(int64(txMaxGas)))
			totalEthCost := new(big.Float).Quo(new(big.Float).SetInt(totalGasCost), new(big.Float).SetInt64(1e18))
			totalGweiPrice := new(big.Float).Quo(new(big.Float).SetInt(totalGasPrice), new(big.Float).SetInt64(1e9))
			fmt.Printf("Transaction would have cost at max: %s ETH (%d gas * %s gwei)\n", totalEthCost.Text('f', 18), txMaxGas, totalGweiPrice.Text('f', 9))
		}
	}

	return nil
}

func addWriteFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&keystorePath, "keystore-path", "", "The path to the keystore file (wallet credentials), takes precedence over environment variable")
	cmd.Flags().IntVar(&maxTxConfirmationInSeconds, "tx-confirmation-sec", 10, "The maximum number of seconds to wait for a transaction confirmation.")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "If true, the transaction will not be sent to the network.")
	cmd.Flags().Int64Var(&gasPriceInWei, "gas-price", 0, "The gas price (in wei) to use for the transaction, if not provided, the gas price from the network will be used.")
	cmd.Flags().Int64Var(&nonce, "nonce", 0, "The nonce to use for the transaction (default is the next available nonce).")
}

func adjustTxOptions(txOptions *bind.TransactOpts) error {
	// Set the nonce if provided
	if nonce != 0 {
		txOptions.Nonce = big.NewInt(nonce)
	}

	// Set the gas price if provided, otherwise suggest it from the network
	if gasPriceInWei != 0 {
		txOptions.GasPrice = big.NewInt(gasPriceInWei)
	} else {
		if feesEstimator == nil {
			return errors.New("fees estimator not initialized")
		}
		feesData, err := feesEstimator.EstimateFees()
		if err != nil {
			return err
		}

		if feesData.GasPrice != nil {
			gweiPrice := new(big.Float).Quo(new(big.Float).SetInt(feesData.GasPrice), new(big.Float).SetInt64(1e9))
			fmt.Printf("⛽ Gas price suggested by the network: %s gwei\n", gweiPrice.Text('f', 9))
			txOptions.GasPrice = feesData.GasPrice
		} else if feesData.GasFeeCap != nil {
			gweiBaseFee := new(big.Float).Quo(new(big.Float).SetInt(feesData.GasFeeCap), new(big.Float).SetInt64(1e9))
			gweiTip := new(big.Float).Quo(new(big.Float).SetInt(feesData.GasTipCap), new(big.Float).SetInt64(1e9))
			fmt.Printf("⛽ Max Base Fee (suggested): %s gwei,  Max Priority Fee (suggested tip): %s gwei\n", gweiBaseFee.Text('f', 9), gweiTip.Text('f', 9))
			txOptions.GasFeeCap = feesData.GasFeeCap
			txOptions.GasTipCap = feesData.GasTipCap
		} else {
			return errors.New("no gas price or gas fee cap provided by the fees estimator")
		}
	}

	// Set the dry run flag if provided (default is false)
	txOptions.NoSend = dryRun
	return nil
}
