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
		//Sanity checks come first
		if providerAddr == "" {
			log.Fatal("provider contract address is required")
		}
		//force to provide keystore path
		keystorePath, err := readKeystorePath()
		if err != nil {
			log.Fatal(err)
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

		//Surface checks done, proceed with removal
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
		//Sanity checks come first
		if providerAddr == "" {
			log.Fatal("provider contract address is required")
		}
		//force to provide keystore path
		keystorePath, err := readKeystorePath()
		if err != nil {
			log.Fatal(err)
		}

		// Check if provider exists
		providerAddr := common.HexToAddress(providerAddr)
		_, err = dinClient.GetProviderByAddress(providerAddr)
		if err != nil {
			log.Fatal("no provider found for this address: ", providerAddr.String())
		}

		//Surface checks done, proceed with status update
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

var removeNetworkServiceCmd = &cobra.Command{
	Use:   "remove-service",
	Short: "Remove a service from a provider",
	Run: func(cmd *cobra.Command, args []string) {
		//Sanity checks come first
		if providerAddr == "" {
			log.Fatal("provider contract address is required")
		}
		if networkServiceAddr == "" {
			log.Fatal("network service contract address is required")
		}

		//force to provide keystore path
		keystorePath, err := readKeystorePath()
		if err != nil {
			log.Fatal(err)
		}

		//Check if provider exists
		providerAddr := common.HexToAddress(providerAddr)
		_, err = dinClient.GetProviderByAddress(providerAddr)
		if err != nil {
			log.Fatal("no provider found for this address: ", providerAddr.String())
		}

		//Check if network service exists
		networkServiceAddr := common.HexToAddress(networkServiceAddr)
		_, err = dinClient.GetNetworkServiceByAddress(networkServiceAddr)
		if err != nil {
			log.Fatal("no network service found for this address: ", networkServiceAddr.String())
		}
		//Surface checks done, proceed with removal
		password, err := readKeystorePassword()
		if err != nil {
			log.Fatal(err)
		}

		tx, err := doRemoveNetworkService(keystorePath, password, providerAddr, networkServiceAddr)
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

var setNetworkServiceStatusCmd = &cobra.Command{
	Use:   "set-service-status",
	Short: "Set the status of a service",
	Run: func(cmd *cobra.Command, args []string) {
		//Sanity checks come first
		if networkServiceAddr == "" {
			log.Fatal("network service contract address is required")
		}

		//force to provide keystore path
		keystorePath, err := readKeystorePath()
		if err != nil {
			log.Fatal(err)
		}

		//Check if network service exists
		networkServiceAddr := common.HexToAddress(networkServiceAddr)
		_, err = dinClient.GetNetworkServiceByAddress(networkServiceAddr)
		if err != nil {
			log.Fatal("no service found for this address: ", networkServiceAddr.String())
		}

		//Surface checks done, proceed with status update
		password, err := readKeystorePassword()
		if err != nil {
			log.Fatal(err)
		}

		tx, err := doSetNetworkServiceStatus(keystorePath, password, networkServiceAddr)
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
	//Add providers command to root command
	rootCmd.AddCommand(providersCmd)

	//Add subcommands to providers command
	providersCmd.AddCommand(listProvidersCmd, removeProviderCmd, setProviderStatusCmd, removeNetworkServiceCmd, setNetworkServiceStatusCmd)

	// Define flags for the list providers command
	listProvidersCmd.Flags().StringVar(&networkURI, "network-uri", "", "Network URI")
	listProvidersCmd.Flags().StringVar(&providerName, "name", "", "Provider name")

	// Define flags for the remove provider command
	removeProviderCmd.Flags().StringVar(&providerAddr, "contract-addr", "", "Provider contract address to remove")
	removeProviderCmd.MarkFlagRequired("contract-addr")
	addWriteFlags(removeProviderCmd)

	// Define flags for the set provider status command
	setProviderStatusCmd.Flags().StringVar(&providerAddr, "contract-addr", "", "Provider contract address to set status for")
	setProviderStatusCmd.MarkFlagRequired("contract-addr")
	addWriteFlags(setProviderStatusCmd)
	setProviderStatusCmd.Flags().Var(&providerStatus, "status", "Provider status ("+strings.Join(func() []string {
		statuses := make([]string, len(din.ProviderStatusAll))
		for i, status := range din.ProviderStatusAll {
			statuses[i] = status.String()
		}
		return statuses
	}(), ", ")+")")
	setProviderStatusCmd.MarkFlagRequired("status")
	setProviderStatusCmd.MarkFlagRequired("contract-addr")

	//Define flags for remove network service command
	removeNetworkServiceCmd.Flags().StringVar(&providerAddr, "contract-addr", "", "Provider contract address to remove service from")
	removeNetworkServiceCmd.Flags().StringVar(&networkServiceAddr, "service-contract-addr", "", "Service contract address to remove")
	removeNetworkServiceCmd.MarkFlagRequired("contract-addr")
	removeNetworkServiceCmd.MarkFlagRequired("service-contract-addr")
	addWriteFlags(removeNetworkServiceCmd)

	//Define flags for set network service status command
	setNetworkServiceStatusCmd.Flags().StringVar(&networkServiceAddr, "service-contract-addr", "", "Service contract address to set status for")
	setNetworkServiceStatusCmd.Flags().Var(&networkServiceStatus, "status", "Service status ("+strings.Join(func() []string {
		statuses := make([]string, len(din.NetworkServiceStatusAll))
		for i, status := range din.NetworkServiceStatusAll {
			statuses[i] = status.String()
		}
		return statuses
	}(), ", ")+")")
	setNetworkServiceStatusCmd.MarkFlagRequired("status")
	setNetworkServiceStatusCmd.MarkFlagRequired("service-contract-addr")
	addWriteFlags(setNetworkServiceStatusCmd)
}

// internal functions used in command execution
func doListProviders() error {
	var providers []*din.Provider
	var err error

	// Get providers based on network URI or all providers
	if networkURI != "" {
		providers, err = dinClient.GetAllProvidersByNetwork(networkURI)
		if err != nil {
			return fmt.Errorf("there is no provider registered for this network")
		}
	} else {
		providers, err = dinClient.GetAllProviders()
		if err != nil {
			return err
		}
	}

	// Filter by provider name if specified
	if providerName != "" {
		providers = filterProviders(providers, providerName)
	}

	// Format output with appropriate verbosity
	verbose := providerName != ""
	outputFormatter.FormatProviders(providers, FormatOptions{
		Verbose: verbose,
	})

	return nil
}

func doSetProviderStatus(keystorePath string, keystorePassword string, providerAddr common.Address) (*types.Transaction, error) {
	// Try to create authorized transactor
	auth, err := dinClient.CreateAuthorizedTransactor(keystorePath, keystorePassword)
	if err != nil {
		return nil, err
	}

	// Adjust transaction options before sending the transaction
	err = adjustTxOptions(auth)
	if err != nil {
		return nil, err
	}

	// Update DIN Registry
	tx, err := dinClient.SetProviderStatus(auth, providerAddr, providerStatus)
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

	// Adjust transaction options before sending the transaction
	err = adjustTxOptions(auth)
	if err != nil {
		return nil, err
	}

	tx, err := dinClient.RemoveProvider(auth, providerAddr)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func doRemoveNetworkService(keystorePath string, keystorePassword string, providerAddr common.Address, networkServiceAddr common.Address) (*types.Transaction, error) {
	auth, err := dinClient.CreateAuthorizedTransactor(keystorePath, keystorePassword)
	if err != nil {
		return nil, err
	}

	// Adjust transaction options before sending the transaction
	err = adjustTxOptions(auth)
	if err != nil {
		return nil, err
	}

	tx, err := dinClient.RemoveNetworkService(auth, providerAddr, networkServiceAddr)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func doSetNetworkServiceStatus(keystorePath string, keystorePassword string, networkServiceAddr common.Address) (*types.Transaction, error) {
	auth, err := dinClient.CreateAuthorizedTransactor(keystorePath, keystorePassword)
	if err != nil {
		return nil, err
	}

	// Adjust transaction options before sending the transaction
	err = adjustTxOptions(auth)
	if err != nil {
		return nil, err
	}

	tx, err := dinClient.SetNetworkServiceStatus(auth, networkServiceAddr, networkServiceStatus)
	if err != nil {
		return nil, err
	}
	return tx, nil
}
