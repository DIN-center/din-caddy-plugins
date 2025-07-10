package modules

import (
	"fmt"
	"strings"
	"time"

	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/web3"
	din "github.com/DIN-center/din-sc/apps/din-go/lib/din"

	"go.uber.org/zap"
)

// syncRegistryWithLatestBlock checks the latest block number from the linea network and updates the middleware object with the latest registry data if the block number difference is greater than or equal to the epoch
func (d *DinMiddleware) syncRegistryWithLatestBlock(web3Client web3.Web3Client) {
	// Get the latest block number from the linea network
	latestBlockNumber, err := web3Client.LatestBlockNumber()
	if err != nil {
		d.logger.Error("Failed to get latest block number", zap.Error(err))
		return
	}

	// Calculate the latest block floor by epoch. for example if the current block number is 55 and the epoch is 10, then the latest block floor by epoch is 50.
	latestBlockFloorByEpoch := latestBlockNumber - (latestBlockNumber % d.RegistryBlockEpoch)

	d.logger.Debug("Checking block number for registry sync", zap.Uint64("block_epoch", d.RegistryBlockEpoch),
		zap.Uint64("latest_linea_block_number", latestBlockNumber), zap.Uint64("latest_block_floor_by_epoch", latestBlockFloorByEpoch),
		zap.Uint64("last_updated_block_number", d.registryLastUpdatedEpochBlockNumber), zap.Uint64("difference", latestBlockFloorByEpoch-d.registryLastUpdatedEpochBlockNumber),
	)

	// If the difference between the latest block floor by epoch and the last updated block number is greater than or equal to the epoch, then update the networks and providers.
	if latestBlockFloorByEpoch-d.registryLastUpdatedEpochBlockNumber >= d.RegistryBlockEpoch {
		registryData, err := d.DingoClient.GetRegistryData()
		if err != nil {
			d.logger.Error("Failed to get data from registry", zap.Error(err))
			return
		}
		d.processRegistryData(registryData)

		// Update the last updated block number
		d.registryLastUpdatedEpochBlockNumber = latestBlockFloorByEpoch
	}
}

// processRegistryData processes the registry data and updates the middleware object with the registry data
func (d *DinMiddleware) processRegistryData(registryData *din.DinRegistryData) {
	if registryData == nil {
		d.logger.Error("Registry data is nil. Check registry connection")
		return
	}
	// Lock the middleware object to prevent race condition when updating the networks and providers
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Loop through the networks in the din registry
	for _, regNetwork := range registryData.Networks {
		// For each network, check if the network is provisioned, if not, skip the network
		if regNetwork.NetworkConfig == nil {
			continue
		}

		// Check if the network exists in the local network list within the middleware object
		network, ok := d.Networks[regNetwork.ProxyName]
		if !ok {
			if regNetwork.Status != din.NetworkStatusActive {
				d.logger.Debug("Network is not active, skipping", zap.String("network", regNetwork.ProxyName))
				continue
			}
			// If the network does not exist in the middleware object, then create a new network and add it to the middleware object
			d.addNetworkWithRegistryData(regNetwork)
		} else {
			// If the network exists in the middleware object, check to see if the registry version is active or not,
			if regNetwork.Status != din.NetworkStatusActive {
				// Delete the network for now if it is not active
				d.logger.Debug("Network is not active, removing from middleware: ", zap.String("network", regNetwork.ProxyName))
				delete(d.Networks, regNetwork.ProxyName)
				continue
			}
			// if active, update the existing network in place with the registry data
			d.updateNetworkWithRegistryData(regNetwork, network)
		}
	}
}

