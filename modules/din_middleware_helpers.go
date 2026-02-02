package modules

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"strings"
	"time"

	"encoding/hex"
	"encoding/json"

	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
	"github.com/DIN-center/din-sc/apps/din-go/lib/watcher"
	"go.uber.org/zap"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/web3"
)

// getRegistryData retrieves registry data with retry logic
func (d *DinMiddleware) getRegistryData() (*din.DinRegistryData, error) {
	var lastErr error

	for attempt := 0; attempt <= d.Registry.RetryMaxAttempts; attempt++ {
		data, err := d.DingoClient.GetRegistryData()
		if err == nil {
			return data, nil
		}
		lastErr = err

		if attempt < d.Registry.RetryMaxAttempts {
			d.logger.Warn("Registry call failed, retrying",
				zap.Int("attempt", attempt+1),
				zap.Int("max_attempts", d.Registry.RetryMaxAttempts),
				zap.Error(err))
			time.Sleep(d.Registry.RetryDelay) // Fixed delay, no backoff
		}
	}

	return nil, fmt.Errorf("registry call failed after %d attempts: %w",
		d.Registry.RetryMaxAttempts, lastErr)
}

// syncRegistryWithLatestBlock checks the latest block number from the linea network and updates the middleware object with the latest registry data if the block number difference is greater than or equal to the epoch
func (d *DinMiddleware) syncRegistryWithLatestBlock(web3Client web3.Web3Client) {
	// Get latest block number with retry
	var latestBlockNumber uint64
	var err error

	for attempt := 0; attempt <= d.Registry.RetryMaxAttempts; attempt++ {
		latestBlockNumber, err = web3Client.LatestBlockNumber()
		if err == nil {
			break
		}
		if attempt < d.Registry.RetryMaxAttempts {
			d.logger.Warn("Failed to get latest block number, retrying",
				zap.Int("attempt", attempt+1),
				zap.Error(err))
			time.Sleep(d.Registry.RetryDelay)
		}
	}

	if err != nil {
		d.logger.Error("Failed to get latest block number after retries",
			zap.Error(err),
			zap.Int("max_retries", d.Registry.RetryMaxAttempts))
		return
	}

	// Calculate the latest block floor by epoch. for example if the current block number is 55 and the epoch is 10, then the latest block floor by epoch is 50.
	latestBlockFloorByEpoch := latestBlockNumber - (latestBlockNumber % d.Registry.BlockEpoch)

	d.logger.Debug("Checking block number for registry sync", zap.Uint64("block_epoch", d.Registry.BlockEpoch),
		zap.Uint64("latest_linea_block_number", latestBlockNumber), zap.Uint64("latest_block_floor_by_epoch", latestBlockFloorByEpoch),
		zap.Uint64("last_updated_block_number", d.Registry.lastUpdatedEpochBlockNumber), zap.Uint64("difference", latestBlockFloorByEpoch-d.Registry.lastUpdatedEpochBlockNumber),
	)

	// If the difference between the latest block floor by epoch and the last updated block number is greater than or equal to the epoch, then update the networks and providers.
	if latestBlockFloorByEpoch-d.Registry.lastUpdatedEpochBlockNumber >= d.Registry.BlockEpoch {
		// Get registry data with retry
		registryData, err := d.getRegistryData()
		if err != nil {
			d.logger.Error("Failed to get data from registry after retries",
				zap.Error(err),
				zap.Int("max_retries", d.Registry.RetryMaxAttempts))
			return
		}
		d.processRegistryData(registryData)

		// Update the last updated block number
		d.Registry.lastUpdatedEpochBlockNumber = latestBlockFloorByEpoch
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

				// Remove the network from the watcher score computation if the dynamic load balancing is enabled
				if d.DynamicLoadBalancing.Enabled {
					d.DynamicLoadBalancing.watcherScoreManager.RemoveNetwork(regNetwork.ProxyName)
					d.logger.Info("[DYNAMIC_LB] Removing network from watcher score manager", zap.String("network", regNetwork.ProxyName), zap.String("machine_id", d.machineID))
				}

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

			provider, err = d.createNewProvider(regNetwork.ProxyName, provider, regProvider.AuthConfig, networkService)
			if err != nil {
				d.logger.Error("Failed to create new provider", zap.Error(err))
				continue
			}

			// Add the provider to the network object
			network.Providers[provider.host] = provider

			// Debug logging
			if d.logger != nil {
				d.logger.Debug("Registry: Added provider to network map",
					zap.String("network", network.Name),
					zap.String("providerHost", provider.host),
					zap.String("providerName", provider.Name),
					zap.String("providerUrl", provider.HttpUrl))
			}
		}
	}
	if len(network.Providers) == 0 {
		d.logger.Debug("Network has no active providers", zap.String("network", network.Name))
		return nil
	}
	// Add the network to the middleware object
	d.Networks[network.Name] = network

	// We should initialize the network and register it in global registry for DinUpstreams access
	d.logger.Info("Initializing network and registering in global registry", zap.String("network", network.Name))
	d.initializeNetwork(network.Name)

	// Add the network to the watcher score manager if the dynamic load balancing is enabled
	if d.DynamicLoadBalancing.Enabled {
		d.DynamicLoadBalancing.watcherScoreManager.AddNetworkWithBuiltInFormula(network.Name, d.GetOrCreateWatcherClient())
		d.logger.Info("[DYNAMIC_LB] Adding network to watcher score manager", zap.String("network", network.Name), zap.String("machine_id", d.machineID))
	}

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
				newProvider, err := d.createNewProvider(regNetwork.ProxyName, newProvider, regProvider.AuthConfig, networkService)
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
				// Note: Auth URL changes are not dynamically updated; provider would need to be recreated
				// for auth config changes to take effect
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
	// Ensure CaddyfileFlags exists
	if network.CaddyfileFlags == nil {
		network.CaddyfileFlags = &caddyfileConfigFlags{}
	}

	// Handler type from registry - only if not set via Caddyfile
	if regNetworkConfig.Handler != "" && !network.CaddyfileFlags.HandlerTypeSetInCaddyfile {
		// Only set handler type if network doesn't have one
		if network.HandlerType == "" {
			network.HandlerType = HandlerType(regNetworkConfig.Handler)
			d.logger.Debug("Setting network handler from registry",
				zap.String("network", network.Name),
				zap.String("handler", regNetworkConfig.Handler))
		}
	}

	// Chain ID - respect Caddyfile priority
	if regNetworkConfig.ChainId != "" && !network.CaddyfileFlags.ChainIdSetInCaddyfile {
		if regNetworkConfig.ChainId != network.ChainId {
			network.ChainId = regNetworkConfig.ChainId
			d.logger.Debug("Setting network chain ID from registry",
				zap.String("network", network.Name),
				zap.String("chain_id", network.ChainId))
		}
	}

	// Health check threshold
	if regNetworkConfig.HealthcheckThreshold != 0 && !network.CaddyfileFlags.HCThresholdSetInCaddyfile {
		network.HCThreshold = int(regNetworkConfig.HealthcheckThreshold)
		d.logger.Debug("Setting network healthcheck threshold from registry",
			zap.String("network", network.Name),
			zap.Int("threshold", network.HCThreshold))
	}

	// Health check timeout
	if regNetworkConfig.HealthcheckTimeout != 0 && !network.CaddyfileFlags.HCTimeoutSetInCaddyfile {
		network.HCTimeout = int(regNetworkConfig.HealthcheckTimeout)
		d.logger.Debug("Setting network healthcheck timeout from registry",
			zap.String("network", network.Name),
			zap.Int("timeout", network.HCTimeout))
	}

	// Health check interval
	if regNetworkConfig.HealthcheckIntervalSec != 0 && !network.CaddyfileFlags.HCIntervalSetInCaddyfile {
		if int(regNetworkConfig.HealthcheckIntervalSec) != network.HCInterval {
			network.HCInterval = int(regNetworkConfig.HealthcheckIntervalSec)
			d.logger.Debug("Setting network healthcheck interval from registry",
				zap.String("network", network.Name),
				zap.Int("interval", network.HCInterval))
		}
	}

	// Block lag limit
	if regNetworkConfig.BlockLagLimit != 0 && !network.CaddyfileFlags.BlockLagLimitSetInCaddyfile {
		if int64(regNetworkConfig.BlockLagLimit) != network.BlockLagLimit {
			network.BlockLagLimit = int64(regNetworkConfig.BlockLagLimit)
			d.logger.Debug("Setting network block lag limit from registry",
				zap.String("network", network.Name),
				zap.Int64("block_lag_limit", network.BlockLagLimit))
		}
	}

	// Block jump limit
	if regNetworkConfig.BlockJumpLimit != 0 && !network.CaddyfileFlags.BlockJumpLimitSetInCaddyfile {
		if int64(regNetworkConfig.BlockJumpLimit) != network.BlockJumpLimit {
			network.BlockJumpLimit = int64(regNetworkConfig.BlockJumpLimit)
			d.logger.Debug("Setting network block jump limit from registry",
				zap.String("network", network.Name),
				zap.Int64("block_jump_limit", network.BlockJumpLimit))
		}
	}

	// Max request payload size
	if regNetworkConfig.MaxRequestPayloadSizeKb != 0 && !network.CaddyfileFlags.MaxRequestPayloadSizeKBSetInCaddyfile {
		if int64(regNetworkConfig.MaxRequestPayloadSizeKb) != network.MaxRequestPayloadSizeKB {
			network.MaxRequestPayloadSizeKB = int64(regNetworkConfig.MaxRequestPayloadSizeKb)
			d.logger.Debug("Setting network max request payload size from registry",
				zap.String("network", network.Name),
				zap.Int64("max_request_payload_size_kb", network.MaxRequestPayloadSizeKB))
		}
	}

	// Request attempt count
	if regNetworkConfig.RequestAttemptCount != 0 && !network.CaddyfileFlags.RequestAttemptCountSetInCaddyfile {
		if int(regNetworkConfig.RequestAttemptCount) != network.RequestAttemptCount {
			network.RequestAttemptCount = int(regNetworkConfig.RequestAttemptCount)
			d.logger.Debug("Setting network request attempt count from registry",
				zap.String("network", network.Name),
				zap.Int("request_attempt_count", network.RequestAttemptCount))
		}
	}

	// Provider block history size
	if regNetworkConfig.ProviderBlockHistorySize != 0 && !network.CaddyfileFlags.ProviderBlockHistorySizeSetInCaddyfile {
		network.ProviderBlockHistorySize = int(regNetworkConfig.ProviderBlockHistorySize)
		d.logger.Debug("Setting provider block history size from registry",
			zap.String("network", network.Name),
			zap.Int("size", network.ProviderBlockHistorySize))
	}

	// Network block history size
	if regNetworkConfig.NetworkBlockHistorySize != 0 && !network.CaddyfileFlags.NetworkBlockHistorySizeSetInCaddyfile {
		network.NetworkBlockHistorySize = int(regNetworkConfig.NetworkBlockHistorySize)
		d.logger.Debug("Setting network block history size from registry",
			zap.String("network", network.Name),
			zap.Int("size", network.NetworkBlockHistorySize))
	}

	// Archive enabled - special case as it's a boolean
	if !network.CaddyfileFlags.ArchiveEnabledSetInCaddyfile {
		if regNetworkConfig.ArchiveEnabled != network.ArchiveEnabled {
			network.ArchiveEnabled = regNetworkConfig.ArchiveEnabled
			d.logger.Debug("Setting network archive enabled from registry",
				zap.String("network", network.Name),
				zap.Bool("archive_enabled", network.ArchiveEnabled))
		}
	}
}

