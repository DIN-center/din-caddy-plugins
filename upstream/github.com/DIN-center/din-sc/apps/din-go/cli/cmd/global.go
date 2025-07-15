package dincli

import (
	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
)

// Command flags
var (
	networkURI                string
	providerAddr              string
	providerName              string
	providerStatus            din.ProviderStatus
	networkStatus             din.NetworkStatus
	networkListFull           bool
	networkConfigJsonAsString string
	networkServiceAddr        string
	networkServiceStatus      din.NetworkServiceStatus
)

// Global flags
var (
	dinRegistryContractAddr    string
	rpcURL                     string
	keystorePath               string
	jsonOutput                 bool
	maxTxConfirmationInSeconds int = 10
)

// Global variable to store the output formatter
var outputFormatter IOutputFormatter = NewPrettyPrinterFormatter()

// Global variable to store the DinClient
var dinClient din.IDinClient
