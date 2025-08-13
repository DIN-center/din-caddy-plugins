package modules

import (
	"context"
	"fmt"
	"strings"
	"time"

	"encoding/json"

	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
	"github.com/DIN-center/din-caddy-plugins/lib/web3"
	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
	"go.uber.org/zap"
)

// syncRegistryWithLatestBlock checks the latest block number from the linea network and updates the middleware object with the latest registry data if the block number difference is greater than or equal to the epoch
func (d *DinMiddleware) syncRegistryWithLatestBlock(web3Client web3.Web3Client) {
	// Create retry config for registry operations
	retryConfig := &utils.RetryConfig{
		MaxRetries:        d.RegistryRetryMaxAttempts,
		InitialDelay:      d.RegistryRetryInitialDelay,
		MaxDelay:          d.RegistryRetryMaxDelay,
		BackoffMultiplier: d.RegistryRetryBackoffFactor,
		JitterFactor:      0.1, // 10% jitter
	}
	
	ctx := context.Background()
	
	// Get the latest block number from the linea network with retry
	latestBlockNumber, err := utils.RetryWithBackoffTyped(ctx, retryConfig, func() (uint64, error) {
		return web3Client.LatestBlockNumber()
	})
	if err != nil {
		d.logger.Error("Failed to get latest block number after retries", 
			zap.Error(err),
			zap.Int("max_retries", d.RegistryRetryMaxAttempts))
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
		// Get registry data with retry
		registryData, err := utils.RetryWithBackoffTyped(ctx, retryConfig, func() (*din.DinRegistryData, error) {
			return d.DingoClient.GetRegistryData()
		})
		if err != nil {
			d.logger.Error("Failed to get data from registry after retries", 
				zap.Error(err),
				zap.Int("max_retries", d.RegistryRetryMaxAttempts))
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
	// IMPORTANT: Using write lock (Lock/Unlock) because we perform write operations on d.Networks map
	d.mu.Lock()
	defer d.mu.Unlock()

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
			err := d.addNetworkWithRegistryData(regNetwork)
			if err != nil {
				// If there is an error adding the network, log the error and continue to the next registry network
				d.logger.Error("Failed to add network from registry", zap.Error(err))
				continue
			}
		} else {
			// If the network exists in the middleware object, check to see if the registry version is active or not,
			if regNetwork.Status != din.NetworkStatusActive {
				// Delete the network for now if it is not active
				d.logger.Debug("Network is not active, removing from middleware: ", zap.String("network", regNetwork.ProxyName))
				delete(d.Networks, regNetwork.ProxyName)
				continue
			}
			// if active, update the existing network in place with the registry data
			err := d.updateNetworkWithRegistryData(regNetwork, network)
			if err != nil {
				d.logger.Error("Failed to update network with registry data", zap.Error(err))
				continue
			}
		}
	}
}

// addNetworkWithRegistryData creates a new network object from the registry network data and adds it to the middleware object
func (d *DinMiddleware) addNetworkWithRegistryData(regNetwork *din.Network) error {
	// Get handler type from registry config, default to empty if not specified
	handlerType := ""
	if regNetwork.NetworkConfig != nil && regNetwork.NetworkConfig.Handler != "" {
		handlerType = regNetwork.NetworkConfig.Handler
	}

	// Create network with handler type from registry
	network, err := NewNetwork(regNetwork.ProxyName, HandlerType(handlerType), d.Env, d.CaddyPort)
	if err != nil {
		return fmt.Errorf("failed to create network '%s': %w", regNetwork.ProxyName, err)
	}

	// Mark handler as not set from Caddyfile (it came from registry)
	if network.ConfigSource != nil {
		network.ConfigSource.HandlerTypeSet = false
	}
	network = d.syncNetworkConfig(regNetwork, network)

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

			provider, err = d.createNewProvider(provider, network, regProvider.AuthConfig, networkService)
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
		return nil
	}
	// Add the network to the middleware object
	d.Networks[network.Name] = network

	// Register network in global registry for DinUpstreams access
	RegisterNetwork(network.Name, network)

	// Start the healthcheck for the network if the middleware is not in test mode
	if !d.testMode {
		network.startHealthcheck()
		d.logger.Info("Starting healthcheck for registry network", zap.String("network", network.Name))
	}
	return nil
}

