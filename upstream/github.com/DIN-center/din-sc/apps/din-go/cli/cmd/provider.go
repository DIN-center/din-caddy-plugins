package dincli

import (
	"fmt"
	"log"
	"strings"

	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/spf13/cobra"
)

var providersCmd = &cobra.Command{
	Use:   "provider",
	Short: "Manage providers",
}

var listProvidersCmd = &cobra.Command{
	Use:   "list",
	Short: "List providers registered in the registry or for a specific network/provider",
	Run: func(cmd *cobra.Command, args []string) {
		err := doListProviders()
		if err != nil {
			log.Fatal(err)
		}
	},
}

var removeProviderCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a provider from the registry by its contract address",
	Run: func(cmd *cobra.Command, args []string) {
		if providerAddr == "" {
			log.Fatal("provider contract address is required")
		}

		providerAddr := common.HexToAddress(providerAddr)
		provider, err := dinClient.GetProviderByAddress(providerAddr)
		if err != nil {
			log.Fatal("No provider found for this address")
		}

		if len(provider.NetworkServices) > 0 {
			fmt.Printf("There are %d network services registered for this provider (%s). Are you sure you want to remove it? (y/n)\n", len(provider.NetworkServices), provider.Name)
			var confirm string
			fmt.Scanln(&confirm)
			if confirm != "y" {
				fmt.Println("Provider removal cancelled")
				return
			}
		}
		keystorePath, err := readKeystorePath()
		if err != nil {
			log.Fatal(err)
		}
		password, err := readKeystorePassword()
		if err != nil {
			log.Fatal(err)
		}
		tx, err := doRemoveProvider(keystorePath, password, providerAddr)
		if err != nil {
			log.Fatal(err)
		}
		// Wait for transaction confirmation and display transaction details
		err = handleTxConfirmation(tx, dinClient)
		if err != nil {
			log.Fatal(err)
		}
	},
}

var setProviderStatusCmd = &cobra.Command{
	Use:   "set-status",
	Short: "Set the status of a provider",
	Run: func(cmd *cobra.Command, args []string) {
		// Check if provider exists
		providerAddr := common.HexToAddress(providerAddr)
		_, err := dinClient.GetProviderByAddress(providerAddr)
		if err != nil {
			log.Fatal("no provider found for this address: ", providerAddr.String())
		}

		// Read keystore path and password
		keystorePath, err := readKeystorePath()
		if err != nil {
			log.Fatal(err)
		}
		password, err := readKeystorePassword()
		if err != nil {
			log.Fatal(err)
		}

		tx, err := doSetProviderStatus(keystorePath, password, providerAddr)
		if err != nil {
			log.Fatal(err)
		}

		// Wait for transaction confirmation and display transaction details
		err = handleTxConfirmation(tx, dinClient)
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(providersCmd)
	providersCmd.AddCommand(listProvidersCmd, removeProviderCmd, setProviderStatusCmd)
	listProvidersCmd.Flags().StringVar(&networkURI, "network-uri", "", "Network URI")
	listProvidersCmd.Flags().StringVar(&providerName, "name", "", "Provider name")

	removeProviderCmd.Flags().StringVar(&providerAddr, "contract-addr", "", "Provider contract address")
	removeProviderCmd.MarkFlagRequired("contract-addr")

	// Define flags for the set provider status command
	setProviderStatusCmd.Flags().StringVar(&providerAddr, "contract-addr", "", "Provider contract address")
	setProviderStatusCmd.MarkFlagRequired("contract-addr")
	setProviderStatusCmd.Flags().Var(&providerStatus, "status", "Provider status ("+strings.Join(func() []string {
		statuses := make([]string, len(din.ProviderStatusAll))
		for i, status := range din.ProviderStatusAll {
			statuses[i] = status.String()
		}
		return statuses
	}(), ", ")+")")
	setProviderStatusCmd.MarkFlagRequired("status")
	setProviderStatusCmd.MarkFlagRequired("contract-addr")
}

// internal functions used in command execution
func doListProviders() error {
	if networkURI != "" {
		providers, err := dinClient.GetAllProvidersByNetwork(networkURI)
		if err != nil {
			return fmt.Errorf("there is no provider registered for this network")
		}
		if providerName != "" {
			filteredProviders := filterProviders(providers, providerName)
			outputFormatter.FormatProviders(filteredProviders, FormatOptions{
				Verbose: true,
			})
		} else {
			outputFormatter.FormatProviders(providers, FormatOptions{
				Verbose: false,
			})
		}
	} else {
		providers, err := dinClient.GetAllProviders()
		if err != nil {
			return err
		}
		if providerName != "" {
			filteredProviders := filterProviders(providers, providerName)
			outputFormatter.FormatProviders(filteredProviders, FormatOptions{
				Verbose: true,
			})
		} else {
			outputFormatter.FormatProviders(providers, FormatOptions{
				Verbose: false,
			})
		}
	}
	return nil
}

func doSetProviderStatus(keystorePath string, keystorePassword string, providerAddr common.Address) (*types.Transaction, error) {
	// Try to create authorized transactor
	authTransactor, err := dinClient.CreateAuthorizedTransactor(keystorePath, keystorePassword)
	if err != nil {
		return nil, err
	}

	// Update DIN Registry
	tx, err := dinClient.SetProviderStatus(authTransactor, providerAddr, providerStatus)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func doRemoveProvider(keystorePath string, keystorePassword string, providerAddr common.Address) (*types.Transaction, error) {

	auth, err := dinClient.CreateAuthorizedTransactor(keystorePath, keystorePassword)
	if err != nil {
		return nil, err
	}

	tx, err := dinClient.RemoveProvider(auth, providerAddr)
	if err != nil {
		return nil, err
	}
	return tx, nil
}
