package modules

import (
	"container/list"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type network struct {
	Name             string
	HandlerType      HandlerType `json:"handler"` // Network handler type for handler registry
	quit             chan struct{}
	HttpClient       din_http.IHTTPClient
	PrometheusClient prom.IPrometheusClient
	CaddyPort        string
	logger           *logger.LoggerClient
	machineID        string
	Environment      utils.Environment

	// NEW: Handler reference for network-specific operations
	handler networklib.NetworkHandler

	// internal health check values
	HCThreshold              int
	HCTimeout                int
	HCEndpoint               string `json:"healthcheck_endpoint,omitempty"` // REST endpoint for health checks
	ProviderBlockHistorySize int
	NetworkBlockHistorySize  int
	blockHistory             *list.List
	blockHistoryMu           sync.RWMutex

	// MethodFilter can be used to route requests based on the method. It implements
	// the ProviderFilter interface, but for now is the only implementation.
	MethodFilter *methodFilter

	// Registry configuration values
	Providers map[string]*provider `json:"providers"`
	Methods   []*string            `json:"methods"`
	ChainId   string               `json:"chain_id"`

	HCInterval              int   `json:"healthcheck_interval_seconds"`
	BlockLagLimit           int64 `json:"healthcheck_blocklag_limit"`
	BlockJumpLimit          int64 `json:"healthcheck_blockjump_limit"`
	MaxRequestPayloadSizeKB int64 `json:"max_request_payload_size_kb"`
	RequestAttemptCount     int   `json:"request_attempt_count"`
	ArchiveEnabled          bool  `json:"archive_enabled"`
}

// NewNetwork creates a new network with the given name and handler type
// Only put values in the struct definition that are constant
// Don't kick off any Background processes here
func NewNetwork(name string, handlerType HandlerType, environment utils.Environment, caddyPort string) (*network, error) {
	n := &network{
		Name: name,
		HandlerType: handlerType, // Used for handler selection
		// Default health check values, to be overridden if specified in the Caddyfile
		HCThreshold:              DefaultHCThreshold,
		HCTimeout:                DefaultHCTimeout,
		HCInterval:               DefaultHCInterval,
		BlockLagLimit:            DefaultBlockLagLimit,
		BlockJumpLimit:           DefaultBlockJumpLimit,
		MaxRequestPayloadSizeKB:  DefaultMaxRequestPayloadSizeKB,
		RequestAttemptCount:      DefaultRequestAttemptCount,
		ProviderBlockHistorySize: DefaultProviderBlockHistorySize,
		NetworkBlockHistorySize:  DefaultNetworkBlockHistorySize,
		blockHistory:             list.New(),
		ArchiveEnabled:           DefaultArchiveEnabled,
		Environment:              environment,
		Providers:                make(map[string]*provider),
		CaddyPort:                caddyPort,
	}

	// Note: Handler initialization is deferred to avoid duplicate initialization.
	// The handler will be set later when:
	// 1. The "type" field is parsed in Caddyfile configuration
	// 2. The network defaults to EVM if no type is specified
	// This ensures the handler is only initialized once with complete configuration
	// including ChainID and other network-specific settings.

	return n, nil
}

// SetHandler sets the network's handler. This is used when:
// 1. Creating a network with an explicit type
// 2. Setting type in Caddyfile configuration
// 3. Defaulting to EVM when no type is specified
func (n *network) SetHandler(handler networklib.NetworkHandler) error {
	if handler == nil {
		return fmt.Errorf("cannot set nil handler")
	}

	// Only set if different to avoid redundant updates
	if n.handler == handler {
		return nil
	}

	n.handler = handler

	// Only log if logger is available (it's initialized during Provision)
	if n.logger != nil {
		n.logger.Debug("Network handler set",
			zap.String("network", n.Name),
			zap.String("handler_type", handler.GetType()))
	}

	return nil
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
	// Add handler status logging at the start of health check

	// Self loopback health check (run asynchronously)
	go n.LoopbackHealthCheck()

	// Get latest network block for comparison
	latestNetworkBlock := n.getLatestHealthyBlock()

	for _, provider := range n.Providers {

		// Get latest block and initial health status
		var healthStatus HealthStatus = Healthy
		latestBlockResult, err := n.getLatestBlockNumber(provider.HttpUrl, provider.Headers, provider.AuthClient(), provider.host)
		if err != nil {
			n.logProviderWarning("Health check failed after all attempts for provider", provider,
				zap.Int64("block_number", latestBlockResult.blockNumber),
				zap.String("provider", provider.host),
				zap.Int("response_status", latestBlockResult.responseStatus),
				zap.String("health_status", latestBlockResult.healthStatus.String()),
				zap.Int("total_attempts", n.RequestAttemptCount),
				zap.Error(err))
			// Handle error cases with grace period logic
			healthStatus := n.handleErrorWithGracePeriod(provider, latestBlockResult.healthStatus, latestBlockResult.blockNumber)

			if healthStatus == Unhealthy {
				// Add the block entry and send metric
				provider.AddBlockEntry(latestBlockResult.blockNumber, Unhealthy, n.ProviderBlockHistorySize)
				n.sendHealthCheckMetric(provider.host, latestBlockResult.responseStatus, latestBlockResult.healthStatus.String(), latestBlockResult.blockNumber, string(n.Environment))

				continue // Skip further checks for confirmed unhealthy providers
			}
		}
		// Evaluate final health status
		newStatus := n.evaluateProviderHealth(provider, latestBlockResult.blockNumber, healthStatus, latestNetworkBlock)

		// Update metrics and history
		provider.AddBlockEntry(latestBlockResult.blockNumber, newStatus, n.ProviderBlockHistorySize)
		n.sendHealthCheckMetric(provider.host, latestBlockResult.responseStatus, newStatus.String(), latestBlockResult.blockNumber, string(n.Environment))
	}
}

// LoopbackHealthCheck performs a self loopback health check and logs/metrics the result.
func (n *network) LoopbackHealthCheck() {
	startTime := time.Now()
	selfResult, selfErr := n.checkSelfLoopbackHealth()
	duration := time.Since(startTime)

	// Prepare metric data regardless of error, as we want to capture response status and duration
	metricData := &prom.PromNetworkHealthCheckMetricData{
		Network:        n.Name, // Use n.Name directly for the network label
		ResponseStatus: 0,      // Default to 0 if selfResult is nil
		Duration:       duration,
		Environment:    string(n.Environment),
	}

	if selfResult != nil {
		metricData.ResponseStatus = selfResult.responseStatus
	}

	n.PrometheusClient.HandleNetworkHealthCheckMetric(metricData)

	if selfErr != nil {
		n.logger.Warn("Self loopback health check failed",
			zap.Error(selfErr),
			zap.String("network", n.Name),
			zap.Int("response_status", metricData.ResponseStatus),
			zap.Duration("duration", duration),
		)
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

	// if provider has no block history and current health check failed, set it to unhealthy
	// Allow providers with no history to be healthy if the current check succeeded
	if len(provider.BlockHistory()) == 0 && healthStatus != Healthy {
		n.logProviderWarning("Provider has no block history and current check failed, marking as unhealthy", provider,
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
			if Warning > worstStatus {
				worstStatus = Warning
			}
			n.logProviderWarning("Provider is lagging behind network", provider,
				zap.Int64("block_lag_limit", n.BlockLagLimit),
				zap.Int64("block_lag", blockLag),
				zap.Int64("provider_block", currentBlock),
				zap.Int64("network_block", latestNetworkBlock),
				zap.String("health_status", Warning.String()))
		}

		// Check if block is too far ahead (block jump)
		blockJump := currentBlock - int64(latestNetworkBlock)
		if blockJump > n.BlockJumpLimit {
			// Only mark as unhealthy if there's at least one other healthy provider
			// This prevents false-negative unhealthy status during network recovery
			// scenarios where a provider might be ahead of others because it's
			// recovering faster from a network-wide issue
			if n.hasOtherHealthyProviders(provider) {
				n.logProviderWarning("Provider is too far ahead of network", provider,
					zap.Int64("block_jump_limit", n.BlockJumpLimit),
					zap.Int64("block_jump", blockJump),
					zap.Int64("provider_block", currentBlock),
					zap.Int64("network_block", latestNetworkBlock),
					zap.String("health_status", Unhealthy.String()))
				return Unhealthy
			} else {
				// If there are no other healthy providers, assume this one is correct
				// and healthy because it might be the first to recover from a network outage
				n.logProviderWarning("Provider is far ahead but keeping as healthy (no other healthy providers)", provider,
					zap.Int64("block_jump_limit", n.BlockJumpLimit),
					zap.Int64("block_jump", blockJump),
					zap.Int64("provider_block", currentBlock),
					zap.Int64("network_block", latestNetworkBlock),
					zap.String("health_status", Healthy.String()))
				return Healthy
			}
		}
	}

	isStalled := n.isStalled(provider)

	if isLagged {
		if Warning > worstStatus {
			worstStatus = Warning
		}

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
	} else if isStalled && !n.allProvidersStalled() {
		n.logProviderWarning("Provider is stalled while others are progressing", provider,
			zap.Int64("provider_block", currentBlock),
			zap.Int64("network_block", latestNetworkBlock),
			zap.String("health_status", Warning.String()))
		return Warning
	}

	// chainId check health check
	// Use handler's GetChainID method for all network types
	if n.handler != nil {
		chainId, err := n.handler.GetChainID(provider.HttpUrl, provider.Headers, n.HttpClient, provider.AuthClient(), n.RequestAttemptCount)
		if err != nil {
			n.logProviderWarning("Error getting chain ID", provider,
				zap.String("expected_chain_id", n.ChainId),
				zap.String("health_status", Unhealthy.String()),
				zap.Error(err))
			return Unhealthy
		}

		if err := n.handler.ValidateChainID(chainId); err != nil {
			n.logProviderWarning("Provider has incorrect chain ID", provider,
				zap.String("chain_id", chainId),
				zap.String("expected_chain_id", n.ChainId),
				zap.String("validation_error", err.Error()),
				zap.String("health_status", Unhealthy.String()))
			return Unhealthy
		}
	}

	// Archive Health Check - Use handler method directly
	if err := n.performArchiveCheck(provider, currentBlock); err != nil {
		n.logProviderWarning("Archive mode check failed", provider,
			zap.Int64("current_block", currentBlock),
			zap.Error(err),
			zap.String("health_status", Warning.String()))
		return Warning
	}

	return worstStatus
}

// performArchiveCheck performs archive mode check for a provider if archive mode is enabled
func (n *network) performArchiveCheck(provider *provider, currentBlock int64) error {
	// Check if archive mode is enabled and supported
	if !n.ArchiveEnabled {
		return nil // Archive mode disabled, skip check
	}

	// Check if handler is available
	if n.handler == nil {
		return nil // No handler available, skip check
	}

	// Check if handler supports archive mode
	if !n.handler.SupportsArchiveMode() {
		return nil // Handler doesn't support archive mode, skip check
	}

	// Check if provider has enough block history
	if len(provider.BlockHistory()) <= 1 {
		return nil // Not enough block history, skip check
	}

	quarterBlockHeight := currentBlock / 4

	// Use handler method to format block height directly
	quarterBlockHeightString := n.handler.FormatBlockHeight(quarterBlockHeight)

	return n.handler.PerformArchiveCheck(provider.HttpUrl, provider.Headers, n.HttpClient, provider.AuthClient(), n.RequestAttemptCount, quarterBlockHeightString)
}

func (n *network) close() {
	close(n.quit)
}

// isStalled checks if provider's block numbers haven't changed
func (n *network) isStalled(provider *provider) bool {
	history := provider.BlockHistory()
	// if history is less than the block history size, return false
	if len(history) < n.ProviderBlockHistorySize {
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

func (n *network) sendHealthCheckMetric(providerName string, responseStatus int, healthStatus string, blockNumber int64, environment string) {
	n.PrometheusClient.HandleHealthCheckMetric(&prom.PromHealthCheckMetricData{
		Network:        n.Name,
		Provider:       providerName,
		ResponseStatus: responseStatus,
		HealthStatus:   healthStatus,
		BlockNumber:    blockNumber,
		Environment:    environment,
	})
}

type getLatestBlockNumberResult struct {
	blockNumber    int64
	healthStatus   HealthStatus
	responseStatus int
}

func (n *network) getLatestBlockNumber(httpUrl string, headers map[string]string, ac auth.IAuthClient, providerHost string) (*getLatestBlockNumberResult, error) {
	// Ensure handler is available
	if n.handler == nil {
		return &getLatestBlockNumberResult{
			blockNumber:    0,
			healthStatus:   Unhealthy,
			responseStatus: 0,
		}, fmt.Errorf("no handler available for network %s", n.Name)
	}

	n.logger.Debug("Using handler to get latest block number",
		zap.String("network", n.Name),
		zap.String("provider", providerHost),
		zap.String("handler_type", n.handler.GetType()))

	// Delegate to handler's GetLatestBlockNumber method
	result, err := n.handler.GetLatestBlockNumber(httpUrl, headers, n.HttpClient, ac, n.RequestAttemptCount)
	if err != nil {
		n.logger.Debug("Handler GetLatestBlockNumber failed",
			zap.Error(err),
			zap.String("network", n.Name),
			zap.String("provider", providerHost))

		// If there's an error, result might be nil or partially populated
		if result != nil {
			return &getLatestBlockNumberResult{
				blockNumber:    result.BlockNumber,
				healthStatus:   HealthStatus(result.HealthStatus), // Convert networklib.HealthStatus to modules.HealthStatus
				responseStatus: result.ResponseStatus,
			}, err
		} else {
			// Return a default unhealthy result when result is nil
			return &getLatestBlockNumberResult{
				blockNumber:    0,
				healthStatus:   Unhealthy,
				responseStatus: 0,
			}, err
		}
	}

	// Success!
	n.logger.Debug("Handler GetLatestBlockNumber succeeded",
		zap.Int64("block_number", result.BlockNumber),
		zap.String("health_status", result.HealthStatus.String()),
		zap.String("network", n.Name),
		zap.String("provider", providerHost))

	return &getLatestBlockNumberResult{
		blockNumber:    result.BlockNumber,
		healthStatus:   HealthStatus(result.HealthStatus), // Convert networklib.HealthStatus to modules.HealthStatus
		responseStatus: result.ResponseStatus,
	}, nil
}

// Layer 3: Process response
func (n *network) processBlockNumberResponse(resBytes []byte, statusCode *int) (int64, HealthStatus, error) {
	// Validate input
	if statusCode == nil {
		return 0, Unhealthy, errors.New("received nil statusCode in processBlockNumberResponse")
	}

	// Ensure handler is available
	if n.handler == nil {
		return 0, Unhealthy, fmt.Errorf("no handler available for network %s", n.Name)
	}

	// Delegate to handler for network-specific parsing
	blockNumber, err := n.handler.ParseBlockNumberResponse(resBytes, *statusCode)
	if err != nil {
		// Determine health status based on error type
		if *statusCode == 429 {
			return 0, Warning, err
		}
		return 0, Unhealthy, err
	}

	return blockNumber, Healthy, nil
}

// getHealthCheckMethod returns the health check method using handler

// hasOtherHealthyProviders checks if there are any other healthy providers in the pool
// besides the one being evaluated. This is used during block jump detection to avoid
// marking a provider as unhealthy when it might actually be correct (e.g., during network
// recovery scenarios where a single provider might recover faster than others).
//
// The function returns:
//   - true if at least one other provider (not the one being evaluated) has a healthy status
//   - false if all other providers are either unhealthy or in warning state
//
// This helps prevent situations where all providers might be marked unhealthy during
// network-wide issues when one provider recovers faster than others.
func (n *network) hasOtherHealthyProviders(provider *provider) bool {
	for _, p := range n.Providers {
		if p != provider {
			entry := p.getLatestHealthyBlockEntry()
			if entry != nil {
				return true
			}
		}
	}
	return false
}

// AddNetworkBlockEntry adds a new block entry to the network's history,
// maintaining the configured history size. It is concurrency-safe.
func (n *network) AddNetworkBlockEntry(blockNumber int64, blockData interface{}) {
	if n == nil { // Guard against nil network pointer
		// Cannot use n.logger here. Consider a global logger for such rare cases if necessary.
		return
	}

	// blockData is allowed to be nil. The check for blockNumber is important.
	if blockNumber <= 0 {
		n.logger.Error("Invalid block number in AddNetworkBlockEntry", zap.Int64("blockNumber", blockNumber), zap.Any("blockData", blockData))
		return
	}

	n.blockHistoryMu.Lock()
	defer n.blockHistoryMu.Unlock()

	if n.blockHistory == nil {
		n.blockHistory = list.New()
		if n.blockHistory == nil { // Should not happen with list.New(), but defensive
			n.logger.Error("Failed to initialize network block history list")
			return
		}
	}

	// Check if the new block number is greater than the latest entry
	if n.blockHistory.Len() > 0 {
		latestEntryValue := n.blockHistory.Back().Value
		latestEntry, ok := latestEntryValue.(blockHistoryEntry)
		if !ok {
			n.logger.Error("Invalid type in network block history during add check, skipping addition")
			return // Or handle error appropriately, e.g., clear history if corrupted
		}
		if blockNumber <= latestEntry.blockNumber {
			n.logger.Debug("New block number is not greater than the latest, skipping addition",
				zap.Int64("new_block", blockNumber),
				zap.Int64("latest_block", latestEntry.blockNumber),
				zap.String("network", n.Name),
			)
			return
		}
	}

	// Extract block hash from blockData using handler
	blockHash := ""
	if blockData != nil {
		switch data := blockData.(type) {
		case string:
			blockHash = data
		default:
			// Use handler to extract block hash if available
			if n.handler != nil {
				blockHash = n.handler.ExtractBlockHash(blockData)
			} else {
				// If no handler available, log warning but continue
				n.logger.Warn("No handler available to extract block hash from blockData",
					zap.String("dataType", reflect.TypeOf(blockData).String()),
					zap.String("network", n.Name))
			}
		}
	}

	now := time.Now()
	entry := blockHistoryEntry{
		blockNumber: blockNumber,
		blockHash:   blockHash,
		timestamp:   &now,
	}

	n.blockHistory.PushBack(entry)

	// Trim the list if it exceeds the history size
	for n.blockHistory.Len() > n.NetworkBlockHistorySize {
		if n.blockHistory.Front() != nil {
			n.blockHistory.Remove(n.blockHistory.Front())
		} else {
			break
		}
	}
}

// getLatestBlockEntry returns the most recent block history entry for the network.
// It is concurrency-safe.
func (n *network) getLatestBlockEntry() *blockHistoryEntry {
	if n == nil {
		return nil
	}
	n.blockHistoryMu.RLock()
	defer n.blockHistoryMu.RUnlock()

	// Keep critical nil checks
	if n.blockHistory == nil {
		return nil
	}

	if n.blockHistory.Len() == 0 {
		return nil
	}

	entry, ok := n.blockHistory.Back().Value.(blockHistoryEntry)
	if !ok {
		// This should ideally not happen if entries are always blockHistoryEntry
		n.logger.Error("Invalid type in network block history")
		return nil
	}
	return &entry
}

// checkSelfLoopbackHealth performs a health check on the router's own endpoint (loopback)
func (n *network) checkSelfLoopbackHealth() (*getLatestBlockNumberResult, error) {
	if n.CaddyPort == "" {
		return nil, errors.New("Caddy port is not set")
	}

	// Ensure handler is available
	if n.handler == nil {
		return &getLatestBlockNumberResult{
			blockNumber:    0,
			healthStatus:   Unhealthy,
			responseStatus: 0,
		}, fmt.Errorf("no handler available for network %s", n.Name)
	}

	// Create synthetic request context for consistent logging (this is critical for logFailedAttempt)
	providerHost := fmt.Sprintf("127.0.0.1:%s", n.CaddyPort) // Use loopback address as provider identifier
	healthCheckMethod := n.handler.GetHealthCheckMethod()

	repl, genericContext, _ := createHealthCheckRequestContext(n.Name, providerHost, healthCheckMethod, n)

	// Check if handler was able to create context
	if repl == nil || genericContext == nil {
		return &getLatestBlockNumberResult{
			blockNumber:    0,
			healthStatus:   Unhealthy,
			responseStatus: 0,
		}, fmt.Errorf("handler unavailable or failed to create health check context for network %s", n.Name)
	}

	// Use the loopback URL as the target
	loopbackURL := fmt.Sprintf("http://127.0.0.1:%s/%s", n.CaddyPort, n.Name)

	// Create headers for the request
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Use handler's GetLatestBlockNumber method but maintain detailed logging
	result, err := n.handler.GetLatestBlockNumber(loopbackURL, headers, n.HttpClient, nil, 1)
	if err != nil {
		// Extract method directly from GenericRequestContext - completely generic
		var method string = "unknown"
		if genericContext != nil {
			method = genericContext.Method
		}

		// Maintain the detailed logFailedAttempt logging that's imperative
		logFailedAttempt(LogFailedAttemptParams{
			Reason:              "Self loopback health check failed",
			Logger:              n.logger,
			NetworkPath:         n.Name,
			FailedAttemptNumber: 1, // Single attempt for loopback
			MaxAttempts:         1, // Total attempts is always 1 for loopback
			StatusCodeOfFailure: result.ResponseStatus,
			Error:               err,
			Replacer:            repl,
			RequestMethod:       method,
			RequestParams:       nil, // No params for health checks - completely generic
			RawResponseBody:     nil, // Handler abstracted the response processing
		})

		// Convert handler result to our expected format
		return &getLatestBlockNumberResult{
			blockNumber:    result.BlockNumber,
			healthStatus:   HealthStatus(result.HealthStatus), // Convert networklib.HealthStatus to modules.HealthStatus
			responseStatus: result.ResponseStatus,
		}, err
	}

	// Success!
	return &getLatestBlockNumberResult{
		blockNumber:    result.BlockNumber,
		healthStatus:   HealthStatus(result.HealthStatus), // Convert networklib.HealthStatus to modules.HealthStatus
		responseStatus: result.ResponseStatus,
	}, nil
}

// getBlock returns the block for a given block or hash
func (n *network) getBlockByNumber(blockNumber int64) (interface{}, error) {
	n.logger.Debug("getBlockByNumber called", zap.Int64("blockNumber", blockNumber), zap.String("networkName", n.Name))

	if n.CaddyPort == "" {
		return nil, errors.New("Caddy port is not set")
	}

	// Check if handler supports get block by number
	if n.handler == nil || !n.handler.SupportsGetBlockByNumber() {
		n.logger.Debug("Network doesn't support getBlockByNumber", zap.String("networkName", n.Name))
		return nil, nil
	}

	// Check if HttpClient is available
	if n.HttpClient == nil {
		return nil, errors.New("HTTP client is not set")
	}

	// Use loopback URL to make request through the Din middleware
	url := fmt.Sprintf("http://127.0.0.1:%s/%s", n.CaddyPort, n.Name)
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Get the block by number method directly from the handler
	getBlockMethod := n.handler.GetBlockByNumberMethod()
	if getBlockMethod == "" {
		return nil, errors.New("handler does not provide a getBlockByNumber method")
	}

	// Use handler to create the block request payload
	payload, err := n.handler.CreateBlockRequest(getBlockMethod, blockNumber, false)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create block request")
	}

	n.logger.Debug("Sending getBlockByNumber request", zap.String("url", url), zap.Any("headers", headers), zap.String("payload", string(payload)))

	resBytes, statusCode, err := n.HttpClient.Post(url, headers, payload, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Error sending POST request")
	}

	responseSnippet := string(resBytes)
	if len(resBytes) > 200 {
		responseSnippet = string(resBytes[:200])
	}
	n.logger.Debug("Received getBlockByNumber response", zap.Int("statusCode", *statusCode), zap.String("responseBodySnippet", responseSnippet))

	if *statusCode != http.StatusOK {
		n.logger.Warn("Error getting block from response, non-OK status", zap.Int("statusCode", *statusCode), zap.String("networkName", n.Name))
		return nil, errors.New("Error getting block from response")
	}

	// Use handler to parse the block response
	blockData, err := n.handler.ParseBlockResponse(resBytes)
	if err != nil {
		return nil, errors.Wrap(err, "Error parsing block response")
	}

	n.logger.Debug("Successfully parsed block response", zap.String("networkName", n.Name))
	return blockData, nil
}
