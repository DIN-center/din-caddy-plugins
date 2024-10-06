package modules

import (
	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
	"go.uber.org/zap"
)

func (d *DinMiddleware) syncRegistryWithLatestBlock() {
	// Check if the linea network exists in the middleware object
	network, ok := d.GetNetwork(d.RegistryEnv)
	if !ok {
		d.logger.Error("Network not found in middleware object. Registry data cannot be retrieved", zap.String("network", d.RegistryEnv))
		return
	}
	// Get the latest block number from the linea network
	latestBlockNumber := network.LatestBlockNumber

	// Calculate the latest block floor by epoch. for example if the current block number is 55 and the epoch is 10, then the latest block floor by epoch is 50.
	latestBlockFloorByEpoch := latestBlockNumber - (latestBlockNumber % d.RegistryBlockEpoch)

	d.logger.Debug("Checking block number for registry sync", zap.Int64("block_epoch", d.RegistryBlockEpoch),
		zap.Int64("latest_linea_block_number", latestBlockNumber), zap.Int64("latest_block_floor_by_epoch", latestBlockFloorByEpoch),
		zap.Int64("last_updated_block_number", d.RegistryLastUpdatedEpochBlockNumber), zap.Int64("difference", latestBlockFloorByEpoch-d.RegistryLastUpdatedEpochBlockNumber),
	)

	// If the difference between the latest block floor by epoch and the last updated block number is greater than or equal to the epoch, then update the networks and providers.
	if latestBlockFloorByEpoch-d.RegistryLastUpdatedEpochBlockNumber >= d.RegistryBlockEpoch {
		registryData, err := d.DingoClient.GetRegistryData()
		if err != nil {
			d.logger.Error("Failed to get data from registry", zap.Error(err))
		}
		d.processRegistryData(registryData)

		// Update the last updated block number
		d.RegistryLastUpdatedEpochBlockNumber = latestBlockFloorByEpoch

	}
}

// TODO: finish this.
func (d *DinMiddleware) processRegistryData(registryData *din.DinRegistryData) {
	// Loop through the networks in the din registry
	for _, regNetwork := range registryData.Networks {
		// For each network, check if the network is provisioned, if not, skip the network
		if regNetwork.NetworkConfig == nil || !regNetwork.NetworkConfig.IsProvisioned {
			continue
		}

		// Check if the network exists in the local network list within the middleware object
		_, ok := d.GetNetwork(regNetwork.ProxyName)
		if !ok {
			// If the network does not exist in the middleware object, then create a new network and add it to the middleware object
			err := d.AddNetworkFromRegistry(&regNetwork)
			if err != nil {
				// If there is an error adding the network, log the error and continue to the next registry network
				d.logger.Error("Failed to add network from registry", zap.Error(err))
				continue
			}
		} else {
			// If the network exists in the middleware object, then update the existing network in place with the registry data
			err := d.UpdateNetworkWithRegistryData(&regNetwork)
			if err != nil {
				d.logger.Error("Failed to update network with registry data", zap.Error(err))
				continue
			}
		}
	}
}

func (d *DinMiddleware) AddNetworkFromRegistry(regNetwork *din.Network) error {
	network := NewNetwork(regNetwork.ProxyName)
	// Set the local network network config values to the registry network config values
	registryHCMethod, err := d.DingoClient.GetNetworkMethodNameByBit(regNetwork.Name, regNetwork.NetworkConfig.HealthcheckMethodBit)
	if err != nil {
		d.logger.Error("Failed to get network healthcheck method name", zap.String("network", regNetwork.Name), zap.Error(err))
		return err
	}

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

	for _, regProvider := range regNetwork.Providers {
		for _, networkService := range regProvider.NetworkServices {
			// If the provider does not exist, create a new provider object
			provider, err := NewProvider(networkService.Url)
			if err != nil {
				d.logger.Error("Failed to create new provider object", zap.Error(err))
				return err
			}

			// set the provider priority to the registry priority
			provider.Priority = d.RegistryPriority

			// Get the network service methods from the din registry
			networkServiceMethods, err := d.DingoClient.GetNetworkServiceMethods(networkService.Address)
			if err != nil {
				d.logger.Error("Failed to get network service methods", zap.String("network_service_address", networkService.Address), zap.Error(err))
				return err
			}
			provider.Methods = networkServiceMethods

			// TODO: FINISH THIS set auth type and object with url
			provider.Auth = nil

			// Add the provider to the network object
			network.Providers[provider.host] = provider
		}
	}
	// Safely add the network to the middleware object
	d.AddNetwork(network)
	return nil
}

func (d *DinMiddleware) UpdateNetworkWithRegistryData(regNetwork *din.Network) error {
	return nil
}

func (d *DinMiddleware) GetNetwork(networkName string) (*network, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	n, ok := d.Networks[networkName]
	return n, ok
}

func (d *DinMiddleware) GetNetworks() []*network {
	d.mu.RLock()
	defer d.mu.RUnlock()

	networks := make([]*network, 0)
	for _, n := range d.Networks {
		networks = append(networks, n)
	}
	return networks
}

func (d *DinMiddleware) AddNetwork(network *network) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.Networks[network.Name] = network
}