// addNetworkWithRegistryData creates a new network object from the registry network data and adds it to the middleware object
func (d *DinMiddleware) addNetworkWithRegistryData(regNetwork *din.Network) {
	network := NewNetwork(regNetwork.ProxyName, d.Env, d.CaddyPort)
	network = d.syncNetworkConfig(regNetwork.NetworkConfig, network)

	httpClient := din_http.NewHTTPClient(time.Duration(network.HCTimeout) * time.Second)
	network.HttpClient = httpClient
	network.logger = d.logger
	network.PrometheusClient = d.PrometheusClient
	network.machineID = d.machineID

	for _, regProvider := range regNetwork.Providers {
		for _, networkService := range regProvider.NetworkServices {
			if networkService.Status != din.NetworkServiceStatusActive {
				d.logger.Debug("Network service is not active", zap.String("network_service", networkService.Url))
				continue
			}
			// Create a new provider object
			provider, err := NewProvider(networkService.Url)
			if err != nil {
				d.logger.Error("Failed to create new provider object", zap.Error(err))
				continue
			}

			provider, err = d.createNewProvider(provider, regProvider.AuthConfig, networkService)
			if err != nil {
				d.logger.Error("Failed to create new provider", zap.Error(err))
				continue
			}

			// Add the provider to the network object
			network.Providers[provider.host] = provider
		}
	}
	if len(network.Providers) == 0 {
		d.logger.Debug("Network has no active providers", zap.String("network", network.Name))
		return
	}
	// Add the network to the middleware object
	d.Networks[network.Name] = network

	// Start the healthcheck for the network if the middleware is not in test mode
	if !d.testMode {
		network.startHealthcheck()
		d.logger.Info("Starting healthcheck for registry network", zap.String("network", network.Name))
	}
}

// updateNetworkWithRegistryData updates the network object in the middleware object with the latest registry network data
func (d *DinMiddleware) updateNetworkWithRegistryData(regNetwork *din.Network, newNetwork *network) {
	// Sync the network config data from the registry network to the copied network object
	newNetwork = d.syncNetworkConfig(regNetwork.NetworkConfig, newNetwork)

	// Loop through the providers/network services in the registry network and update the copied network.providers map with the registry provider data
	for _, regProvider := range regNetwork.Providers {
		for _, networkService := range regProvider.NetworkServices {
			// Create a provider object
			newProvider, err := NewProvider(networkService.Url)
			if err != nil {
				d.logger.Error("Failed to create new provider object", zap.Error(err))
				continue
			}

			// check to see if the provider exists in the local network object
			_, ok := newNetwork.Providers[newProvider.host]
			if !ok {
				// if the provider doesn't exist
				// check if the network service is active, if not, skip the provider
				if networkService.Status != din.NetworkServiceStatusActive {
					d.logger.Debug("Network service is not active", zap.String("network_service", networkService.Url))
					continue
				}
				// create a new provider object and add it to the copied network object
				newProvider, err := d.createNewProvider(newProvider, regProvider.AuthConfig, networkService)
				if err != nil {
					d.logger.Error("Failed to create new provider", zap.Error(err))
					continue
				}

				// add the new provider to the copied network object
				newNetwork.Providers[newProvider.host] = newProvider
			} else {
				// if the provider exists in the copied network object,
				// check if the network service is active, if not, don't update the provider data and remove the provider from the copied network object
				if networkService.Status != din.NetworkServiceStatusActive {
					delete(newNetwork.Providers, newProvider.host)
					d.logger.Debug("Network service is not active", zap.String("network_service", networkService.Url))
					continue
				}
				// if the provider auth url is different, then update the provider auth url on the middleware object
				oldProvider := d.Networks[newNetwork.Name].Providers[newProvider.host]
				if regProvider.AuthConfig != nil && oldProvider.Auth != nil && oldProvider.Auth.ProviderURL != regProvider.AuthConfig.Url {
					d.Networks[newNetwork.Name].Providers[newProvider.host].Auth.ProviderURL = regProvider.AuthConfig.Url
				}
			}
		}
	}

	// safely update the middleware network object with the copied network data.
	d.updateNetworkData(newNetwork)
}