// updateNetworkWithRegistryData updates the network object in the middleware object with the latest registry network data
func (d *DinMiddleware) updateNetworkWithRegistryData(regNetwork *din.Network, newNetwork *network) error {
	// Sync the network config data from the registry network to the copied network object
	newNetwork = d.syncNetworkConfig(regNetwork, newNetwork)

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
				newProvider, err := d.createNewProvider(newProvider, newNetwork, regProvider.AuthConfig, networkService)
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
	return nil
}

// syncNetworkConfig updates the network object with the registry network config data
func (d *DinMiddleware) syncNetworkConfig(regNetwork *din.Network, network *network) *network {
	// Update network configuration using extracted values
	d.updateNetworkFields(network, regNetwork.NetworkConfig)

	return network
}

// NetworkConfigFields holds extracted configuration values
type NetworkConfigFields struct {
	ChainID             string
	HealthCheckInterval int
	BlockLagLimit       int64
	BlockJumpLimit      int64
	MaxPayloadSizeKB    int64
	RequestAttemptCount int
	ArchiveEnabled      bool
}

// updateNetworkFields applies configuration updates using a streamlined approach
// Respects Caddyfile configuration priority - only updates fields not explicitly set via Caddyfile
func (d *DinMiddleware) updateNetworkFields(network *network, regNetworkConfig *din.NetworkOperationsConfig) {
	// Ensure ConfigSource exists
	if network.ConfigSource == nil {
		network.ConfigSource = &networkConfigSource{}
	}

	// Handler type from registry - only if not set via Caddyfile
	if regNetworkConfig.Handler != "" && !network.ConfigSource.HandlerTypeSet {
		// Only set handler type if network doesn't have one
		if network.HandlerType == "" {
			network.HandlerType = HandlerType(regNetworkConfig.Handler)
			d.logger.Debug("Setting network handler from registry",
				zap.String("network", network.Name),
				zap.String("handler", regNetworkConfig.Handler))
		}
	}

	// Chain ID - respect Caddyfile priority
	if regNetworkConfig.ChainId != "" && !network.ConfigSource.ChainIdSet {
		if regNetworkConfig.ChainId != network.ChainId {
			network.ChainId = regNetworkConfig.ChainId
			d.logger.Debug("Setting network chain ID from registry",
				zap.String("network", network.Name),
				zap.String("chain_id", network.ChainId))
		}
	}

	// Health check threshold (NEW)
	if regNetworkConfig.HealthcheckThreshold != 0 && !network.ConfigSource.HCThresholdSet {
		network.HCThreshold = int(regNetworkConfig.HealthcheckThreshold)
		d.logger.Debug("Setting network healthcheck threshold from registry",
			zap.String("network", network.Name),
			zap.Int("threshold", network.HCThreshold))
	}

	// Health check timeout (NEW)
	if regNetworkConfig.HealthcheckTimeout != 0 && !network.ConfigSource.HCTimeoutSet {
		network.HCTimeout = int(regNetworkConfig.HealthcheckTimeout)
		d.logger.Debug("Setting network healthcheck timeout from registry",
			zap.String("network", network.Name),
			zap.Int("timeout", network.HCTimeout))
	}

	// Health check interval
	if regNetworkConfig.HealthcheckIntervalSec != 0 && !network.ConfigSource.HCIntervalSet {
		if int(regNetworkConfig.HealthcheckIntervalSec) != network.HCInterval {
			network.HCInterval = int(regNetworkConfig.HealthcheckIntervalSec)
			d.logger.Debug("Setting network healthcheck interval from registry",
				zap.String("network", network.Name),
				zap.Int("interval", network.HCInterval))
		}
	}

	// Block lag limit
	if regNetworkConfig.BlockLagLimit != 0 && !network.ConfigSource.BlockLagLimitSet {
		if int64(regNetworkConfig.BlockLagLimit) != network.BlockLagLimit {
			network.BlockLagLimit = int64(regNetworkConfig.BlockLagLimit)
			d.logger.Debug("Setting network block lag limit from registry",
				zap.String("network", network.Name),
				zap.Int64("block_lag_limit", network.BlockLagLimit))
		}
	}

	// Block jump limit
	if regNetworkConfig.BlockJumpLimit != 0 && !network.ConfigSource.BlockJumpLimitSet {
		if int64(regNetworkConfig.BlockJumpLimit) != network.BlockJumpLimit {
			network.BlockJumpLimit = int64(regNetworkConfig.BlockJumpLimit)
			d.logger.Debug("Setting network block jump limit from registry",
				zap.String("network", network.Name),
				zap.Int64("block_jump_limit", network.BlockJumpLimit))
		}
	}

	// Max request payload size
	if regNetworkConfig.MaxRequestPayloadSizeKb != 0 && !network.ConfigSource.MaxRequestPayloadSizeKBSet {
		if int64(regNetworkConfig.MaxRequestPayloadSizeKb) != network.MaxRequestPayloadSizeKB {
			network.MaxRequestPayloadSizeKB = int64(regNetworkConfig.MaxRequestPayloadSizeKb)
			d.logger.Debug("Setting network max request payload size from registry",
				zap.String("network", network.Name),
				zap.Int64("max_request_payload_size_kb", network.MaxRequestPayloadSizeKB))
		}
	}

	// Request attempt count
	if regNetworkConfig.RequestAttemptCount != 0 && !network.ConfigSource.RequestAttemptCountSet {
		if int(regNetworkConfig.RequestAttemptCount) != network.RequestAttemptCount {
			network.RequestAttemptCount = int(regNetworkConfig.RequestAttemptCount)
			d.logger.Debug("Setting network request attempt count from registry",
				zap.String("network", network.Name),
				zap.Int("request_attempt_count", network.RequestAttemptCount))
		}
	}

	// Provider block history size (NEW)
	if regNetworkConfig.ProviderBlockHistorySize != 0 && !network.ConfigSource.ProviderBlockHistorySizeSet {
		network.ProviderBlockHistorySize = int(regNetworkConfig.ProviderBlockHistorySize)
		d.logger.Debug("Setting provider block history size from registry",
			zap.String("network", network.Name),
			zap.Int("size", network.ProviderBlockHistorySize))
	}

	// Network block history size (NEW)
	if regNetworkConfig.NetworkBlockHistorySize != 0 && !network.ConfigSource.NetworkBlockHistorySizeSet {
		network.NetworkBlockHistorySize = int(regNetworkConfig.NetworkBlockHistorySize)
		d.logger.Debug("Setting network block history size from registry",
			zap.String("network", network.Name),
			zap.Int("size", network.NetworkBlockHistorySize))
	}

	// Archive enabled - special case as it's a boolean
	if !network.ConfigSource.ArchiveEnabledSet {
		if regNetworkConfig.ArchiveEnabled != network.ArchiveEnabled {
			network.ArchiveEnabled = regNetworkConfig.ArchiveEnabled
			d.logger.Debug("Setting network archive enabled from registry",
				zap.String("network", network.Name),
				zap.Bool("archive_enabled", network.ArchiveEnabled))
		}
	}
}

