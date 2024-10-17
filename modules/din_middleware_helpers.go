package modules

import (
	"fmt"
	"os"

	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
	dinreg "github.com/DIN-center/din-sc/apps/din-go/pkg/dinregistry"
	"go.uber.org/zap"
)

// syncRegistryWithLatestBlock checks the latest block number from the linea network and updates the middleware object with the latest registry data if the block number difference is greater than or equal to the epoch
func (d *DinMiddleware) syncRegistryWithLatestBlock() {
	// Check if the linea network exists in the middleware object
	network, ok := d.getNetwork(d.RegistryEnv)
	if !ok {
		d.logger.Error("Network not found in middleware object. Registry data cannot be retrieved", zap.String("network", d.RegistryEnv))
		return
	}
	// Get the latest block number from the linea network
	latestBlockNumber := network.latestBlockNumber

	// Calculate the latest block floor by epoch. for example if the current block number is 55 and the epoch is 10, then the latest block floor by epoch is 50.
	latestBlockFloorByEpoch := latestBlockNumber - (latestBlockNumber % d.RegistryBlockEpoch)

	d.logger.Debug("Checking block number for registry sync", zap.Int64("block_epoch", d.RegistryBlockEpoch),
		zap.Int64("latest_linea_block_number", latestBlockNumber), zap.Int64("latest_block_floor_by_epoch", latestBlockFloorByEpoch),
		zap.Int64("last_updated_block_number", d.registryLastUpdatedEpochBlockNumber), zap.Int64("difference", latestBlockFloorByEpoch-d.registryLastUpdatedEpochBlockNumber),
	)

	// If the difference between the latest block floor by epoch and the last updated block number is greater than or equal to the epoch, then update the networks and providers.
	if latestBlockFloorByEpoch-d.registryLastUpdatedEpochBlockNumber >= d.RegistryBlockEpoch {
		registryData, err := d.DingoClient.GetRegistryData()
		if err != nil {
			d.logger.Error("Failed to get data from registry", zap.Error(err))
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
				// Skip over network for now if it is not active
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
	network := NewNetwork(regNetwork.ProxyName)
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
		d.logger.Info("Starting healthcheck for network", zap.String("network", network.Name), zap.String("machine_id", d.machineID))
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
				// if the provider does exist in the copied network object, then update the provider data on the middleware object.
				d.updateProviderData(newNetwork.Name, newProvider)

				// remove the provider from the copied network object to keep track of the providers that are not in the registry network
				delete(newNetwork.Providers, newProvider.host)
			}
		}
	}

	// safely update the middleware network object with the copied network data.
	d.updateNetworkData(newNetwork)
	return nil
}

// syncNetworkConfig updates the network object with the registry network config data
func (d *DinMiddleware) syncNetworkConfig(regNetwork *din.Network, network *network) (*network, error) {
	registryHCMethod, err := d.DingoClient.GetNetworkMethodNameByBit(regNetwork.Name, regNetwork.NetworkConfig.HealthcheckMethodBit)
	if err != nil {
		d.logger.Error("Failed to get network healthcheck method name", zap.String("network", regNetwork.Name), zap.Error(err))
		return nil, err
	}

	// Sync the value if it is not 0 and different from the current middleware network value
	if registryHCMethod != "" && registryHCMethod != network.HCMethod {
		d.logger.Debug("Setting network healthcheck method", zap.String("network", network.Name), zap.String("method", registryHCMethod))
		network.HCMethod = registryHCMethod
	}
	registryHCInterval := int(regNetwork.NetworkConfig.HealthcheckIntervalSec)
	if registryHCInterval != 0 && registryHCInterval != network.HCInterval {
		d.logger.Debug("Setting network healthcheck interval", zap.String("network", network.Name), zap.Int("interval", registryHCInterval))
		network.HCInterval = registryHCInterval
	}
	registryBlockLagLimit := int64(regNetwork.NetworkConfig.BlockLagLimit)
	if registryBlockLagLimit != 0 && registryBlockLagLimit != network.BlockLagLimit {
		d.logger.Debug("Setting network block lag limit", zap.String("network", network.Name), zap.Int64("block_lag_limit", registryBlockLagLimit))
		network.BlockLagLimit = int64(registryBlockLagLimit)
	}
	registryMaxRequestPayloadSizeKB := int64(regNetwork.NetworkConfig.MaxRequestPayloadSizeKb)
	if registryMaxRequestPayloadSizeKB != 0 && registryMaxRequestPayloadSizeKB != network.MaxRequestPayloadSizeKB {
		d.logger.Debug("Setting network max request payload size", zap.String("network", network.Name), zap.Int64("max_request_payload_size_kb", registryMaxRequestPayloadSizeKB))
		network.MaxRequestPayloadSizeKB = registryMaxRequestPayloadSizeKB
	}
	registryRequestAttemptCount := int(regNetwork.NetworkConfig.RequestAttemptCount)
	if registryRequestAttemptCount != 0 && registryRequestAttemptCount != network.RequestAttemptCount {
		d.logger.Debug("Setting network request attempt count", zap.String("network", network.Name), zap.Int("request_attempt_count", registryRequestAttemptCount))
		network.RequestAttemptCount = int(registryRequestAttemptCount)
	}

	return network, nil
}

// createNewProvider creates a new provider object and initializes the provider with the network service address
func (d *DinMiddleware) createNewProvider(provider *provider, authConfig *dinreg.NetworkServiceAuthConfig, networkServiceAddress string) (*provider, error) {
	httpClient := din_http.NewHTTPClient()

	// TODO: Finish when the signer is configured globally in the Caddy config instead of per provider
	// Set the provider auth config based on the auth type
	// if authConfig != nil {
	// 	switch authConfig.Type {
	// 	case dinreg.SIWE:
	// 		provider.Auth = &siwe.SIWEClientAuth{
	// 			ProviderURL:  authConfig.Url,
	// 			SessionCount: 16,
	// 		}
	// 	default:
	// 		provider.Auth = nil
	// 	}
	// }

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
	provider.Methods = networkServiceMethods

	return provider, nil
}

func (d *DinMiddleware) updateProviderData(networkName string, provider *provider) {
	// update the provider object with the registry provider data
	d.Networks[networkName].Providers[provider.host].Auth = provider.Auth
	d.Networks[networkName].Providers[provider.host].Methods = provider.Methods
}

// updateNetwork updates the network object in the middleware object with the provided registry network data
func (d *DinMiddleware) updateNetworkData(network *network) {
	// update the network object with the registry network config data
	d.Networks[network.Name].HCMethod = network.HCMethod
	d.Networks[network.Name].HCInterval = network.HCInterval
	d.Networks[network.Name].BlockLagLimit = network.BlockLagLimit
	d.Networks[network.Name].MaxRequestPayloadSizeKB = network.MaxRequestPayloadSizeKB
	d.Networks[network.Name].RequestAttemptCount = network.RequestAttemptCount

	// add the new providers to the middleware network.Providers map
	for _, p := range network.Providers {
		d.Networks[network.Name].Providers[p.host] = p
	}
}

// getMachineId returns a unique string for the current running process
func getMachineId() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "UNKNOWN"
	}
	currentPid := os.Getpid()
	return fmt.Sprintf("@%s:%d", hostname, currentPid)
}