// syncNetworkConfig updates the network object with the registry network config data
func (d *DinMiddleware) syncNetworkConfig(regNetworkConfig *din.NetworkOperationsConfig, network *network) *network {
	// Get the healthcheck method name from the registry, usually eth_blockNumber or similar
	registryHCMethod := regNetworkConfig.HealthcheckMethod

	// Get the chain ID method name from the registry, usually eth_chainId or similar
	registryChainIdMethod := regNetworkConfig.ChainIdMethod

	// Get the call contract method name from the registry, usually eth_call or similar
	registryCallContractMethod := regNetworkConfig.CallContractMethod

	// Update Chain ID if changed
	if regNetworkConfig.ChainId != "" && regNetworkConfig.ChainId != network.ChainId {
		d.logger.Debug("Setting network chain Id",
			zap.String("network", network.Name),
			zap.String("chain_id", regNetworkConfig.ChainId))
		network.ChainId = regNetworkConfig.ChainId
	}

	// Update Healthcheck Method if changed
	if registryHCMethod != "" && registryHCMethod != network.HCMethod {
		d.logger.Debug("Setting network healthcheck method",
			zap.String("network", network.Name),
			zap.String("healthcheck_method", registryHCMethod))
		network.HCMethod = registryHCMethod
	}

	// Update Chain ID Method if changed
	if registryChainIdMethod != "" && registryChainIdMethod != network.ChainIdMethod {
		d.logger.Debug("Setting network chain ID method",
			zap.String("network", network.Name),
			zap.String("chain_id_method", registryChainIdMethod))
		network.ChainIdMethod = registryChainIdMethod
	}

	// Update Call Contract Method if changed
	if registryCallContractMethod != "" && registryCallContractMethod != network.CallContractMethod {
		d.logger.Debug("Setting network call contract method",
			zap.String("network", network.Name),
			zap.String("call_contract_method", registryCallContractMethod))
		network.CallContractMethod = registryCallContractMethod
	}

	// Update Healthcheck Interval if changed
	hcInterval := int(regNetworkConfig.HealthcheckIntervalSec)
	if hcInterval != 0 && hcInterval != network.HCInterval {
		d.logger.Debug("Setting network healthcheck interval",
			zap.String("network", network.Name),
			zap.Int("interval", hcInterval))
		network.HCInterval = hcInterval
	}

	// Update Block Lag Limit if changed
	blockLagLimit := int64(regNetworkConfig.BlockLagLimit)
	if blockLagLimit != 0 && blockLagLimit != network.BlockLagLimit {
		d.logger.Debug("Setting network block lag limit",
			zap.String("network", network.Name),
			zap.Int64("block_lag_limit", blockLagLimit))
		network.BlockLagLimit = blockLagLimit
	}

	// Update Block Jump Limit if changed
	blockJumpLimit := int64(regNetworkConfig.BlockJumpLimit)
	if blockJumpLimit != 0 && blockJumpLimit != network.BlockJumpLimit {
		d.logger.Debug("Setting network block jump limit",
			zap.String("network", network.Name),
			zap.Int64("block_jump_limit", blockJumpLimit))
		network.BlockJumpLimit = blockJumpLimit
	}

	// Update Max Request Payload Size if changed
	maxPayloadSize := int64(regNetworkConfig.MaxRequestPayloadSizeKb)
	if maxPayloadSize != 0 && maxPayloadSize != network.MaxRequestPayloadSizeKB {
		d.logger.Debug("Setting network max request payload size",
			zap.String("network", network.Name),
			zap.Int64("max_payload_size_kb", maxPayloadSize))
		network.MaxRequestPayloadSizeKB = maxPayloadSize
	}

	// Update Request Attempt Count if changed
	requestAttempts := int(regNetworkConfig.RequestAttemptCount)
	if requestAttempts != 0 && requestAttempts != network.RequestAttemptCount {
		d.logger.Debug("Setting network request attempt count",
			zap.String("network", network.Name),
			zap.Int("request_attempts", requestAttempts))
		network.RequestAttemptCount = requestAttempts
	}

	// Update Archive Enabled if changed
	archiveEnabled := regNetworkConfig.ArchiveEnabled
	if archiveEnabled != network.ArchiveEnabled {
		d.logger.Debug("Setting network archive enabled",
			zap.String("network", network.Name),
			zap.Bool("archive_enabled", archiveEnabled))
		network.ArchiveEnabled = archiveEnabled
	}

	return network
}

