package modules

import (
	"sort"
	"sync"
	"time"

	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	"go.uber.org/zap"
)

type network struct {
	Name              string
	quit              chan struct{}
	latestBlockNumber int64
	HttpClient        din_http.IHTTPClient
	PrometheusClient  prom.IPrometheusClient
	logger            *zap.Logger
	machineID         string

	// internal health check values
	healthCheckListMutex sync.RWMutex
	HCThreshold          int
	CheckedProviders     map[string][]healthCheckEntry

	// Registry configuration values
	Providers               map[string]*provider `json:"providers"`
	Methods                 []*string            `json:"methods"`
	HCMethod                string               `json:"healthcheck_method"`
	HCInterval              int                  `json:"healthcheck_interval_seconds"`
	BlockLagLimit           int64                `json:"healthcheck_blocklag_limit"`
	BlockNumberDelta        int64                `json:"block_number_delta"`
	MaxRequestPayloadSizeKB int64                `json:"max_request_payload_size_kb"`
	RequestAttemptCount     int                  `json:"request_attempt_count"`

	// EVMSpeedEnabled is a flag to enable EVM Speed service provided by Infura
	EVMSpeedEnabled bool `json:"evm_speed_enabled"`
}

// NewNetwork creates a new network with the given name
// Only put values in the struct definition that are constant
// Don't kick off any Background processes here
func NewNetwork(name string) *network {
	return &network{
		Name: name,
		// Default health check values, to be overridden if specified in the Caddyfile
		HCMethod:                DefaultHCMethod,
		HCThreshold:             DefaultHCThreshold,
		HCInterval:              DefaultHCInterval,
		BlockLagLimit:           DefaultBlockLagLimit,
		BlockNumberDelta:        DefaultBlockNumberDelta,
		MaxRequestPayloadSizeKB: DefaultMaxRequestPayloadSizeKB,
		RequestAttemptCount:     DefaultRequestAttemptCount,
		EVMSpeedEnabled:         DefaultEVMSpeedEnabled,

		CheckedProviders: make(map[string][]healthCheckEntry),
		Providers:        make(map[string]*provider),
	}
}

func (n *network) startHealthcheck() {
	n.healthCheck()
	ticker := time.NewTicker(time.Second * time.Duration(n.HCInterval))
	go func() {
		// Keep an index for RPC request IDs
		for i := 0; ; i++ {
			select {
			// Cleanup if the quit channel gets closed. Right now nothing closes this channel, but
			// once we integrate the authentication work there's code that should.
			case <-n.quit:
				ticker.Stop()
				return
			case <-ticker.C:
				// Set up the healthcheck request with authentication for this provider.
				n.healthCheck()
			}
		}
	}()
}

type healthCheckEntry struct {
	blockNumber int64
	timestamp   *time.Time
}

func (n *network) healthCheck() {
	// wait group to wait for all the providers to finish their health checks
	var wg sync.WaitGroup
	var blockTime time.Time

	for name, currentProvider := range n.Providers {
		// check all of the providers simultaneously using async job management for more accurate blocknumber results.
		wg.Add(1) // Increment the WaitGroup counter
		go func(providerName string, provider *provider) {
			defer wg.Done() // Decrement the counter when the goroutine completes
			// Use provider's method instead
			providerBlockNumber, statusCode, err := provider.getLatestBlockNumber(n.HCMethod)
			if err != nil {
				n.handleBlockNumberError(providerName, provider, statusCode, providerBlockNumber, err)
				return
			}
			blockTime = time.Now()

			err = provider.saveLatestBlockNumber(providerBlockNumber)
			if err != nil {
				n.logger.Error("Error saving block number", zap.String("provider", providerName), zap.String("network", n.Name), zap.Error(err), zap.String("machine_id", n.machineID))
				return
			}

			if n.EVMSpeedEnabled {
				// Only check and set earliest block number if it hasn't been set yet.
				// earliest block to check will be block 1.
				if provider.earliestBlockNumber == 0 {
					earliestBlockNumber, statusCode, err := provider.getEarliestBlockNumber(DefaultGetBlockNumberMethod, n.RequestAttemptCount)
					if err != nil {
						n.handleBlockNumberError(providerName, provider, statusCode, providerBlockNumber, err)
						return
					}

					err = provider.saveEarliestBlockNumber(earliestBlockNumber)
					if err != nil {
						n.logger.Error("Error saving earliest block number", zap.String("provider", providerName), zap.String("network", n.Name), zap.Error(err), zap.String("machine_id", n.machineID))
						return
					}
				}
			}

			if n.pingHealthCheck(providerName, provider, statusCode, providerBlockNumber) {
				return
			}

			if n.blockNumberDeltaHealthCheck(providerName, provider, providerBlockNumber) {
				return
			}

			n.consistencyHealthCheck(providerName, provider, providerBlockNumber)

			n.sendLatestBlockMetric(provider.host, statusCode, provider.healthStatus.String(), providerBlockNumber)

			// add the current provider to the checked providers map
			n.addHealthCheckToCheckedProviderList(provider.host, healthCheckEntry{blockNumber: providerBlockNumber, timestamp: &blockTime})
		}(name, currentProvider) // Pass the loop variable to the goroutine
	}
	// Wait for all goroutines to complete
	wg.Wait()
}

