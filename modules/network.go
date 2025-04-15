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
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type network struct {
	Name             string
	quit             chan struct{}
	HttpClient       din_http.IHTTPClient
	PrometheusClient prom.IPrometheusClient
	logger           *logger.LoggerClient
	machineID        string
	Environment      utils.Environment
	// internal health check values
	HCThreshold      int
	BlockHistorySize int

	// MethodFilter can be used to route requests based on the method. It implements
	// the ProviderFilter interface, but for now is the only implementation.
	MethodFilter *methodFilter

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
func NewNetwork(name string, environment utils.Environment) *network {
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
		Environment:             environment,
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
			n.logProviderWarning("Error getting latest block number for provider", provider,
				zap.Int64("block_number", blockNum),
				zap.String("provider", provider.host),
				zap.String("health_status", initialHealth.String()),
				zap.Error(err))
			// Handle error cases with grace period logic
			healthStatus := n.handleErrorWithGracePeriod(provider, initialHealth, blockNum)

			if healthStatus == Unhealthy {
				// Add the block entry and send metric
				provider.AddBlockEntry(blockNum, Unhealthy, n.BlockHistorySize)
				n.sendHealthCheckMetric(provider.host, Unhealthy.String(), blockNum, string(n.Environment))

				continue // Skip further checks for confirmed unhealthy providers
			}
		}
		// Evaluate final health status
		newStatus := n.evaluateProviderHealth(provider, blockNum, healthStatus, latestNetworkBlock)

		// Update metrics and history
		provider.AddBlockEntry(blockNum, newStatus, n.BlockHistorySize)
		n.sendHealthCheckMetric(provider.host, newStatus.String(), blockNum, string(n.Environment))
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
	n.logProviderWarning("Provider has exceeded grace period, marking as unhealthy", provider,
		zap.Int64("block_number", blockNum),
		zap.Int("consecutive_unhealthy_checks", provider.consecutiveUnhealthyChecks),
		zap.Int("healthcheck_threshold", n.HCThreshold),
		zap.String("health_status", Unhealthy.String()))
	return Unhealthy
}

// logProviderWarning logs a warning message with standard provider context
func (n *network) logProviderWarning(msg string, provider *provider, fields ...zap.Field) {
	baseFields := []zap.Field{
		zap.String("provider", provider.host),
		zap.String("network", n.Name),
	}

	n.logger.Warn(msg, append(baseFields, fields...)...)
}