// createNewProvider creates a new provider object and initializes the provider with the network service address
func (d *DinMiddleware) createNewProvider(networkName string, provider *provider, authConfig *din.ProviderAuthConfig, regNetworkService *din.NetworkService) (*provider, error) {
	httpClient := din_http.NewHTTPClient(time.Duration(DefaultHCTimeout) * time.Second)

	// Set the provider auth config based on the auth type
	if authConfig != nil {
		authClient, err := d.createProviderAuth(authConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider auth: %w", err)
		}
		if authClient != nil {
			provider.SetAuthClient(authClient)
		}
	}

	err := d.initializeProvider(networkName, provider, httpClient, d.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize provider: %w", err)
	}
	provider.Priority = d.Registry.Priority

	// Get the network service methods from the din registry
	provider.Methods = make(map[string]struct{})
	for _, method := range regNetworkService.Methods {
		provider.Methods[method.Name] = struct{}{}
	}
	return provider, nil
}

// createProviderAuth creates an auth client based on the registry auth config
func (d *DinMiddleware) createProviderAuth(authConfig *din.ProviderAuthConfig) (auth.IAuthClient, error) {
	switch authConfig.Type {
	case din.ProviderAuthTypeSIWE:
		// Build config for SIWE factory
		cfg := auth.AuthConfig{
			siwe.ConfigKeyURL:      authConfig.Url,
			siwe.ConfigKeySessions: siwe.DefaultSessionCount,
		}

		// Use default signer
		if d.DefaultSiweSigner == nil {
			return nil, fmt.Errorf("siwe default signer is not configured")
		}
		cfg[siwe.ConfigKeySigner] = d.DefaultSiweSigner

		return auth.Create("siwe", cfg)
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

// extractProviderSuffix extracts a unique suffix from URL path or headers
func (d *DinMiddleware) extractProviderSuffix(parsedUrl *url.URL, headers map[string]string) string {
	var suffix string

	// Try to extract suffix from URL path (e.g., validation cloud API keys)
	if parsedUrl != nil && parsedUrl.Path != "" {
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

	return suffix
}

// ensureUniqueProviderHost ensures the provider has a unique host identifier
// by appending API key suffix when multiple providers share the same base host.
// It first tries to extract a suffix from the URL path (e.g., validation cloud API keys),
// then from X-API-Key header (e.g., nodefleet), and falls back to a counter if neither is available.
// When a second provider with the same base host is detected, it retroactively updates the first provider
// to also have a suffix for consistency.
func (d *DinMiddleware) ensureUniqueProviderHost(networkName string, parsedUrl *url.URL, headers map[string]string) string {
	if parsedUrl == nil || parsedUrl.Host == "" {
		return ""
	}

	baseHost := parsedUrl.Host
	providers := d.Networks[networkName].Providers

	// Check if this exact URL already exists (duplicate provider)
	for existingHost, existingProvider := range providers {
		if existingProvider != nil && existingProvider.HttpUrl == parsedUrl.String() {
			if d.logger != nil {
				d.logger.Debug("Provider with same URL already exists",
					zap.String("network", networkName),
					zap.String("existingHost", existingHost),
					zap.String("url", parsedUrl.String()))
			}
			return existingHost
		}
	}

	// Find all providers with the same base host
	var sameBaseProviders []string
	var firstProvider *provider

	for host, provider := range providers {
		if host == baseHost {
			firstProvider = provider
			sameBaseProviders = append(sameBaseProviders, host)
		} else if strings.HasPrefix(host, baseHost+"-") {
			sameBaseProviders = append(sameBaseProviders, host)
		}
	}

	// First provider with this host - use base name
	if len(sameBaseProviders) == 0 {
		if d.logger != nil {
			d.logger.Debug("First provider with this host",
				zap.String("network", networkName),
				zap.String("host", baseHost))
		}
		return baseHost
	}

	// Extract suffix for current provider
	suffix := d.extractProviderSuffix(parsedUrl, headers)

	// Handle retroactive update for first provider (only when adding second provider)
	if firstProvider != nil && len(sameBaseProviders) == 1 && sameBaseProviders[0] == baseHost {
		d.retroactivelyUpdateFirstProvider(networkName, baseHost, firstProvider)
	}

	// Generate unique host name
	return d.generateUniqueHostName(networkName, baseHost, suffix, len(sameBaseProviders))
}

// retroactivelyUpdateFirstProvider updates the first provider with a suffix for consistency
func (d *DinMiddleware) retroactivelyUpdateFirstProvider(networkName, baseHost string, firstProvider *provider) {
	firstProviderUrl, err := url.Parse(firstProvider.HttpUrl)
	if err != nil {
		return
	}

	firstSuffix := d.extractProviderSuffix(firstProviderUrl, firstProvider.Headers)
	if firstSuffix == "" {
		return
	}

	newHost := fmt.Sprintf("%s-%s", baseHost, firstSuffix)

	// Check for conflicts
	if d.providerHostExists(networkName, newHost) {
		if d.logger != nil {
			d.logger.Warn("Cannot retroactively update first provider - name conflict",
				zap.String("network", networkName),
				zap.String("proposedHost", newHost))
		}
		return
	}

	// Update the provider
	firstProvider.host = newHost
	delete(d.Networks[networkName].Providers, baseHost)
	d.Networks[networkName].Providers[newHost] = firstProvider

	if d.logger != nil {
		d.logger.Info("Retroactively updated first provider with suffix",
			zap.String("network", networkName),
			zap.String("oldHost", baseHost),
			zap.String("newHost", newHost))
	}
}

// generateUniqueHostName creates a unique host name with suffix and handles collisions
func (d *DinMiddleware) generateUniqueHostName(networkName, baseHost, suffix string, existingCount int) string {
	// No suffix available - use counter
	if suffix == "" {
		return fmt.Sprintf("%s-%d", baseHost, existingCount)
	}

	proposedHost := fmt.Sprintf("%s-%s", baseHost, suffix)

	// Check for collision
	if !d.providerHostExists(networkName, proposedHost) {
		return proposedHost
	}

	// Handle collision with counter
	counter := 1
	for {
		alternativeHost := fmt.Sprintf("%s-%s-%d", baseHost, suffix, counter)
		if !d.providerHostExists(networkName, alternativeHost) {
			if d.logger != nil {
				d.logger.Debug("Using alternative host due to collision",
					zap.String("network", networkName),
					zap.String("host", alternativeHost))
			}
			return alternativeHost
		}
		counter++
	}
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
		var method = "unknown"
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
		var getBlockMethodName = "unknown"
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

func (d *DinMiddleware) getAPIKeyId(key string) string {
	if v, ok := d.ApiKeys[key]; ok {
		return v
	}
	h := sha256.New()
	h.Write([]byte(d.ApiSalt))
	h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil))[:10]
}

// GetOrCreateWatcherClient creates a new watcher client if it doesn't exist and returns the watcher client
func (d *DinMiddleware) GetOrCreateWatcherClient() watcher.IWatcherAPIClient {
	if d.DynamicLoadBalancing.watcherClient == nil {
		d.DynamicLoadBalancing.watcherClient = watcher.NewClient(d.DynamicLoadBalancing.WatcherApiEndpoint, d.DynamicLoadBalancing.WatcherApiKey)
	}
	return d.DynamicLoadBalancing.watcherClient
}

// Fetches the latest score from the watcher score manager and updates the provider score for all active networks
// Scores are already precomputed and stored in the watcher score manager internal state,
// so this function is just a way to move the scores to the middleware object for use in the load balancing logic.
func (d *DinMiddleware) SyncMiddlewareWithLatestScores() {

	// Keep track of the last time the scores were synced to the middleware
	d.DynamicLoadBalancing.WatcherScoreLastSyncTime = time.Now().UTC()

	// Lock (read)the middleware object to prevent race condition when looping through the networks/providers map
	d.mu.RLock()
	defer d.mu.RUnlock()

	d.logger.Info("[DYNAMIC_LB] Syncing watcher scores to the middleware")
	for _, network := range d.Networks {
		for _, provider := range network.Providers {
			newScore := d.DynamicLoadBalancing.watcherScoreManager.GetScore(network.Name, provider.host)
			if newScore.HasValue() {
				// As the name suggests, SafeUpdateScore is safe to use because it is protected by a write lock to only protect the score object (very fine-granular locking)
				provider.SafeUpdateScore(newScore)
				d.logger.Info("[DYNAMIC_LB] Synced watcher score",
					zap.String("network", network.Name),
					zap.String("provider", provider.host),
					zap.Float64("score_value", newScore.Value()),
					zap.String("score_updated_at", newScore.LastUpdated().Format(time.RFC3339)),
					zap.String("middleware_synced_at", d.DynamicLoadBalancing.WatcherScoreLastSyncTime.Format(time.RFC3339)),
					zap.Bool("is_healthy", provider.Healthy()),
					zap.Bool("is_warning", provider.Warning()))
			} else {
				d.logger.Info("[DYNAMIC_LB] No score found for provider",
					zap.String("network", network.Name),
					zap.String("provider", provider.host))
			}
		}
	}
}
