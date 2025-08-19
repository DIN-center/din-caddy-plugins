package modules

import (
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"

	"encoding/json"

	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
	dinreg "github.com/DIN-center/din-sc/apps/din-go/pkg/dinregistry"
	"go.uber.org/zap"
)

// Helper functions for backward compatibility with different NetworkConfig struct versions

// getNetworkConfigUint8Field safely gets a uint8 field from NetworkConfig using reflection
func getNetworkConfigUint8Field(config *dinreg.NetworkConfig, fieldName string) uint8 {
	if config == nil {
		return 0
	}
	v := reflect.ValueOf(config).Elem()
	field := v.FieldByName(fieldName)
	if !field.IsValid() || field.Kind() != reflect.Uint8 {
		return 0
	}
	return uint8(field.Uint())
}

// getNetworkConfigStringField safely gets a string field from NetworkConfig using reflection
func getNetworkConfigStringField(config *dinreg.NetworkConfig, fieldName string) string {
	if config == nil {
		return ""
	}
	v := reflect.ValueOf(config).Elem()
	field := v.FieldByName(fieldName)
	if !field.IsValid() || field.Kind() != reflect.String {
		return ""
	}
	return field.String()
}

// getNetworkConfigBoolField safely gets a bool field from NetworkConfig using reflection
func getNetworkConfigBoolField(config *dinreg.NetworkConfig, fieldName string) bool {
	if config == nil {
		return false
	}
	v := reflect.ValueOf(config).Elem()
	field := v.FieldByName(fieldName)
	if !field.IsValid() || field.Kind() != reflect.Bool {
		return false
	}
	return field.Bool()
}