// evaluateProviderHealth performs both levels of health checks
func (n *network) evaluateProviderHealth(provider *provider, currentBlock int64, healthStatus HealthStatus, latestNetworkBlock int64) HealthStatus {
	// Track the worst status we find
	worstStatus := healthStatus

	// if provider has no block history, set it to unhealthy
	if len(provider.BlockHistory()) == 0 {
		n.logProviderWarning("Provider has no block history, marking as unhealthy", provider,
			zap.Int64("current_block", currentBlock),
			zap.Int64("latest_network_block", latestNetworkBlock),
			zap.String("health_status", Unhealthy.String()))
		return Unhealthy
	}

	// Check for block lag
	var isLagged bool
	var blockLag int64

	if latestNetworkBlock > 0 {
		blockLag = int64(latestNetworkBlock) - currentBlock
		// If block lag is greater than limit, mark as warning and set isLagged flag
		if blockLag > n.BlockLagLimit {
			isLagged = true
			n.logProviderWarning("Provider is lagging behind network", provider,
				zap.Int64("block_lag_limit", n.BlockLagLimit),
				zap.Int64("block_lag", blockLag),
				zap.Int64("provider_block", currentBlock),
				zap.Int64("network_block", latestNetworkBlock),
				zap.String("health_status", Warning.String()))
			if Warning > worstStatus {
				worstStatus = Warning
			}
		}

		// Check if block is too far ahead (block jump)
		blockJump := currentBlock - int64(latestNetworkBlock)
		if blockJump > n.BlockJumpLimit {
			n.logProviderWarning("Provider is too far ahead of network", provider,
				zap.Int64("block_jump_limit", n.BlockJumpLimit),
				zap.Int64("block_jump", blockJump),
				zap.Int64("provider_block", currentBlock),
				zap.Int64("network_block", latestNetworkBlock),
				zap.String("health_status", Unhealthy.String()))
			return Unhealthy
		}
	}

	if isLagged {
		if Warning > worstStatus {
			worstStatus = Warning
		}

		isStalled := n.isStalled(provider)
		if isStalled {
			// Provider is both stalled and lagged - more serious issue
			n.logProviderWarning("Provider is stalled and lagged", provider,
				zap.Int64("block_lag_limit", n.BlockLagLimit),
				zap.Int64("block_lag", blockLag),
				zap.Int64("provider_block", currentBlock),
				zap.Int64("network_block", latestNetworkBlock),
				zap.String("health_status", Unhealthy.String()))
			return Unhealthy
		}
	} else if n.isStalled(provider) && !n.allProvidersStalled() {
		// Edge case: Provider is stalled but not yet lagged, while others are making progress
		n.logProviderWarning("Provider is stalled while others are progressing", provider,
			zap.Int64("provider_block", currentBlock),
			zap.Int64("network_block", latestNetworkBlock),
			zap.String("health_status", Warning.String()))
		return Warning
	}

	// chainId check health check
	chainId, err := n.getChainID(provider.HttpUrl, provider.Headers, provider.AuthClient())
	if err != nil {
		n.logProviderWarning("Error getting chain ID", provider,
			zap.String("chain_id", chainId),
			zap.String("expected_chain_id", n.ChainId),
			zap.String("health_status", Unhealthy.String()))
		return Unhealthy
	}

	if !n.verifyChainID(chainId) {
		n.logProviderWarning("Provider has incorrect chain ID", provider,
			zap.String("chain_id", chainId),
			zap.String("expected_chain_id", n.ChainId),
			zap.String("health_status", Unhealthy.String()))
		return Unhealthy
	}

	// Archive Health Check
	// if the provider name doesn't contains "bitcoin or solana and archive is enabled, return unhealthy
	// then check if the provider can return back block data from half of its block height
	if n.ArchiveEnabled && !strings.Contains(n.Name, "bitcoin") && !strings.Contains(n.Name, "solana") && len(provider.BlockHistory()) > 1 {

		// get a quarter of the block height
		quarterBlockHeight := currentBlock / 4

		var quarterBlockHeightString string
		if strings.Contains(n.Name, "starknet") {
			// Starknet uses decimal for block height, no need to convert to hex
			quarterBlockHeightString = strconv.FormatInt(quarterBlockHeight, 10)
		} else {
			// convert quarterBlockHeight to hex string
			quarterBlockHeightString = fmt.Sprintf("%#x", quarterBlockHeight)
		}

		// call the network method
		err := n.archiveModeCheck(provider.HttpUrl, provider.Headers, provider.AuthClient(), quarterBlockHeightString)
		if err != nil {
			n.logProviderWarning("Error testing archive mode", provider,
				zap.Int64("quarter_block_height", quarterBlockHeight),
				zap.String("quarter_block_height_hex", quarterBlockHeightString),
				zap.Error(err),
				zap.String("health_status", Unhealthy.String()))
			return Warning
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

		latestBlock := provider.getLatestBlockEntry()
		if latestBlock == nil {
			continue
		}
		switch latestBlock.healthStatus {
		case Healthy:
			if latestBlock.blockNumber > latestBlockFromHealthy {
				latestBlockFromHealthy = latestBlock.blockNumber
			}
		case Warning:
			if latestBlock.blockNumber > latestBlockFromWarning {
				latestBlockFromWarning = latestBlock.blockNumber
			}
		case Unhealthy:
			if latestBlock.blockNumber > latestBlockFromUnhealthy {
				latestBlockFromUnhealthy = latestBlock.blockNumber
			}
		}
	}

	// Return highest block number, prioritizing by health status
	if latestBlockFromHealthy > 0 {
		return latestBlockFromHealthy
	}
	return latestBlockFromWarning
}

func (n *network) sendHealthCheckMetric(providerName string, healthStatus string, blockNumber int64, environment string) {
	n.PrometheusClient.HandleHealthCheckMetric(&prom.PromHealthCheckMetricData{
		Network:      n.Name,
		Provider:     providerName,
		HealthStatus: healthStatus,
		BlockNumber:  blockNumber,
		Environment:  environment,
	})
}

func (n *network) getLatestBlockNumber(httpUrl string, headers map[string]string, ac auth.IAuthClient) (int64, HealthStatus, error) {
	var lastErr error
	var lastHealthStatus HealthStatus = Unhealthy

	// Layer 1: Handle attempts
	for attempt := 0; attempt < n.RequestAttemptCount; attempt++ {
		payload := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method": "%s","params":[],"id":1}`, n.HCMethod))
		resBytes, statusCode, err := n.HttpClient.Post(httpUrl, headers, payload, ac)
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
	var lastErr error

	// Layer 1: Handle attempts
	for attempt := 0; attempt < n.RequestAttemptCount; attempt++ {
		payload := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method": "%s","params":[],"id":1}`, n.ChainIdMethod))

		// Send the POST request
		resBytes, statusCode, err := n.HttpClient.Post(httpUrl, headers, []byte(payload), ac)
		if err != nil {
			lastErr = errors.Wrap(err, "Error sending POST request")
			continue
		}

		if *statusCode != http.StatusOK {
			lastErr = errors.New("Error getting chain ID from response")
			continue
		}

		// response struct
		var respObject map[string]interface{}

		// Unmarshal the response
		err = json.Unmarshal(resBytes, &respObject)
		if err != nil {
			lastErr = errors.Wrap(err, "Error unmarshalling response")
			continue
		}

		if _, ok := respObject["result"]; !ok {
			lastErr = errors.New("Error getting chain ID from response")
			continue
		}

		var chainReference string
		var ok bool
		namespace := EVMNamespace

		// Map network names to their namespaces
		namespaceMap := map[string]string{
			"bitcoin":  BitcoinNamespace,
			"solana":   SolanaNamespace,
			"starknet": StarknetNamespace,
		}

		// Check if network name contains any of the special cases
		for key, ns := range namespaceMap {
			if strings.Contains(n.Name, key) {
				namespace = ns
				break
			}
		}

		// For Bitcoin networks, the chain ID is in a nested "chain" field in the result object
		// For all other networks, the chain ID is directly in the result field as a string
		if strings.Contains(n.Name, "bitcoin") {
			result, ok := respObject["result"].(map[string]interface{})
			if !ok || result["chain"] == nil {
				lastErr = errors.New("Error getting chain ID from response")
				continue
			}
			chainReference = result["chain"].(string)
		} else {
			chainReference, ok = respObject["result"].(string)
			if !ok {
				lastErr = errors.New("Error getting chain ID from response")
				continue
			}
		}

		fullChainId := namespace + ":" + chainReference

		// Success case - return the chain ID
		return fullChainId, nil
	}

	return "", errors.Wrap(lastErr, fmt.Sprintf("Failed after %d attempts", n.RequestAttemptCount))
}

func (n *network) archiveModeCheck(httpUrl string, headers map[string]string, ac auth.IAuthClient, quarterBlockHeight string) error {
	var lastErr error

	// Layer 1: Handle attempts
	for attempt := 0; attempt < n.RequestAttemptCount; attempt++ {
		var payload []byte
		if strings.Contains(n.Name, "starknet") {
			// For Starknet, quarterBlockHeight is already in decimal format
			blockNum, err := strconv.ParseInt(quarterBlockHeight, 10, 64)
			if err != nil {
				lastErr = errors.Wrap(err, "Failed to parse quarter block height")
				continue
			}

			// Starknet uses a different method for archive mode check
			payload = []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method": "%s","id":1,"params":[{"block_number":%d}]}`, StarknetArchiveMethod, blockNum))
		} else {
			payload = []byte(fmt.Sprintf(`{"jsonrpc":"2.0","method": "%s","id":1,"params":[{"input":"0x436000526004601cf3"},"%s"]}`, n.CallContractMethod, quarterBlockHeight))
		}

		// Send the POST request
		resBytes, statusCode, err := n.HttpClient.Post(httpUrl, headers, []byte(payload), ac)
		if err != nil {
			lastErr = errors.Wrap(err, "Error sending POST request")
			continue
		}

		if *statusCode == http.StatusServiceUnavailable || *statusCode == StatusOriginUnreachable {
			lastErr = errors.New("Network Unavailable")
			continue
		}

		// response struct
		var respObject map[string]interface{}

		// Unmarshal the response
		err = json.Unmarshal(resBytes, &respObject)
		if err != nil {
			lastErr = errors.Wrap(err, "Error unmarshalling response")
			continue
		}

		// if the response contains an error, return an error
		if _, ok := respObject["error"]; ok {
			lastErr = errors.New("network doesn't support archive mode")
			continue
		}

		if strings.Contains(n.Name, "starknet") {
			// Starknet uses a different response structure
			result, ok := respObject["result"].(map[string]interface{})
			if !ok {
				lastErr = errors.New("Error getting archive mode check from response: missing or invalid result object")
				continue
			}

			// Check for block_hash field
			blockHash, ok := result["block_hash"].(string)
			if !ok || blockHash == "" {
				lastErr = errors.New("Error getting archive mode check from response: missing or invalid block_hash")
				continue
			}

			// Success case - block_hash exists and is non-empty
			return nil
		}

		// Success case - return nil
		return nil
	}

	return errors.Wrap(lastErr, fmt.Sprintf("Failed after %d attempts", n.RequestAttemptCount))
}

func (n *network) close() {
	close(n.quit)
}
