package dincli

import (
	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
)

// Command flags
var (
	networkURI                 string
	providerAddr               string
	providerName               string
	providerStatus             din.ProviderStatus
	networkStatus              din.NetworkStatus
	networkListFull            bool
	networkConfigJsonAsString  string
	networkServiceAddr         string
	networkServiceStatus       din.NetworkServiceStatus
	keystorePath               string
	maxTxConfirmationInSeconds int  = 10
	dryRun                     bool = false
	gasPriceInWei              int64
	nonce                      int64
)

// Global flags
var (
	dinRegistryContractAddr string
	rpcURL                  string
	jsonOutput              bool
)

// Global variable to store the output formatter
var outputFormatter IOutputFormatter = NewPrettyPrinterFormatter()

// Global variable to store the DinClient
var dinClient din.IDinClient

// Global variable to store Fees Estimator
var feesEstimator IFeesEstimator
