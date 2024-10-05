package modules

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type network struct {
	Name              string               `json:"name"`
	Providers         map[string]*provider `json:"providers"`
	Methods           []*string            `json:"methods"`
	quit              chan struct{}
	LatestBlockNumber int64 `json:"latest_block_number"`
	HTTPClient        din_http.IHTTPClient
	PrometheusClient  prom.IPrometheusClient
	logger            *zap.Logger
	machineID         string

	healthCheckListMutex sync.RWMutex

	// Healthcheck configuration
	CheckedProviders        map[string][]healthCheckEntry `json:"checked_providers"`
	HCMethod                string                        `json:"healthcheck_method"`
	HCInterval              int                           `json:"healthcheck_interval_seconds"`
	HCThreshold             int                           `json:"healthcheck_threshold"`
	BlockLagLimit           int64                         `json:"healthcheck_blocklag_limit"`
	MaxRequestPayloadSizeKB int64                         `json:"max_request_payload_size_kb"`
	RequestAttemptCount     int                           `json:"request_attempt_count"`
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
		MaxRequestPayloadSizeKB: DefaultMaxRequestPayloadSizeKB,
		RequestAttemptCount:     DefaultRequestAttemptCount,

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
			// get the latest block number from the current provider
			providerBlockNumber, statusCode, err := n.getLatestBlockNumber(provider.HttpUrl, provider.Headers, provider.AuthClient())
			if err != nil {
				// if there is an error getting the latest block number, mark the provider as a failure
				n.logger.Warn("Error getting latest block number for provider", zap.String("provider", providerName), zap.String("network", n.Name), zap.Error(err), zap.String("machine_id", n.machineID))
				provider.markPingFailure(n.HCThreshold)
				n.sendLatestBlockMetric(provider.host, statusCode, provider.healthStatus.String(), providerBlockNumber)
				return
			}
			blockTime = time.Now()

			// Ping Health Check
			if statusCode > 399 {
				if statusCode == 429 {
					// if the status code is 429, mark the provider as a warning
					n.logger.Warn("Provider is rate limited", zap.String("provider", providerName), zap.String("network", n.Name), zap.String("machine_id", n.machineID))
					provider.markPingWarning()
				} else {
					// if the status code is greater than 399, mark the provider as a failure
					n.logger.Warn("Provider returned an error status code", zap.String("provider", providerName), zap.String("network", n.Name), zap.Int("status_code", statusCode), zap.String("machine_id", n.machineID))
					provider.markPingFailure(n.HCThreshold)
				}
				n.sendLatestBlockMetric(provider.host, statusCode, provider.healthStatus.String(), providerBlockNumber)
				return
			} else {
				provider.markPingSuccess(n.HCThreshold)
			}

			// Consistency health check
			if n.LatestBlockNumber == 0 || n.LatestBlockNumber < providerBlockNumber {
				// if the current provider's latest block number is greater than the network's latest block number, update the network's latest block number,
				// set the current provider as healthy and loop through all of the previously checked providers and set them as unhealthy
				n.LatestBlockNumber = providerBlockNumber
				provider.markHealthy()
				n.evaluateCheckedProviders()
			} else if n.LatestBlockNumber == providerBlockNumber {
				// if the current provider's latest block number is equal to the network's latest block number, set the current provider to healthy
				provider.markHealthy()
			} else if providerBlockNumber+n.BlockLagLimit < n.LatestBlockNumber {
				// if the current provider's latest block number is below the network's latest block number by more than the acceptable threshold, set the current provider to warning
				n.logger.Warn("Provider is lagging behind", zap.String("provider", providerName), zap.String("network", n.Name), zap.Int64("provider_block_number", providerBlockNumber), zap.Int64("network_block_number", n.LatestBlockNumber), zap.String("machine_id", n.machineID))
				provider.markWarning()
			}

			// TODO: create a check based on time window of a provider's latest block number
			n.sendLatestBlockMetric(provider.host, statusCode, provider.healthStatus.String(), providerBlockNumber)

			// add the current provider to the checked providers map
			n.addHealthCheckToCheckedProviderList(provider.host, healthCheckEntry{blockNumber: providerBlockNumber, timestamp: &blockTime})
		}(name, currentProvider) // Pass the loop variable to the goroutine
	}
	// Wait for all goroutines to complete
	wg.Wait()
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
	n.mu.RLock()
	defer n.mu.RUnlock()
	values, ok := n.CheckedProviders[providerName]
	return values, ok
}

func (n *network) setCheckedProviderHCList(providerName string, newHealthCheckList []healthCheckEntry) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.CheckedProviders[providerName] = newHealthCheckList
}

// evaluateCheckedProviders loops through all of the checked providers and sets them as unhealthy if they are not the current provider
func (n *network) evaluateCheckedProviders() {
	// read lock the checked providers map
	n.mu.RLock()
	defer n.mu.RUnlock()
	// loop through all of the checked providers and set them as unhealthy if they are not the current provider
	checkedProviders := n.CheckedProviders
	for providerName, healthCheckList := range checkedProviders {
		if healthCheckList[0].blockNumber+n.BlockLagLimit < n.LatestBlockNumber {
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

func (n *network) getLatestBlockNumber(httpUrl string, headers map[string]string, ac auth.IAuthClient) (int64, int, error) {
	payload := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method": "%s","params":[],"id":1}`, n.HCMethod))

	// Send the POST request
	resBytes, statusCode, err := n.HTTPClient.Post(httpUrl, headers, []byte(payload), ac)
	if err != nil {
		return 0, 0, errors.Wrap(err, "Error sending POST request")
	}

	if *statusCode == http.StatusServiceUnavailable || *statusCode == StatusOriginUnreachable {
		return 0, *statusCode, errors.New("Network Unavailable")
	}

	// response struct
	var respObject map[string]interface{}

	// Unmarshal the response
	err = json.Unmarshal(resBytes, &respObject)
	if err != nil {
		return 0, 0, errors.Wrap(err, "Error unmarshalling response")
	}

	if _, ok := respObject["result"]; !ok {
		return 0, 0, errors.New("Error getting block number from response")
	}

	var blockNumber int64

	switch result := respObject["result"].(type) {
	case string:
		if result == "" || result[:2] != "0x" {
			return 0, 0, errors.New("Invalid block number")
		}

		// Convert the hexadecimal string to an int64
		blockNumber, err = strconv.ParseInt(result[2:], 16, 64)
		if err != nil {
			return 0, 0, errors.Wrap(err, "Error converting block number")
		}
	case float64:
		blockNumber = int64(result)
	default:
		return 0, 0, errors.New("unsupported block number type")
	}
	return blockNumber, *statusCode, nil
}

func (n *network) close() {
	close(n.quit)
}