// createNewProvider creates a new provider object and initializes the provider with the network service address
func (d *DinMiddleware) createNewProvider(provider *provider, regAuthConfig *din.ProviderAuthConfig, regNetworkService *din.NetworkService) (*provider, error) {
	httpClient := din_http.NewHTTPClient(time.Duration(DefaultHCTimeout) * time.Second)

	// Set the provider auth config based on the auth type
	if regAuthConfig != nil {
		var err error
		provider.Auth, err = d.createProviderSIWEAuth(regAuthConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider SIWE auth: %w", err)
		}
	}

	err := d.initializeProvider(provider, httpClient, d.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize provider: %w", err)
	}
	provider.Priority = d.RegistryPriority

	// Get the network service methods from the din registry
	provider.Methods = make(map[string]struct{})
	for _, method := range regNetworkService.Methods {
		provider.Methods[method.Name] = struct{}{}
	}

	return provider, nil
}

func (d *DinMiddleware) createProviderSIWEAuth(authConfig *din.ProviderAuthConfig) (*siwe.SIWEClientAuth, error) {
	switch authConfig.Type {
	case din.ProviderAuthTypeSIWE:
		// Create a new SIWE auth object
		auth := d.SiweSignerClient.CreateNewSIWEAuth(authConfig.Url, 16)
		// Set the signer for the provider to the default signer. The default signer is set on proxy startup.
		// it requires a secret key defined in the caddyfile.
		if auth.Signer == nil {
			if d.DefaultSiweSigner == nil {
				return nil, fmt.Errorf("siwe default signer is not configured")
			}
			auth.Signer = d.DefaultSiweSigner
		}
		return auth, nil
	default:
		return nil, nil
	}
}

// updateNetwork updates the network object in the middleware object with the provided registry network data
func (d *DinMiddleware) updateNetworkData(network *network) {
	// update the network object with the registry network config data
	d.Networks[network.Name].HCMethod = network.HCMethod
	d.Networks[network.Name].HCInterval = network.HCInterval
	d.Networks[network.Name].BlockLagLimit = network.BlockLagLimit
	d.Networks[network.Name].BlockJumpLimit = network.BlockJumpLimit
	d.Networks[network.Name].MaxRequestPayloadSizeKB = network.MaxRequestPayloadSizeKB
	d.Networks[network.Name].RequestAttemptCount = network.RequestAttemptCount
	// Don't override ExpectedChainID as it's a required config value

	// add the new providers to the middleware network.Providers map
	for _, p := range network.Providers {
		d.Networks[network.Name].Providers[p.host] = p
	}
}

// ensureUniqueProviderHost ensures the provider has a unique host in the network's providers map
// by appending a counter if necessary. Returns the unique host value.
func (d *DinMiddleware) ensureUniqueProviderHost(networkName string, host string) string {
	// Get existing hosts with the same base name
	baseHosts := []string{}

	for existingHost := range d.Networks[networkName].Providers {
		// We need to match exact host or host-N pattern
		if existingHost == host || strings.HasPrefix(existingHost, host+"-") {
			baseHosts = append(baseHosts, existingHost)
		}
	}

	// If no hosts with this base exist yet, use base host without suffix
	if len(baseHosts) == 0 {
		return host
	}

	// For subsequent hosts, use host-1, host-2, etc.
	return fmt.Sprintf("%s-%d", host, len(baseHosts))
}

