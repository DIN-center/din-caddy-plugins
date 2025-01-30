package modules

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type network struct {
	Name             string
	quit             chan struct{}
	HttpClient       din_http.IHTTPClient
	PrometheusClient prom.IPrometheusClient
	logger           *zap.Logger
	machineID        string
	ChainID          string

	// internal health check values
	healthCheckListMutex sync.RWMutex
	HCThreshold          int
	BlockHistorySize     int
	// CheckedProviders     map[string][]healthCheckEntry

	// Registry configuration values
	Providers               map[string]*provider `json:"providers"`
	Methods                 []*string            `json:"methods"`
	HCMethod                string               `json:"healthcheck_method"`
	ChainIDMethod           string               `json:"chainid_method"`
	HCInterval              int                  `json:"healthcheck_interval_seconds"`
	BlockLagLimit           int64                `json:"healthcheck_blocklag_limit"`
	BlockJumpLimit          int64                `json:"healthcheck_blockjump_limit"`
	MaxRequestPayloadSizeKB int64                `json:"max_request_payload_size_kb"`
	RequestAttemptCount     int                  `json:"request_attempt_count"`
}

// NewNetwork creates a new network with the given name
// Only put values in the struct definition that are constant
// Don't kick off any Background processes here
func NewNetwork(name string) *network {
	return &network{
		Name: name,
		// Default health check values, to be overridden if specified in the Caddyfile
		HCMethod:                DefaultHCMethod,
		ChainIDMethod:           DefaultChainIDMethod,
		HCThreshold:             DefaultHCThreshold,
		HCInterval:              DefaultHCInterval,
		BlockLagLimit:           DefaultBlockLagLimit,
		BlockJumpLimit:          DefaultBlockJumpLimit,
		MaxRequestPayloadSizeKB: DefaultMaxRequestPayloadSizeKB,
		RequestAttemptCount:     DefaultRequestAttemptCount,
		BlockHistorySize:        DefaultBlockHistorySize,
		// CheckedProviders: make(map[string][]healthCheckEntry),
		Providers: make(map[string]*provider),
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

// HealthCheck performs health checks on all providers and updates their status
func (n *network) healthCheck() {
	// First get latest blocks from all providers and update their history
	for _, provider := range n.Providers {
		blockNum, statusCode, err := n.getLatestBlockNumber(provider.HttpUrl, provider.Headers, provider.AuthClient())
		if err != nil {
			n.logger.Warn("Error getting latest block number for provider",
				zap.String("provider", provider.host),
				zap.String("network", n.Name),
				zap.Error(err),
				zap.String("machine_id", n.machineID))
			// Set provider status to Unhealthy on error
			provider.healthStatus = Unhealthy
			// Send metric with error status
			n.sendLatestBlockMetric(provider.host, statusCode, provider.healthStatus.String(), blockNum)
			continue
		}

		// Evaluate health status for this provider
		newStatus := n.evaluateProviderHealth(provider, blockNum, statusCode)
		provider.healthStatus = newStatus

		// Add block entry with final status
		provider.AddBlockEntry(blockNum, newStatus, n.BlockHistorySize)

		// Send metric with current status
		n.sendLatestBlockMetric(provider.host, statusCode, newStatus.String(), blockNum)
	}
}

// logProviderWarning logs a warning message with standard provider context
func (n *network) logProviderWarning(msg string, provider *provider, fields ...zap.Field) {
	baseFields := []zap.Field{
		zap.String("provider", provider.host),
		zap.String("network", n.Name),
		zap.String("machine_id", n.machineID),
	}
	n.logger.Warn(msg, append(baseFields, fields...)...)
}

// evaluateProviderHealth performs both levels of health checks
func (n *network) evaluateProviderHealth(provider *provider, currentBlock int64, statusCode int) HealthStatus {
	// Track the worst status we find
	worstStatus := Healthy

	// Check status code
	statusHealth := n.evaluateStatusCode(statusCode)
	if statusHealth > worstStatus {
		if statusCode == 429 {
			n.logProviderWarning("Provider returned a rate limit error", provider,
				zap.Int("status_code", statusCode))
		} else if statusCode >= 400 {
			n.logProviderWarning("Provider returned an error status code", provider,
				zap.Int("status_code", statusCode))
		}
		worstStatus = statusHealth
	}

	// Check chain ID - this is a critical check that should always result in Unhealthy
	if !n.verifyChainID(provider) {
		n.logProviderWarning("Provider has incorrect chain ID", provider)
		return Unhealthy
	}

	// if provider has no block history, set it to healthy
	if len(provider.BlockHistory()) == 0 {
		return Healthy
	}

	// Get latest network block for comparison
	latestNetworkBlock := n.getLatestHealthyBlock()

	// Check block lag
	if latestNetworkBlock > 0 {
		blockLag := int64(latestNetworkBlock) - currentBlock
		// If block lag is greater than limit, mark as warning
		if blockLag > n.BlockLagLimit {
			n.logProviderWarning("Provider is lagging behind network", provider,
				zap.Int64("block_lag", blockLag),
				zap.Int64("provider_block", currentBlock),
				zap.Int64("network_block", latestNetworkBlock))
			if Warning > worstStatus {
				worstStatus = Warning
			}
		}

		// Check if block is too far ahead (block jump)
		blockJump := currentBlock - int64(latestNetworkBlock)
		if blockJump > n.BlockJumpLimit {
			n.logProviderWarning("Provider is too far ahead of network", provider,
				zap.Int64("block_jump", blockJump),
				zap.Int64("provider_block", currentBlock),
				zap.Int64("network_block", latestNetworkBlock))
			return Unhealthy
		}
	}

	// Check for stalling - all blocks in history are identical
	if n.isStalled(provider) {
		if n.allProvidersStalled() {
			// This signifies a network outage
			n.logProviderWarning("All providers are stalled", provider)
			if Warning > worstStatus {
				worstStatus = Warning
			}
		} else {
			// This signifies a provider outage
			n.logProviderWarning("Provider is stalled while others are progressing", provider)
			return Unhealthy // Stalling when others aren't is always Unhealthy
		}
	}

	// Check monotonicity as quality indicator
	if !n.isMonotonic(provider) {
		n.logProviderWarning("Provider blocks are not monotonically increasing", provider)
		if Unhealthy > worstStatus {
			worstStatus = Unhealthy
		}
	}

	return worstStatus
}

// verifyChainID checks if provider is serving correct chain
func (n *network) verifyChainID(provider *provider) bool {
	return provider.getChainID() == n.ChainID
}

// isStalled checks if provider's block numbers haven't changed
func (n *network) isStalled(provider *provider) bool {
	history := provider.BlockHistory()
	// if history is less than the block history size, return false
	if len(history) < n.BlockHistorySize {
		return false
	}

	// if all blocks in history are the same, return true
	firstBlock := history[0].blockNumber
	for _, entry := range history[1:] {
		if entry.blockNumber != firstBlock {
			return false
		}
	}
	return true
}

// allProvidersStalled checks if all providers are showing no progress
func (n *network) allProvidersStalled() bool {
	for _, p := range n.Providers {
		if !n.isStalled(p) {
			return false
		}
	}
	return true
}

// isMonotonic checks if block numbers are non-decreasing
func (n *network) isMonotonic(provider *provider) bool {
	history := provider.BlockHistory()
	for i := 1; i < len(history); i++ {
		if history[i].blockNumber < history[i-1].blockNumber {
			return false
		}
	}
	return true
}

// getLatestHealthyBlock returns the highest block number among healthy providers
// Falls back to warning providers if no healthy providers are available
func (n *network) getLatestHealthyBlock() int64 {
	var latestBlock int64
	hasHealthyProviders := false

	// First try to get block from healthy providers
	for _, provider := range n.Providers {
		if provider.healthStatus == Healthy && len(provider.BlockHistory()) > 0 {
			history := provider.BlockHistory()
			lastBlock := history[len(history)-1].blockNumber
			if lastBlock > latestBlock {
				latestBlock = lastBlock
			}
			hasHealthyProviders = true
		}
	}

	// If no healthy providers, try warning providers
	if !hasHealthyProviders {
		for _, provider := range n.Providers {
			if provider.healthStatus == Warning && len(provider.BlockHistory()) > 0 {
				history := provider.BlockHistory()
				lastBlock := history[len(history)-1].blockNumber
				if lastBlock > latestBlock {
					latestBlock = lastBlock
				}
			}
		}
	}

	return latestBlock
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

func (n *network) getLatestBlockNumber(httpUrl string, headers map[string]string, ac auth.IAuthClient) (int64, int, error) {
	payload := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method": "%s","params":[],"id":1}`, n.HCMethod))

	// Send the POST request
	resBytes, statusCode, err := n.HttpClient.Post(httpUrl, headers, []byte(payload), ac)
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

func (n *network) getChainID(httpUrl string, headers map[string]string, ac auth.IAuthClient) (string, int, error) {
	payload := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method": "%s","params":[],"id":1}`, n.ChainIDMethod))

	// Send the POST request
	resBytes, statusCode, err := n.HttpClient.Post(httpUrl, headers, []byte(payload), ac)
	if err != nil {
		return "", 0, errors.Wrap(err, "Error sending POST request")
	}

	if *statusCode == http.StatusServiceUnavailable || *statusCode == StatusOriginUnreachable {
		return "", *statusCode, errors.New("Network Unavailable")
	}

	// response struct
	var respObject map[string]interface{}

	// Unmarshal the response
	err = json.Unmarshal(resBytes, &respObject)
	if err != nil {
		return "", 0, errors.Wrap(err, "Error unmarshalling response")
	}

	if _, ok := respObject["result"]; !ok {
		return "", 0, errors.New("Error getting chain ID from response")
	}

	var chainID string
	var ok bool

	// Bitcoin returns back chain ID nested in an object.
	if strings.Contains(n.Name, "bitcoin") {
		chainID, ok = respObject["result"].(map[string]interface{})["chain"].(string)
		if !ok {
			return "", 0, errors.New("Error getting chain ID from response")
		}
	}
	chainID, ok = respObject["result"].(string)
	if !ok {
		return "", 0, errors.New("Error getting chain ID from response")
	}

	return chainID, *statusCode, nil
}

func (n *network) close() {
	close(n.quit)
}

// evaluateStatusCode checks the HTTP status code and returns appropriate health status
func (n *network) evaluateStatusCode(statusCode int) HealthStatus {
	if statusCode >= 400 {
		if statusCode == 429 {
			return Warning
		}
		return Unhealthy
	}
	return Healthy
}