func (n *network) handleBlockNumberError(providerName string, provider *provider, statusCode int, providerBlockNumber int64, err error) {
	n.logger.Warn("Error getting latest block number for provider", zap.String("provider", providerName), zap.String("network", n.Name), zap.Error(err), zap.String("machine_id", n.machineID))
	provider.markPingFailure(n.HCThreshold)
	n.sendLatestBlockMetric(provider.host, statusCode, provider.healthStatus.String(), providerBlockNumber)
}

func (n *network) pingHealthCheck(providerName string, provider *provider, statusCode int, providerBlockNumber int64) bool {
	if statusCode > 399 {
		if statusCode == 429 {
			n.logger.Warn("Provider is rate limited", zap.String("provider", providerName), zap.String("network", n.Name), zap.String("machine_id", n.machineID))
			provider.markPingWarning()
		} else {
			n.logger.Warn("Provider returned an error status code", zap.String("provider", providerName), zap.String("network", n.Name), zap.Int("status_code", statusCode), zap.String("machine_id", n.machineID))
			provider.markPingFailure(n.HCThreshold)
		}
		n.sendLatestBlockMetric(provider.host, statusCode, provider.healthStatus.String(), providerBlockNumber)
		return true
	}
	provider.markPingSuccess(n.HCThreshold)
	return false
}

func (n *network) blockNumberDeltaHealthCheck(providerName string, provider *provider, providerBlockNumber int64) bool {
	// If there's only one provider, any block number is acceptable
	if len(n.Providers) == 1 {
		return false
	}

	// Use 75th percentile as reference point
	referenceBlock := n.getPercentileBlockNumber(0.75)
	if referenceBlock == 0 {
		// Not enough data to make a determination
		return false
	}

	// Check if the provider's block number is too far from the reference block
	if providerBlockNumber > referenceBlock+n.BlockNumberDelta {
		n.logger.Warn("Provider is too far ahead of the network",
			zap.String("provider", providerName),
			zap.String("network", n.Name),
			zap.Int64("provider_block_number", providerBlockNumber),
			zap.Int64("reference_block_number", referenceBlock),
			zap.String("machine_id", n.machineID))
		provider.markUnhealthy()
		return true
	} else if providerBlockNumber < referenceBlock-n.BlockNumberDelta {
		n.logger.Warn("Provider is too far behind the network",
			zap.String("provider", providerName),
			zap.String("network", n.Name),
			zap.Int64("provider_block_number", providerBlockNumber),
			zap.Int64("reference_block_number", referenceBlock),
			zap.String("machine_id", n.machineID))
		provider.markUnhealthy()
		return true
	}
	return false
}

func (n *network) consistencyHealthCheck(providerName string, provider *provider, providerBlockNumber int64) {
	// For a single provider, always consider it healthy if it's responding
	if len(n.Providers) == 1 {
		provider.markHealthy(n.HCThreshold)
		n.latestBlockNumber = providerBlockNumber
		return
	}

	referenceBlock := n.getPercentileBlockNumber(0.75)
	if referenceBlock == 0 {
		// First health check or not enough data
		n.latestBlockNumber = providerBlockNumber
		provider.markHealthy(n.HCThreshold)
		return
	}

	// Update network's latest block number if we see a higher one
	// Move this before the lag check to ensure we capture the highest block
	if providerBlockNumber > n.latestBlockNumber {
		n.latestBlockNumber = providerBlockNumber
	}

	// Also update latest block number with reference block if it's higher
	if referenceBlock > n.latestBlockNumber {
		n.latestBlockNumber = referenceBlock
	}

	if providerBlockNumber+n.BlockLagLimit < referenceBlock {
		n.logger.Warn("Provider is lagging behind",
			zap.String("provider", providerName),
			zap.String("network", n.Name),
			zap.Int64("provider_block_number", providerBlockNumber),
			zap.Int64("reference_block_number", referenceBlock),
			zap.String("machine_id", n.machineID))
		provider.markWarning()
	} else {
		provider.markHealthy(n.HCThreshold)
	}
}

