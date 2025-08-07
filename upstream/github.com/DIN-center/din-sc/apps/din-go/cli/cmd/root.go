package dincli

import (
	"log"
	"os"

	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var rootCmd = &cobra.Command{
	Use:   "din-cli",
	Short: "DIN Registry CLI",
	Long:  `A CLI tool for managing networks, providers, and their states in the DIN registry.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Read contract address and RPC URL
		contractAddr, err := readContractAddr()
		if err != nil {
			log.Fatal(err)
		}
		rpcURL, err := readRPCURL()
		if err != nil {
			log.Fatal(err)
		}
		// Initialize the DinClient
		dinClient, err = din.NewDinClient(zap.NewNop(), rpcURL, contractAddr)
		if err != nil {
			log.Fatal(err)
		}

		// Initialize the output formatter
		if jsonOutput {
			outputFormatter = NewJSONFormatter()
		} else {
			outputFormatter = NewPrettyPrinterFormatter()
		}

		// Initialize the gas estimator
		feesEstimator = NewEVMFeesEstimator(dinClient.GetEthereumRpcClient())
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dinRegistryContractAddr, "din-registry-contract-addr", "", "The DIN Registry contract address, takes precedence over environment variable")
	rootCmd.PersistentFlags().StringVar(&rpcURL, "rpc-url", "", "The RPC URL to connect to, takes precedence over environment variable")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