// createNewProvider creates a new provider object and initializes the provider with the network service address
func (d *DinMiddleware) createNewProvider(provider *provider, network *network, authConfig *din.ProviderAuthConfig, regNetworkService *din.NetworkService) (*provider, error) {
	httpClient := din_http.NewHTTPClient(time.Duration(DefaultHCTimeout) * time.Second)

	// Set the provider auth config based on the auth type
	if authConfig != nil {
		var err error
		provider.Auth, err = d.createProviderSIWEAuth(authConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider SIWE auth: %w", err)
		}
	}

	err := d.initializeProvider(provider, network, httpClient, d.logger)
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
	case din.ProviderAuthTypeNone:
		return nil, nil
	default:
		return nil, nil
	}
}

// updateNetwork updates the network object in the middleware object with the provided registry network data
func (d *DinMiddleware) updateNetworkData(network *network) {
	// update the network object with the registry network config data
	// REMOVED: HCMethod is now provided by handlers
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
func (d *DinMiddleware) processHCMethodResponseAsync(networkObj *network, networkPath string, respBody []byte, respStatus int, requestMethod string) {
	if len(respBody) == 0 || networkObj == nil || networkObj.handler == nil {
		return
	}

	// Only process if the request method matches the network's health check method
	if requestMethod != networkObj.handler.GetHealthCheckMethod() {
		return
	}

	// This log should only appear for health check method requests
	d.logger.Debug("Goroutine: Processing response for HCMethod", zap.String("method", requestMethod), zap.String("network", networkPath))

	// Create synthetic request context for consistent logging
	// Since this is async processing of a response, we use "din" as provider identifier
	providerHost := "din"
	repl, genericContext, payload := createHealthCheckRequestContext(networkPath, providerHost, requestMethod, networkObj)

	// Check if handler was able to create context - if not, skip detailed logging
	if repl == nil || genericContext == nil || payload == nil {
		d.logger.Debug("Handler unavailable or failed to create health check context, skipping detailed async processing",
			zap.String("method", requestMethod),
			zap.String("network", networkPath))
		return
	}

	// Ensure the replacer has the correct payload for consistent logging
	repl.Set(RequestBodyKey, payload)

	// Pass the address of respStatus to processBlockNumberResponse
	// processBlockNumberResponse checks for respStatus >= 400
	blockNumber, _, processingError := networkObj.processBlockNumberResponse(respBody, &respStatus)
	if processingError != nil {
		// Extract method directly from GenericRequestContext - completely generic
		var method string = "unknown"
		if genericContext != nil {
			method = genericContext.Method
		}

		// Log the failed async processing with detailed information
		logFailedAttempt(LogFailedAttemptParams{
			Reason:              "Async health check processing failed",
			Logger:              d.logger,
			NetworkPath:         networkPath,
			FailedAttemptNumber: 1, // Single attempt for async processing
			MaxAttempts:         1, // Total attempts is always 1 for async processing
			StatusCodeOfFailure: respStatus,
			Error:               processingError,
			Replacer:            repl,
			RequestMethod:       method,
			RequestParams:       nil, // No params for health checks - completely generic
			RawResponseBody:     respBody,
		})
		return
	}

	block, err := networkObj.getBlockByNumber(blockNumber)
	if err != nil {
		// Create a new context specifically for the getBlockByNumber call that failed
		// This ensures the logging shows the correct method and parameters for the failed call
		getBlockMethod := networkObj.handler.GetBlockByNumberMethod()
		if getBlockMethod == "" {
			getBlockMethod = "getBlockByNumber" // Default fallback method name for logging
		}

		// Create context for the getBlockByNumber call
		getBlockRepl, getBlockGenericContext, _ := createGetBlockByNumberRequestContext(networkPath, providerHost, getBlockMethod, blockNumber, networkObj)

		// Check if handler was able to create context
		if getBlockRepl == nil || getBlockGenericContext == nil {
			d.logger.Debug("Handler unavailable or failed to create getBlockByNumber context, using minimal error logging",
				zap.String("method", getBlockMethod),
				zap.String("network", networkPath),
				zap.Int64("block_number", blockNumber),
				zap.Error(err))
			return
		}

		// Extract method and params directly from getBlockByNumber GenericRequestContext for logging
		var getBlockMethodName string = "unknown"
		var getBlockParams json.RawMessage
		if getBlockGenericContext != nil {
			getBlockMethodName = getBlockGenericContext.Method
			if getBlockGenericContext.Context != nil {
				if rpcParams, hasParams := getBlockGenericContext.Context["params"]; hasParams {
					if paramBytes, ok := rpcParams.(json.RawMessage); ok {
						getBlockParams = paramBytes
					}
				}
			}
		}

		// For getBlockByNumber errors, we don't have a JSON-RPC error from the original response
		// since this is a separate internal call
		logFailedAttempt(LogFailedAttemptParams{
			Reason:              "Get block by number failed",
			Logger:              d.logger,
			NetworkPath:         networkPath,
			FailedAttemptNumber: 1,   // Single attempt for async processing
			MaxAttempts:         1,   // Total attempts is always 1 for async processing
			StatusCodeOfFailure: 200, // getBlockByNumber is an internal call, assume 200 for the original response
			Error:               err,
			Replacer:            getBlockRepl,
			RequestMethod:       getBlockMethodName,
			RequestParams:       getBlockParams,
			RawResponseBody:     nil, // No response body for internal getBlockByNumber calls
		})
		return
	}

	// save the block number to the network object's history
	networkObj.AddNetworkBlockEntry(blockNumber, block) // Add the block number and block hash to the network object's history as long as its the the latest block number
	d.logger.Debug("Goroutine: HCMethod matched, successfully processed block number and added to network history", zap.Int64("block_number", blockNumber), zap.String("network", networkPath))
}
