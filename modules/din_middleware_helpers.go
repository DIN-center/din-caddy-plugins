package modules

import (
	"fmt"

	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
	dinreg "github.com/DIN-center/din-sc/apps/din-go/pkg/dinregistry"
	"go.uber.org/zap"
)

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
	network := NewNetwork(regNetwork.ProxyName, d.Env)
	network, err := d.syncNetworkConfig(regNetwork, network)
	if err != nil {
		d.logger.Error("Failed to sync network config", zap.Error(err))
		return err
	}

	httpClient := din_http.NewHTTPClient()
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

			provider, err = d.createNewProvider(provider, regProvider.AuthConfig, networkService.Address)
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
				newProvider, err := d.createNewProvider(newProvider, regProvider.AuthConfig, networkService.Address)
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
	// Get the healthcheck method name from the registry, usually eth_blockNumber or similar
	registryHCMethod, err := d.DingoClient.GetNetworkMethodNameByBit(regNetwork.Name, regNetwork.NetworkConfig.HealthcheckMethodBit)
	if err != nil {
		d.logger.Error("Failed to get network healthcheck method name", zap.String("network", regNetwork.Name), zap.Error(err))
		return nil, err
	}

	// Get the chain ID method name from the registry, usually eth_chainId or similar
	registryChainIdMethod, err := d.DingoClient.GetNetworkMethodNameByBit(regNetwork.Name, regNetwork.NetworkConfig.ChainIdMethodBit)
	if err != nil {
		d.logger.Error("Failed to get network chain ID method name", zap.String("network", regNetwork.Name), zap.Error(err))
		return nil, err
	}

	// Get the call contract method name from the registry, usually eth_call or similar
	registryCallContractMethod, err := d.DingoClient.GetNetworkMethodNameByBit(regNetwork.Name, regNetwork.NetworkConfig.CallContractMethodBit)
	if err != nil {
		d.logger.Error("Failed to get network call contract method name", zap.String("network", regNetwork.Name), zap.Error(err))
		return nil, err
	}

	// Update Chain ID if changed
	if regNetwork.NetworkConfig.ChainId != "" && regNetwork.NetworkConfig.ChainId != network.ChainId {
		d.logger.Debug("Setting network chain Id",
			zap.String("network", network.Name),
			zap.String("chain_id", regNetwork.NetworkConfig.ChainId))
		network.ChainId = regNetwork.NetworkConfig.ChainId
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
	hcInterval := int(regNetwork.NetworkConfig.HealthcheckIntervalSec)
	if hcInterval != 0 && hcInterval != network.HCInterval {
		d.logger.Debug("Setting network healthcheck interval",
			zap.String("network", network.Name),
			zap.Int("interval", hcInterval))
		network.HCInterval = hcInterval
	}

	// Update Block Lag Limit if changed
	blockLagLimit := int64(regNetwork.NetworkConfig.BlockLagLimit)
	if blockLagLimit != 0 && blockLagLimit != network.BlockLagLimit {
		d.logger.Debug("Setting network block lag limit",
			zap.String("network", network.Name),
			zap.Int64("block_lag_limit", blockLagLimit))
		network.BlockLagLimit = blockLagLimit
	}

	// Update Block Jump Limit if changed
	blockJumpLimit := int64(regNetwork.NetworkConfig.BlockJumpLimit)
	if blockJumpLimit != 0 && blockJumpLimit != network.BlockJumpLimit {
		d.logger.Debug("Setting network block jump limit",
			zap.String("network", network.Name),
			zap.Int64("block_jump_limit", blockJumpLimit))
		network.BlockJumpLimit = blockJumpLimit
	}

	// Update Max Request Payload Size if changed
	maxPayloadSize := int64(regNetwork.NetworkConfig.MaxRequestPayloadSizeKb)
	if maxPayloadSize != 0 && maxPayloadSize != network.MaxRequestPayloadSizeKB {
		d.logger.Debug("Setting network max request payload size",
			zap.String("network", network.Name),
			zap.Int64("max_payload_size_kb", maxPayloadSize))
		network.MaxRequestPayloadSizeKB = maxPayloadSize
	}

	// Update Request Attempt Count if changed
	requestAttempts := int(regNetwork.NetworkConfig.RequestAttemptCount)
	if requestAttempts != 0 && requestAttempts != network.RequestAttemptCount {
		d.logger.Debug("Setting network request attempt count",
			zap.String("network", network.Name),
			zap.Int("request_attempts", requestAttempts))
		network.RequestAttemptCount = requestAttempts
	}

	// Update Archive Enabled if changed
	archiveEnabled := regNetwork.NetworkConfig.ArchiveEnabled
	if archiveEnabled != network.ArchiveEnabled {
		d.logger.Debug("Setting network archive enabled",
			zap.String("network", network.Name),
			zap.Bool("archive_enabled", archiveEnabled))
		network.ArchiveEnabled = archiveEnabled
	}

	return network, nil
}

// createNewProvider creates a new provider object and initializes the provider with the network service address
func (d *DinMiddleware) createNewProvider(provider *provider, authConfig *dinreg.NetworkServiceAuthConfig, networkServiceAddress string) (*provider, error) {
	httpClient := din_http.NewHTTPClient()

	// Set the provider auth config based on the auth type
	if authConfig != nil {
		var err error
		provider.Auth, err = d.createProviderSIWEAuth(authConfig)
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
