package modules

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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

	// internal health check values
	HCThreshold      int
	BlockHistorySize int

	// Registry configuration values
	Providers               map[string]*provider `json:"providers"`
	Methods                 []*string            `json:"methods"`
	HCMethod                string               `json:"healthcheck_method"`
	ChainIdMethod           string               `json:"chainid_method"`
	ChainId                 string               `json:"chain_id"`
	CallContractMethod      string               `json:"call_contract_method"`
	HCInterval              int                  `json:"healthcheck_interval_seconds"`
	BlockLagLimit           int64                `json:"healthcheck_blocklag_limit"`
	BlockJumpLimit          int64                `json:"healthcheck_blockjump_limit"`
	MaxRequestPayloadSizeKB int64                `json:"max_request_payload_size_kb"`
	RequestAttemptCount     int                  `json:"request_attempt_count"`
	ArchiveEnabled          bool                 `json:"archive_enabled"`
}

// NewNetwork creates a new network with the given name
// Only put values in the struct definition that are constant
// Don't kick off any Background processes here
func NewNetwork(name string) *network {
	return &network{
		Name: name,
		// Default health check values, to be overridden if specified in the Caddyfile
		HCMethod:                DefaultHCMethod,
		ChainIdMethod:           DefaultChainIdMethod,
		CallContractMethod:      DefaultCallContractMethod,
		HCThreshold:             DefaultHCThreshold,
		HCInterval:              DefaultHCInterval,
		BlockLagLimit:           DefaultBlockLagLimit,
		BlockJumpLimit:          DefaultBlockJumpLimit,
		MaxRequestPayloadSizeKB: DefaultMaxRequestPayloadSizeKB,
		RequestAttemptCount:     DefaultRequestAttemptCount,
		BlockHistorySize:        BlockHistorySize,
		ArchiveEnabled:          DefaultArchiveEnabled,
		Providers:               make(map[string]*provider),
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
	// Get latest network block for comparison
	latestNetworkBlock := n.getLatestHealthyBlock()

	for _, provider := range n.Providers {
		// Get latest block and initial health status
		var healthStatus HealthStatus = Healthy
		blockNum, initialHealth, err := n.getLatestBlockNumber(provider.HttpUrl, provider.Headers, provider.AuthClient())
		if err != nil {
			n.logProviderWarning(
				"Error getting latest block number for provider",
				provider,
				zap.Error(err),
			)

			// Handle error cases with grace period logic
			healthStatus := n.handleErrorWithGracePeriod(provider, initialHealth, blockNum)
			if healthStatus == Unhealthy {
				continue // Skip further checks for confirmed unhealthy providers
			}

		}
		// Evaluate final health status
		newStatus := n.evaluateProviderHealth(provider, blockNum, healthStatus, latestNetworkBlock)
		provider.healthStatus = newStatus

		// Update metrics and history
		provider.AddBlockEntry(blockNum, newStatus, n.BlockHistorySize)
		n.sendLatestBlockMetric(provider.host, 0, newStatus.String(), blockNum)
	}
}

// handleErrorWithGracePeriod implements the grace period logic for unhealthy providers
func (n *network) handleErrorWithGracePeriod(provider *provider, healthStatus HealthStatus, blockNum int64) HealthStatus {
	if healthStatus != Unhealthy {
		// For Warning status, reset counter and continue with checks
		provider.consecutiveUnhealthyChecks = 0
		return healthStatus
	}

	// Handle Unhealthy status with grace period
	provider.consecutiveUnhealthyChecks++
	if provider.consecutiveUnhealthyChecks < n.HCThreshold {
		// First unhealthy response - give grace period by converting to warning
		return Warning
	}

	// Provider has exceeded grace period - mark as unhealthy
	provider.healthStatus = Unhealthy
	provider.AddBlockEntry(blockNum, Unhealthy, n.BlockHistorySize)
	n.sendLatestBlockMetric(provider.host, 0, provider.healthStatus.String(), blockNum)
	return Unhealthy
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
func (n *network) evaluateProviderHealth(provider *provider, currentBlock int64, healthStatus HealthStatus, latestNetworkBlock int64) HealthStatus {
	// Track the worst status we find
	worstStatus := healthStatus

	// if provider has no block history, set it to healthy
	if len(provider.BlockHistory()) == 0 {
		return Unhealthy
	}

	// Check block lag and block jump
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

	// chainId check health check
	chainId, err := n.getChainID(provider.HttpUrl, provider.Headers, provider.AuthClient())
	if err != nil {
		n.logProviderWarning("Error getting chain ID", provider, zap.Error(err))
		return Unhealthy
	}

	if !n.verifyChainID(chainId) {
		n.logProviderWarning("Provider has incorrect chain ID", provider)
		return Unhealthy
	}

	// Archive Health Check
	// if the provider name doesn't contains "bitcoin or solana and archive is enabled, return unhealthy
	// then check if the provider can return back block data from half of its block height
	if n.ArchiveEnabled && !strings.Contains(n.Name, "bitcoin") && !strings.Contains(n.Name, "solana") {
		// check if the provider can return back block data from half of its block height
		currentBlock := provider.getLatestHealthyBlockEntry()
		if currentBlock == nil {
			// if the provider has no healthy block history, return unhealthy
			return Unhealthy
		}
		// get a quarter of the block height
		quarterBlockHeight := currentBlock.blockNumber / 4

		// convert quarterBlockHeight to hex string
		quarterBlockHeightHex := fmt.Sprintf("0x%x", quarterBlockHeight)

		// call the network method
		err := n.testArchiveMode(provider.HttpUrl, provider.Headers, provider.AuthClient(), quarterBlockHeightHex)
		if err != nil {
			n.logProviderWarning("Error testing archive mode", provider, zap.Error(err))
			return Unhealthy
		}
	}

	return worstStatus
}

// verifyChainID checks if provider is serving correct chain
func (n *network) verifyChainID(chainId string) bool {
	return chainId == n.ChainId
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

// getLatestHealthyBlock returns the highest block number among healthy providers
// Falls back to warning providers if no healthy providers are available
func (n *network) getLatestHealthyBlock() int64 {
	var latestBlockFromHealthy int64
	var latestBlockFromWarning int64
	var latestBlockFromUnhealthy int64

	// Single pass through providers to track latest blocks by status
	for _, provider := range n.Providers {
		history := provider.BlockHistory()
		if len(history) == 0 {
			continue
		}

		lastBlock := history[len(history)-1].blockNumber
		switch provider.healthStatus {
		case Healthy:
			if lastBlock > latestBlockFromHealthy {
				latestBlockFromHealthy = lastBlock
			}
		case Warning:
			if lastBlock > latestBlockFromWarning {
				latestBlockFromWarning = lastBlock
			}
		case Unhealthy:
			if lastBlock > latestBlockFromUnhealthy {
				latestBlockFromUnhealthy = lastBlock
			}
		}
	}

	// Return highest block number, prioritizing by health status
	if latestBlockFromHealthy > 0 {
		return latestBlockFromHealthy
	}
	return latestBlockFromWarning
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

func (n *network) getLatestBlockNumber(httpUrl string, headers map[string]string, ac auth.IAuthClient) (int64, HealthStatus, error) {
	var lastErr error
	var lastHealthStatus HealthStatus = Unhealthy

	// Layer 1: Handle attempts
	for attempt := 0; attempt < n.RequestAttemptCount; attempt++ {
		resBytes, statusCode, err := n.tryBlockNumberRequestWithTimeout(httpUrl, headers, ac)
		if err != nil {
			lastErr = err
			continue
		}

		blockNumber, health, err := n.processBlockNumberResponse(resBytes, statusCode)
		if err != nil {
			lastErr = err
			lastHealthStatus = health
			continue
		}

		return blockNumber, health, nil
	}

	return 0, lastHealthStatus, errors.Wrap(lastErr, fmt.Sprintf("Failed after %d attempts", n.RequestAttemptCount))
}

// Layer 2: Handle timeout
func (n *network) tryBlockNumberRequestWithTimeout(httpUrl string, headers map[string]string, ac auth.IAuthClient) ([]byte, *int, error) {
	payload := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method": "%s","params":[],"id":1}`, n.HCMethod))

	// Try the request up to 2 times if it times out
	for retryCount := 0; retryCount < 2; retryCount++ {
		// Create a channel for the response
		type result struct {
			resBytes   []byte
			statusCode *int
			err        error
		}
		resChan := make(chan result, 1)

		// Make the request in a goroutine
		go func(payload []byte) {
			resBytes, statusCode, err := n.HttpClient.Post(httpUrl, headers, payload, ac)
			resChan <- result{resBytes, statusCode, err}
		}(payload)

		// Wait for response or timeout
		select {
		case res := <-resChan:
			return res.resBytes, res.statusCode, res.err
		case <-time.After(time.Duration(BlockNumberTimeoutSeconds) * time.Second):
			if retryCount == 1 {
				return nil, nil, errors.New("request is taking too long after retry")
			}
			continue
		}
	}

	return nil, nil, errors.New("request timed out")
}

// Layer 3: Process response
func (n *network) processBlockNumberResponse(resBytes []byte, statusCode *int) (int64, HealthStatus, error) {
	// Evaluate health status based on status code
	if *statusCode >= 400 {
		// If status code is 429, set health status to Warning, otherwise return as unhealthy
		if *statusCode == 429 {
			return 0, Warning, fmt.Errorf("rate limit error (status code: %d)", *statusCode)
		}
		return 0, Unhealthy, fmt.Errorf("error status code: %d", *statusCode)
	}

	var respObject map[string]interface{}
	err := json.Unmarshal(resBytes, &respObject)
	if err != nil {
		return 0, Unhealthy, errors.Wrap(err, "Error unmarshalling response")
	}

	var blockNumber int64

	switch result := respObject["result"].(type) {
	case string:
		if result == "" || result[:2] != "0x" {
			return 0, Unhealthy, errors.New("Invalid block number")
		}

		blockNumber, err = strconv.ParseInt(result[2:], 16, 64)
		if err != nil {
			return 0, Unhealthy, errors.Wrap(err, "Error converting block number")
		}
	case float64:
		blockNumber = int64(result)
	default:
		return 0, Unhealthy, errors.New("unsupported block number type")
	}

	return blockNumber, Healthy, nil
}

func (n *network) getChainID(httpUrl string, headers map[string]string, ac auth.IAuthClient) (string, error) {
	payload := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method": "%s","params":[],"id":1}`, n.ChainIdMethod))

	// Send the POST request
	resBytes, statusCode, err := n.HttpClient.Post(httpUrl, headers, []byte(payload), ac)
	if err != nil {
		return "", errors.Wrap(err, "Error sending POST request")
	}

	if *statusCode != http.StatusOK {
		return "", errors.New("Error getting chain ID from response")
	}

	// response struct
	var respObject map[string]interface{}

	// Unmarshal the response
	err = json.Unmarshal(resBytes, &respObject)
	if err != nil {
		return "", errors.Wrap(err, "Error unmarshalling response")
	}

	if _, ok := respObject["result"]; !ok {
		return "", errors.New("Error getting chain ID from response")
	}

	var chainID string
	var ok bool

	// Bitcoin returns back chain ID nested in an object.
	if strings.Contains(n.Name, "bitcoin") {
		chainID, ok = respObject["result"].(map[string]interface{})["chain"].(string)
		if !ok {
			return "", errors.New("Error getting chain ID from response")
		}
	} else {
		chainID, ok = respObject["result"].(string)
		if !ok {
			return "", errors.New("Error getting chain ID from response")
		}
	}

	return chainID, nil
}

func (n *network) testArchiveMode(httpUrl string, headers map[string]string, ac auth.IAuthClient, quarterBlockHeightHex string) error {
	payload := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method": "%s","id":1,"params":[{"input":"0x436000526004601cf3"},"%s"]}`, n.CallContractMethod, quarterBlockHeightHex))

	// Send the POST request
	resBytes, statusCode, err := n.HttpClient.Post(httpUrl, headers, []byte(payload), ac)
	if err != nil {
		return errors.Wrap(err, "Error sending POST request")
	}

	if *statusCode == http.StatusServiceUnavailable || *statusCode == StatusOriginUnreachable {
		return errors.New("Network Unavailable")
	}

	// response struct
	var respObject map[string]interface{}

	// Unmarshal the response
	err = json.Unmarshal(resBytes, &respObject)
	if err != nil {
		return errors.Wrap(err, "Error unmarshalling response")
	}

	// if the response contains an error, return an error
	if _, ok := respObject["error"]; ok {
		return errors.New("network doesn't support archive mode")
	}

	return nil
}

func (n *network) close() {
	close(n.quit)
}
