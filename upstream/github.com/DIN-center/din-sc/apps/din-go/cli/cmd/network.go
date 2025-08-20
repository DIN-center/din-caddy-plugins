package dincli

import (
	"fmt"
	"log"
	"strings"

	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/spf13/cobra"
)

var networkCmd = &cobra.Command{
	Use:   "network",
	Short: "Manage networks",
}

var listNetworksCmd = &cobra.Command{
	Use:   "list",
	Short: "List a single network if name is provided, otherwise list all networks",
	Run: func(cmd *cobra.Command, args []string) {
		err := doListNetworks()
		if err != nil {
			log.Fatal(err)
		}
	},
}

var setNetworkStatusCmd = &cobra.Command{
	Use:   "set-status",
	Short: "Set network status",
	Run: func(cmd *cobra.Command, args []string) {
		keystorePath, err := readKeystorePath()
		if err != nil {
			log.Fatal(err)
		}
		password, err := readKeystorePassword()
		if err != nil {
			log.Fatal(err)
		}
		tx, err := doSetNetworkStatus(keystorePath, password)
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

var setNetworkConfigCmd = &cobra.Command{
	Use:   "set-config",
	Short: "Set network config",
	Run: func(cmd *cobra.Command, args []string) {
		keystorePath, err := readKeystorePath()
		if err != nil {
			log.Fatal(err)
		}
		password, err := readKeystorePassword()
		if err != nil {
			log.Fatal(err)
		}
		tx, err := doSetNetworkConfig(keystorePath, password)
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
	// Add the network command to the root command
	rootCmd.AddCommand(networkCmd)

	// Define subcommands for the network command
	networkCmd.AddCommand(listNetworksCmd, setNetworkStatusCmd, setNetworkConfigCmd)

	// Define flags for the list subcommand
	listNetworksCmd.Flags().StringVar(&networkURI, "uri", "", "Network URI in the format of 'network://flavor'")
	listNetworksCmd.Flags().BoolVar(&networkListFull, "full", false, "Show all details of the network")

	// Define flags for the set-status subcommand
	setNetworkStatusCmd.Flags().Var(&networkStatus, "status", "Network status ("+strings.Join(func() []string {
		statuses := make([]string, len(din.NetworkStatusAll))
		for i, status := range din.NetworkStatusAll {
			statuses[i] = status.String()
		}
		return statuses
	}(), ", ")+")")
	setNetworkStatusCmd.Flags().StringVar(&networkURI, "uri", "", "Network URI in the format of 'network://flavor'")
	setNetworkStatusCmd.MarkFlagRequired("status")
	setNetworkStatusCmd.MarkFlagRequired("uri")
	addWriteFlags(setNetworkStatusCmd)
	// Define flags for the set-config subcommand
	setNetworkConfigCmd.Flags().StringVar(&networkURI, "uri", "", "Network URI in the format of 'network://flavor'")
	setNetworkConfigCmd.Flags().StringVar(&networkConfigJsonAsString, "config-json", "", "Network config in JSON format")
	setNetworkConfigCmd.MarkFlagRequired("uri")
	setNetworkConfigCmd.MarkFlagRequired("config-json")
	addWriteFlags(setNetworkConfigCmd)
}

// internal functions used in command execution
func doListNetworks() error {
	networkName := strings.TrimSpace(networkURI)
	opts := FormatOptions{}
	if networkListFull {
		opts = FormatOptions{
			ShowMethods:   true,
			ShowProviders: true,
			Verbose:       true,
		}
	}
	if networkName != "" {
		network, err := dinClient.GetNetworkByName(networkName)
		if err != nil {
			return fmt.Errorf("there is no network with name '%s' in the registry. Please remove the --uri flag to see all networks available", networkName)
		}

		outputFormatter.FormatNetwork(network, opts)

	} else {
		networks, err := dinClient.GetAllNetworks()
		if err != nil {
			return err
		}
		outputFormatter.FormatNetworks(networks, opts)
	}

	return nil
}

func doSetNetworkStatus(keystorePath string, keystorePassword string) (*types.Transaction, error) {
	auth, err := dinClient.CreateAuthorizedTransactor(keystorePath, keystorePassword)
	if err != nil {
		return nil, err
	}

	// Adjust transaction options before sending the transaction
	err = adjustTxOptions(auth)
	if err != nil {
		return nil, err
	}

	tx, err := dinClient.SetNetworkStatus(auth, networkURI, networkStatus)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func doSetNetworkConfig(keystorePath string, keystorePassword string) (*types.Transaction, error) {
	newConfig, err := buildNetworkConfigFromUserInput(networkConfigJsonAsString, dinClient, networkURI)
	if err != nil {
		return nil, err
	}

	auth, err := dinClient.CreateAuthorizedTransactor(keystorePath, keystorePassword)
	if err != nil {
		return nil, err
	}

	// Adjust transaction options before sending the transaction
	err = adjustTxOptions(auth)
	if err != nil {
		return nil, err
	}

	tx, err := dinClient.SetNetworkConfig(auth, networkURI, *newConfig)
	if err != nil {
		return nil, err
	}

	return tx, nil
}