// syncRegistryWithLatestBlock checks the latest block number from the linea network and updates the middleware object with the latest registry data if the block number difference is greater than or equal to the epoch
func (d *DinMiddleware) syncRegistryWithLatestBlock() {
	// Get the latest block number from the linea network
	latestBlockNumber, err := d.DingoClient.GetLatestBlockNumber()
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
			if regNetwork.Status != dinreg.Active {
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
			if regNetwork.Status != dinreg.Active {
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
	// Step 2: Create a new network without type - will be set via Caddyfile configuration
	// Registry networks must have explicit 'type' configuration in Caddyfile like all other networks
	network, err := NewNetwork(regNetwork.ProxyName, "", d.Env, d.CaddyPort)
	if err != nil {
		return fmt.Errorf("failed to create network '%s': %w", regNetwork.ProxyName, err)
	}
	network, err = d.syncNetworkConfig(regNetwork, network)
	if err != nil {
		d.logger.Error("Failed to sync network config", zap.Error(err))
		return err
	}

	httpClient := din_http.NewHTTPClient(time.Duration(network.HCTimeout) * time.Second)
	network.HttpClient = httpClient
	network.logger = d.logger
	network.PrometheusClient = d.PrometheusClient
	network.machineID = d.machineID

	for _, regProvider := range regNetwork.Providers {
		for _, networkService := range regProvider.NetworkServices {
			if networkService.Status != dinreg.Active {
				d.logger.Debug("Network service is not active", zap.String("network_service", networkService.Url))
				continue
			}
			// Create a new provider object
			provider, err := NewProvider(networkService.Url)
			if err != nil {
				d.logger.Error("Failed to create new provider object", zap.Error(err))
				continue
			}

			provider, err = d.createNewProvider(regNetwork.ProxyName, provider, regProvider.AuthConfig, networkService.Address)
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
	newNetwork, err := d.syncNetworkConfig(regNetwork, newNetwork)
	if err != nil {
		d.logger.Error("Failed to sync network config", zap.Error(err))
		return err
	}

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
				if networkService.Status != dinreg.Active {
					d.logger.Debug("Network service is not active", zap.String("network_service", networkService.Url))
					continue
				}
				// create a new provider object and add it to the copied network object
				newProvider, err := d.createNewProvider(regNetwork.ProxyName, newProvider, regProvider.AuthConfig, networkService.Address)
				if err != nil {
					d.logger.Error("Failed to create new provider", zap.Error(err))
					continue
				}

				// add the new provider to the copied network object
				newNetwork.Providers[newProvider.host] = newProvider
			} else {
				// if the provider exists in the copied network object,
				// check if the network service is active, if not, don't update the provider data and remove the provider from the copied network object
				if networkService.Status != dinreg.Active {
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
func (d *DinMiddleware) syncNetworkConfig(regNetwork *din.Network, network *network) (*network, error) {
	// Extract all required method names in a single batch to minimize registry calls
	methodNames, err := d.extractNetworkMethods(regNetwork)
	if err != nil {
		return nil, err
	}

	// Extract all config fields once to minimize reflection calls
	config := extractNetworkConfigFields(regNetwork.NetworkConfig)

	// Update network configuration using extracted values
	d.updateNetworkFields(network, config, methodNames)

	return network, nil
}

// NetworkMethodNames holds extracted method names
type NetworkMethodNames struct {
	HealthCheck  string
	ChainID      string
	CallContract string
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

// extractNetworkMethods gets all required method names in a single batch
func (d *DinMiddleware) extractNetworkMethods(regNetwork *din.Network) (*NetworkMethodNames, error) {
	methods := &NetworkMethodNames{}

	// Get healthcheck method (required)
	if hcMethod, err := d.DingoClient.GetNetworkMethodNameByBit(regNetwork.Name, regNetwork.NetworkConfig.HealthcheckMethodBit); err != nil {
		d.logger.Error("Failed to get network healthcheck method name", zap.String("network", regNetwork.Name), zap.Error(err))
		return nil, err
	} else {
		methods.HealthCheck = hcMethod
	}

	// Get chain ID method (optional)
	if chainIdBit := getNetworkConfigUint8Field(regNetwork.NetworkConfig, "ChainIdMethodBit"); chainIdBit > 0 {
		if chainIdMethod, err := d.DingoClient.GetNetworkMethodNameByBit(regNetwork.Name, chainIdBit); err != nil {
			d.logger.Error("Failed to get network chain ID method name", zap.String("network", regNetwork.Name), zap.Error(err))
			return nil, fmt.Errorf("failed to get network chain ID method: %w", err)
		} else {
			methods.ChainID = chainIdMethod
		}
	}

	// Get call contract method (optional)
	if callContractBit := getNetworkConfigUint8Field(regNetwork.NetworkConfig, "CallContractMethodBit"); callContractBit > 0 {
		if callMethod, err := d.DingoClient.GetNetworkMethodNameByBit(regNetwork.Name, callContractBit); err != nil {
			d.logger.Error("Failed to get network call contract method name", zap.String("network", regNetwork.Name), zap.Error(err))
			return nil, fmt.Errorf("failed to get network call contract method: %w", err)
		} else {
			methods.CallContract = callMethod
		}
	}

	return methods, nil
}

// extractNetworkConfigFields extracts all config fields in a single pass
func extractNetworkConfigFields(config *dinreg.NetworkConfig) *NetworkConfigFields {
	return &NetworkConfigFields{
		ChainID:             getNetworkConfigStringField(config, "ChainId"),
		HealthCheckInterval: int(config.HealthcheckIntervalSec),
		BlockLagLimit:       int64(config.BlockLagLimit),
		BlockJumpLimit:      int64(getNetworkConfigUint8Field(config, "BlockJumpLimit")),
		MaxPayloadSizeKB:    int64(config.MaxRequestPayloadSizeKb),
		RequestAttemptCount: int(config.RequestAttemptCount),
		ArchiveEnabled:      getNetworkConfigBoolField(config, "ArchiveEnabled"),
	}
}

// updateNetworkFields applies configuration updates using a streamlined approach
func (d *DinMiddleware) updateNetworkFields(network *network, config *NetworkConfigFields, methods *NetworkMethodNames) {
	// Log available registry methods (handlers provide the actual implementations)
	d.logRegistryMethods(network.Name, methods)

	// Update fields using helper function to reduce duplication
	d.updateField("chain Id", network.Name, &network.ChainId, config.ChainID, config.ChainID != "" && config.ChainID != network.ChainId)
	d.updateField("healthcheck interval", network.Name, &network.HCInterval, config.HealthCheckInterval, config.HealthCheckInterval != 0 && config.HealthCheckInterval != network.HCInterval)
	d.updateField("block lag limit", network.Name, &network.BlockLagLimit, config.BlockLagLimit, config.BlockLagLimit != 0 && config.BlockLagLimit != network.BlockLagLimit)
	d.updateField("block jump limit", network.Name, &network.BlockJumpLimit, config.BlockJumpLimit, config.BlockJumpLimit != 0 && config.BlockJumpLimit != network.BlockJumpLimit)
	d.updateField("max request payload size", network.Name, &network.MaxRequestPayloadSizeKB, config.MaxPayloadSizeKB, config.MaxPayloadSizeKB != 0 && config.MaxPayloadSizeKB != network.MaxRequestPayloadSizeKB)
	d.updateField("request attempt count", network.Name, &network.RequestAttemptCount, config.RequestAttemptCount, config.RequestAttemptCount != 0 && config.RequestAttemptCount != network.RequestAttemptCount)
	d.updateField("archive enabled", network.Name, &network.ArchiveEnabled, config.ArchiveEnabled, config.ArchiveEnabled != network.ArchiveEnabled)
}

// logRegistryMethods logs available registry methods in a batch
func (d *DinMiddleware) logRegistryMethods(networkName string, methods *NetworkMethodNames) {
	if methods.HealthCheck != "" {
		d.logger.Debug("Registry healthcheck method available (provided by handler)",
			zap.String("network", networkName),
			zap.String("healthcheck_method", methods.HealthCheck))
	}
	if methods.ChainID != "" {
		d.logger.Debug("Registry chain ID method available (provided by handler)",
			zap.String("network", networkName),
			zap.String("chain_id_method", methods.ChainID))
	}
	if methods.CallContract != "" {
		d.logger.Debug("Registry call contract method available (provided by handler)",
			zap.String("network", networkName),
			zap.String("call_contract_method", methods.CallContract))
	}
}

// updateField is a generic helper that updates a field and logs the change
func (d *DinMiddleware) updateField(fieldName, networkName string, target interface{}, newValue interface{}, shouldUpdate bool) {
	if !shouldUpdate {
		return
	}

	// Use reflection to set the value generically
	targetVal := reflect.ValueOf(target).Elem()
	newVal := reflect.ValueOf(newValue)

	if targetVal.CanSet() && targetVal.Type() == newVal.Type() {
		targetVal.Set(newVal)
		d.logger.Debug(fmt.Sprintf("Setting network %s", fieldName),
			zap.String("network", networkName),
			zap.Any(strings.ReplaceAll(fieldName, " ", "_"), newValue))
	}
}

// createNewProvider creates a new provider object and initializes the provider with the network service address
func (d *DinMiddleware) createNewProvider(networkName string, provider *provider, authConfig *dinreg.NetworkServiceAuthConfig, networkServiceAddress string) (*provider, error) {
	httpClient := din_http.NewHTTPClient(time.Duration(DefaultHCTimeout) * time.Second)

	// Set the provider auth config based on the auth type
	if authConfig != nil {
		var err error
		provider.Auth, err = d.createProviderSIWEAuth(authConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider SIWE auth: %w", err)
		}
	}

	err := d.initializeProvider(networkName, provider, httpClient, d.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize provider: %w", err)
	}
	provider.Priority = d.RegistryPriority
	// Get the network service methods from the din registry
	networkServiceMethods, err := d.DingoClient.GetNetworkServiceMethods(networkServiceAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get network service methods: %w", err)
	}

	// TODO: Figure out how to customize this per provider via registry
	provider.Methods = make(map[string]struct{})
	for _, method := range networkServiceMethods {
		provider.Methods[*method] = struct{}{}
	}

	return provider, nil
}

func (d *DinMiddleware) createProviderSIWEAuth(authConfig *dinreg.NetworkServiceAuthConfig) (*siwe.SIWEClientAuth, error) {
	switch authConfig.Type {
	case dinreg.SIWE:
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
	case dinreg.None:
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

// ensureUniqueProviderHost ensures the provider has a unique host identifier
// by appending API key suffix when multiple providers share the same base host.
// It first tries to extract a suffix from the URL path (e.g., validation cloud API keys),
// then from X-API-Key header (e.g., nodefleet), and falls back to a counter if neither is available.
func (d *DinMiddleware) ensureUniqueProviderHost(networkName string, parsedUrl *url.URL, headers map[string]string) string {
	if parsedUrl == nil {
		return ""
	}
	
	baseHost := parsedUrl.Host
	if baseHost == "" {
		return ""
	}
	
	// Check if any existing provider has the same base host
	existingProviders := []string{}
	for existingHost := range d.Networks[networkName].Providers {
		// Check for exact match or host with suffix pattern
		if existingHost == baseHost || strings.HasPrefix(existingHost, baseHost+"-") {
			existingProviders = append(existingProviders, existingHost)
		}
	}
	
	// If this is the first provider with this host, use as-is
	if len(existingProviders) == 0 {
		return baseHost
	}
	
	// Multiple providers with same base host - need differentiation
	var suffix string
	
	// Try to extract suffix from URL path (e.g., validation cloud API keys)
	if parsedUrl.Path != "" {
		// Remove leading slash and any path segments
		path := strings.TrimPrefix(parsedUrl.Path, "/")
		// Get the last segment which typically contains the API key
		segments := strings.Split(path, "/")
		if len(segments) > 0 {
			lastSegment := segments[len(segments)-1]
			// Use last 4 chars if segment is long enough (likely an API key)
			if len(lastSegment) >= 4 {
				suffix = lastSegment[len(lastSegment)-4:]
			}
		}
	}
	
	// If no path suffix, try headers (case-insensitive check for X-API-Key)
	if suffix == "" && headers != nil {
		for key, value := range headers {
			if strings.EqualFold(key, "X-API-Key") {
				// Remove any whitespace and use last 4 chars
				value = strings.TrimSpace(value)
				if len(value) >= 4 {
					suffix = value[len(value)-4:]
					break
				}
			}
		}
	}
	
	// Build the unique host identifier
	if suffix != "" {
		proposedHost := fmt.Sprintf("%s-%s", baseHost, suffix)
		// Check if this host already exists (collision handling)
		for _, existing := range existingProviders {
			if existing == proposedHost {
				// Collision detected, append counter
				counter := 1
				for {
					alternativeHost := fmt.Sprintf("%s-%s-%d", baseHost, suffix, counter)
					if !d.providerHostExists(networkName, alternativeHost) {
						return alternativeHost
					}
					counter++
				}
			}
		}
		return proposedHost
	}
	
	// Fallback: use incrementing counter
	counter := len(existingProviders)
	return fmt.Sprintf("%s-%d", baseHost, counter)
}

// providerHostExists checks if a provider host already exists in the network
func (d *DinMiddleware) providerHostExists(networkName string, host string) bool {
	if network, exists := d.Networks[networkName]; exists {
		_, exists := network.Providers[host]
		return exists
	}
	return false
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