func (n *network) sendLatestBlockMetric(providerName string, statusCode int, healthStatus string, providerBlockNumber int64) {
	n.PrometheusClient.HandleLatestBlockMetric(&prom.PromLatestBlockMetricData{
		Network:        n.Name,
		Provider:       providerName,
		ResponseStatus: statusCode,
		HealthStatus:   healthStatus,
		BlockNumber:    providerBlockNumber,
	})
}

func (n *network) getCheckedProviderHCList(providerName string) ([]healthCheckEntry, bool) {
	n.healthCheckListMutex.RLock()
	defer n.healthCheckListMutex.RUnlock()
	values, ok := n.CheckedProviders[providerName]
	return values, ok
}

func (n *network) setCheckedProviderHCList(providerName string, newHealthCheckList []healthCheckEntry) {
	n.healthCheckListMutex.Lock()
	defer n.healthCheckListMutex.Unlock()
	n.CheckedProviders[providerName] = newHealthCheckList
}

// evaluateCheckedProviders loops through all of the checked providers and sets them as unhealthy if they are not the current provider
func (n *network) evaluateCheckedProviders() {
	// read lock the checked providers map
	n.healthCheckListMutex.RLock()
	defer n.healthCheckListMutex.RUnlock()
	// loop through all of the checked providers and set them as unhealthy if they are not the current provider
	checkedProviders := n.CheckedProviders
	for providerName, healthCheckList := range checkedProviders {
		if healthCheckList[0].blockNumber+n.BlockLagLimit < n.latestBlockNumber {
			n.Providers[providerName].markWarning()
		}
	}
}

// addHealthCheckToCheckedProviderList adds a new healthCheckEntry to the beginning of the CheckedProviders healthCheck list for the given provider
// the list will not exceed 10 entries
func (n *network) addHealthCheckToCheckedProviderList(providerName string, healthCheckInput healthCheckEntry) {
	// if the provider is not in the checked providers map, add it with its initial block number and timestamp
	currentHealthCheckList, ok := n.getCheckedProviderHCList(providerName)
	if !ok {
		n.setCheckedProviderHCList(providerName, []healthCheckEntry{healthCheckInput})
		return
	}

	// to add a new healthCheckEntry to index 0 of the provider's slice, we need to make a new slice and copy the old slice to the new slice
	newHealthCheckList := []healthCheckEntry{healthCheckInput}

	// if the old slice is full at 10 entries, we need to remove the last entry and append the rest of the entries to the new slice
	if len(currentHealthCheckList) == 10 {
		currentHealthCheckList = append(newHealthCheckList, currentHealthCheckList[:9]...)
		n.setCheckedProviderHCList(providerName, currentHealthCheckList)
	} else {
		// if the old slice is not full, we can copy the old slice to the new slice and add the new entry to index 0
		currentHealthCheckList = append(newHealthCheckList, currentHealthCheckList...)
		n.setCheckedProviderHCList(providerName, currentHealthCheckList)
	}
}

func (n *network) close() {
	close(n.quit)
}

// getPercentileBlockNumber returns the block number at the specified percentile across all providers
// percentile should be between 0 and 1 (e.g., 0.75 for 75th percentile)
func (n *network) getPercentileBlockNumber(percentile float64) int64 {
	if len(n.Providers) == 0 {
		return 0
	}

	// Collect all block numbers
	blockNumbers := make([]int64, 0, len(n.Providers))
	for _, provider := range n.Providers {
		// Get the most recent block number from the provider's health check entries
		entries, ok := n.getCheckedProviderHCList(provider.host)
		if ok && len(entries) > 0 {
			blockNumbers = append(blockNumbers, entries[0].blockNumber)
		}
	}

	if len(blockNumbers) == 0 {
		return 0
	}

	// Sort block numbers
	sort.Slice(blockNumbers, func(i, j int) bool {
		return blockNumbers[i] < blockNumbers[j]
	})

	// Calculate the index for the percentile
	index := int(float64(len(blockNumbers)-1) * percentile)
	return blockNumbers[index]
}