// processHCMethodResponseAsync asynchronously processes responses for health check method requests.
// It extracts the request method from the replacer, verifies if it matches the network's health check
// method (HCMethod), and if so, processes the block number from the response body.
// This function is designed to run in a separate goroutine to avoid blocking the main request handling flow.
// When a valid block number is extracted, it also retrieves the corresponding block hash and adds both
// to the network's block history. This information is crucial for tracking the network's current state
// and ensuring proper synchronization across providers.
func (d *DinMiddleware) processHCMethodResponseAsync(networkObj *network, networkPath string, respBody []byte, respStatus int, method string) {
	if len(respBody) == 0 || networkObj == nil || networkObj.HCMethod == "" {
		return
	}

	// If method is "other_method" and networkObj.HCMethod is "eth_blockNumber", this should be true.
	if method != networkObj.HCMethod {
		return
	}

	// This log should only appear if Condition 2 is false.
	d.logger.Debug("Goroutine: Processing response for HCMethod", zap.String("method", method), zap.String("network", networkPath))

	// Create synthetic request context for consistent logging
	// Since this is async processing of a response, we use "din" as provider identifier
	providerHost := "din"
	repl, jsonRPCReq, payload := createHealthCheckRequestContext(networkPath, providerHost, method)

	// Ensure the replacer has the correct payload for consistent logging
	repl.Set(RequestBodyKey, payload)

	// Pass the address of respStatus to processBlockNumberResponse
	// processBlockNumberResponse checks for respStatus >= 400
	blockNumber, _, processingError := networkObj.processBlockNumberResponse(respBody, &respStatus)
	if processingError != nil {
		// Log the failed async processing with detailed information
		logFailedAttempt(&LogFailedAttemptParams{
			Reason:              "Async health check processing failed",
			Logger:              d.logger,
			NetworkPath:         networkPath,
			FailedAttemptNumber: 1, // Single attempt for async processing
			MaxAttempts:         1, // Total attempts is always 1 for async processing
			StatusCodeOfFailure: respStatus,
			Error:               processingError,
			Replacer:            repl,
			ParsedReqBody:       jsonRPCReq,
			RawResponseBody:     respBody,
		})
		return
	}

	block, err := networkObj.getBlockByNumber(blockNumber)
	if err != nil {
		// Create a new context specifically for the getBlockByNumber call that failed
		// This ensures the logging shows the correct method and parameters for the failed call
		getBlockMethod := networkObj.GetBlockByNumberMethod
		if getBlockMethod == "" {
			getBlockMethod = DefaultGetBlockByNumberMethod // Default fallback
		}

		// Create context for the getBlockByNumber call
		getBlockRepl, getBlockJSONRPCReq, _ := createGetBlockByNumberRequestContext(networkPath, providerHost, getBlockMethod, blockNumber, networkObj)

		// For getBlockByNumber errors, we don't have a JSON-RPC error from the original response
		// since this is a separate internal call
		logFailedAttempt(&LogFailedAttemptParams{
			Reason:              "Get block by number failed",
			Logger:              d.logger,
			NetworkPath:         networkPath,
			FailedAttemptNumber: 1,   // Single attempt for async processing
			MaxAttempts:         1,   // Total attempts is always 1 for async processing
			StatusCodeOfFailure: 200, // getBlockByNumber is an internal call, assume 200 for the original response
			Error:               err,
			Replacer:            getBlockRepl,
			ParsedReqBody:       getBlockJSONRPCReq,
			RawResponseBody:     nil, // No response body for internal getBlockByNumber calls
		})
		return
	}

	// save the block number to the network object's history
	networkObj.AddNetworkBlockEntry(blockNumber, block) // Add the block number and block hash to the network object's history as long as its the the latest block number
	d.logger.Debug("Goroutine: HCMethod matched, successfully processed block number and added to network history", zap.Int64("block_number", blockNumber), zap.String("network", networkPath))
}
