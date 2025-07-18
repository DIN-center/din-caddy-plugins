package modules

import (
	"container/list"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
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
	Type             string `json:"type"` // Network type for handler registry (evm, eth_beacon_chain, starknet, solana)
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

// NewNetwork creates a new network with the given name and network type
// Only put values in the struct definition that are constant
// Don't kick off any Background processes here
func NewNetwork(name string, networkType string, environment utils.Environment, caddyPort string) (*network, error) {
	n := &network{
		Name: name,
		Type: networkType, // Used for handler selection
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

	// Initialize handler based on network type
	if networkType != "" {
		config := &networklib.NetworkConfig{
			Name:           name,
			Type:           networkType,
			ChainID:        "",
			MaxPayloadSize: DefaultMaxRequestPayloadSizeKB * 1024,
			RequestTimeout: time.Duration(DefaultHCTimeout) * time.Second,
			Custom:         make(map[string]interface{}),
		}

		// Add logging for handler initialization

		handler, err := networklib.DefaultRegistry.GetHandler(networkType, config)
		if err != nil {
			return nil, fmt.Errorf("failed to get handler for network type '%s': %w", networkType, err)
		}

		n.handler = handler
	}

	return n, nil
}

// UpdateHandler updates the network's handler (used when type is explicitly set in Caddyfile)
func (n *network) UpdateHandler(handler networklib.NetworkHandler) {
	n.handler = handler

	// Verify handler immediately after setting
	if n.handler == nil {
		// This should never happen, but log if it does
		if n.logger != nil {
			n.logger.Error("CRITICAL: Handler is nil immediately after setting", zap.String("network", n.Name))
		}
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
	// Add handler status logging at the start of health check

	// Self loopback health check (run asynchronously) - TEMPORARILY DISABLED due to circular dependency
	// go n.LoopbackHealthCheck()

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

	// Archive Health Check - Use handler method instead of string checks
	if n.ArchiveEnabled && n.supportsArchiveMode() && len(provider.BlockHistory()) > 1 {
		quarterBlockHeight := currentBlock / 4

		// Use handler method to format block height
		quarterBlockHeightString := n.formatBlockHeight(quarterBlockHeight)

		// Use handler method to create archive payload
		err := n.archiveModeCheck(provider.HttpUrl, provider.Headers, provider.AuthClient(), quarterBlockHeightString)
		if err != nil {
			n.logProviderWarning("Error testing archive mode", provider,
				zap.Int64("quarter_block_height", quarterBlockHeight),
				zap.String("quarter_block_height_formatted", quarterBlockHeightString),
				zap.Error(err),
				zap.String("health_status", Warning.String()))
			return Warning
		}
	}

	return worstStatus
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
	var lastErr error
	var lastHealthStatus HealthStatus = Unhealthy
	var lastResponseStatus int = 0

	// Use handler method instead of hardcoded HCMethod
	healthCheckMethod := n.getHealthCheckMethod()

	// Create synthetic request context for consistent logging
	repl, jsonRPCReq, _ := createHealthCheckRequestContext(n.Name, providerHost, healthCheckMethod)

	// Layer 1: Handle attempts
	for attempt := 0; attempt < n.RequestAttemptCount; attempt++ {
		// Use handler-created payload instead of hardcoded format
		healthPayload, err := n.handler.CreateHealthCheckPayload(healthCheckMethod)
		if err != nil {
			return nil, fmt.Errorf("failed to create health check payload: %w", err)
		}

		// Update the replacer with actual payload
		if healthPayload != nil {
			repl.Set(RequestBodyKey, healthPayload)
		}

		// Use appropriate HTTP method based on handler preference
		var resBytes []byte
		var statusCode *int

		// Debug logging
		if n.handler != nil {
			n.logger.Debug("Health check method selection",
				zap.String("network", n.Name),
				zap.String("handler_type", n.handler.GetType()),
				zap.String("http_method", n.handler.GetHealthCheckHTTPMethod()),
				zap.String("health_check_method", healthCheckMethod),
				zap.String("provider_url", httpUrl))
		}

		if n.handler != nil && n.handler.GetHealthCheckHTTPMethod() == "GET" {
			// For REST APIs like beacon chain, make GET request to the endpoint
			parsedURL, parseErr := url.Parse(httpUrl)
			if parseErr != nil {
				lastErr = fmt.Errorf("failed to parse provider URL: %w", parseErr)
				continue
			}

			// Combine the existing path with the health check method
			parsedURL.Path = parsedURL.Path + healthCheckMethod
			healthCheckURL := parsedURL.String()

			n.logger.Debug("Making GET request for health check",
				zap.String("url", healthCheckURL))
			resBytes, statusCode, err = n.HttpClient.Get(healthCheckURL, headers, ac)
		} else {
			// For JSON-RPC APIs, make POST request with payload
			n.logger.Debug("Making POST request for health check",
				zap.String("url", httpUrl),
				zap.ByteString("payload", healthPayload))
			resBytes, statusCode, err = n.HttpClient.Post(httpUrl, headers, healthPayload, ac)
		}
		if err != nil {
			lastErr = err
			if statusCode != nil {
				lastResponseStatus = *statusCode
			}

			// Log the failed attempt with detailed information
			logFailedAttempt(&LogFailedAttemptParams{
				Reason:              "Health check HTTP request failed",
				Logger:              n.logger,
				NetworkPath:         n.Name,
				FailedAttemptNumber: attempt + 1,
				MaxAttempts:         n.RequestAttemptCount,
				StatusCodeOfFailure: lastResponseStatus,
				Error:               err,
				Replacer:            repl,
				ParsedReqBody:       jsonRPCReq,
				RawResponseBody:     nil,
			})
			continue
		}

		// Use handler method to parse response instead of hardcoded parsing
		blockInfo, health, err := n.processHealthCheckResponse(resBytes, statusCode)
		if err != nil {
			lastErr = err
			lastHealthStatus = health
			if statusCode != nil {
				lastResponseStatus = *statusCode
			}

			logFailedAttempt(&LogFailedAttemptParams{
				Reason:              "Health check response processing failed",
				Logger:              n.logger,
				NetworkPath:         n.Name,
				FailedAttemptNumber: attempt + 1,
				MaxAttempts:         n.RequestAttemptCount,
				StatusCodeOfFailure: lastResponseStatus,
				Error:               err,
				Replacer:            repl,
				ParsedReqBody:       jsonRPCReq,
				RawResponseBody:     resBytes,
			})

			continue
		}

		// Debug log before checking for separate block info call
		if n.logger != nil {
			n.logger.Debug("Checking if separate block info call is needed",
				zap.String("network", n.Name),
				zap.Bool("has_handler", n.handler != nil),
				zap.Bool("requires_separate_call", n.handler != nil && n.handler.RequiresSeparateBlockInfoCall()),
				zap.Int64("block_info", blockInfo),
				zap.Bool("condition_met", n.handler != nil && n.handler.RequiresSeparateBlockInfoCall() && blockInfo == -1))
		}

		// Check if we need a separate call for block info (e.g., beacon chain)
		if n.handler != nil && n.handler.RequiresSeparateBlockInfoCall() && blockInfo == -1 {
			// Make separate call to get block info
			blockInfoMethod := n.handler.GetBlockInfoMethod()
			blockInfoURL := fmt.Sprintf("%s%s", httpUrl, blockInfoMethod)

			n.logger.Debug("Making separate GET request for block info",
				zap.String("network", n.Name),
				zap.String("url", blockInfoURL),
				zap.String("block_info_method", blockInfoMethod))

			blockInfoBytes, blockInfoStatus, blockInfoErr := n.HttpClient.Get(blockInfoURL, headers, ac)
			if blockInfoErr != nil {
				lastErr = blockInfoErr
				if blockInfoStatus != nil {
					lastResponseStatus = *blockInfoStatus
				}

				logFailedAttempt(&LogFailedAttemptParams{
					Reason:              "Block info request failed",
					Logger:              n.logger,
					NetworkPath:         n.Name,
					FailedAttemptNumber: attempt + 1,
					MaxAttempts:         n.RequestAttemptCount,
					StatusCodeOfFailure: lastResponseStatus,
					Error:               blockInfoErr,
					Replacer:            repl,
					ParsedReqBody:       nil,
					RawResponseBody:     nil,
				})
				continue
			}

			// Parse block info response
			blockInfoResult, parseErr := n.handler.ParseHealthCheckResponse(blockInfoBytes)
			if parseErr != nil {
				lastErr = parseErr
				if blockInfoStatus != nil {
					lastResponseStatus = *blockInfoStatus
				}

				logFailedAttempt(&LogFailedAttemptParams{
					Reason:              "Block info parsing failed",
					Logger:              n.logger,
					NetworkPath:         n.Name,
					FailedAttemptNumber: attempt + 1,
					MaxAttempts:         n.RequestAttemptCount,
					StatusCodeOfFailure: lastResponseStatus,
					Error:               parseErr,
					Replacer:            repl,
					ParsedReqBody:       nil,
					RawResponseBody:     blockInfoBytes,
				})
				continue
			}

			// Check if block info result is nil
			if blockInfoResult == nil {
				lastErr = fmt.Errorf("handler returned nil block info")
				logFailedAttempt(&LogFailedAttemptParams{
					Reason:              "Block info result is nil",
					Logger:              n.logger,
					NetworkPath:         n.Name,
					FailedAttemptNumber: attempt + 1,
					MaxAttempts:         n.RequestAttemptCount,
					StatusCodeOfFailure: lastResponseStatus,
					Error:               lastErr,
					Replacer:            repl,
					ParsedReqBody:       nil,
					RawResponseBody:     blockInfoBytes,
				})
				continue
			}

			// Update block info with actual data
			blockInfo = blockInfoResult.Number

			// Log successful block info update
			if n.logger != nil {
				n.logger.Debug("Successfully updated block info from separate call",
					zap.String("network", n.Name),
					zap.Int64("block_number", blockInfo),
					zap.Int64("slot", blockInfoResult.Slot),
					zap.Int64("epoch", blockInfoResult.Epoch))
			}
		}

		// If the current attempt was successful, its status code should be used.
		if statusCode != nil {
			lastResponseStatus = *statusCode
		}

		return &getLatestBlockNumberResult{
			blockNumber:    blockInfo,
			healthStatus:   health,
			responseStatus: lastResponseStatus,
		}, nil
	}

	return &getLatestBlockNumberResult{
		blockNumber:    0,
		healthStatus:   lastHealthStatus,
		responseStatus: lastResponseStatus,
	}, errors.Wrap(lastErr, fmt.Sprintf("Failed after %d attempts", n.RequestAttemptCount))
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
func (n *network) getHealthCheckMethod() string {
	// Add logging to track handler usage
	if n.logger != nil {
		n.logger.Debug("getHealthCheckMethod called",
			zap.String("network", n.Name),
			zap.String("type", n.Type),
			zap.Bool("has_handler", n.handler != nil))
	}

	// Use handler method if available
	if n.handler != nil {
		method := n.handler.GetHealthCheckMethod()
		if n.logger != nil {
			n.logger.Debug("Using handler health check method",
				zap.String("network", n.Name),
				zap.String("method", method),
				zap.String("handler_type", n.handler.GetType()))
		}
		return method
	}

	return ""
}

// getArchiveMethod returns the archive method using handler
func (n *network) getArchiveMethod() string {
	// Use handler method if available
	if n.handler != nil {
		return n.handler.GetArchiveMethod()
	}

	return ""
}

// processHealthCheckResponse processes health check response using handler
func (n *network) processHealthCheckResponse(resBytes []byte, statusCode *int) (int64, HealthStatus, error) {
	// Evaluate health status based on status code
	if statusCode == nil {
		return 0, Unhealthy, errors.New("received nil statusCode in processHealthCheckResponse")
	}

	if *statusCode >= 400 {
		// If status code is 429, set health status to Warning, otherwise return as unhealthy
		if *statusCode == 429 {
			return 0, Warning, fmt.Errorf("rate limit error (status code: %d)", *statusCode)
		}
		return 0, Unhealthy, fmt.Errorf("error status code: %d", *statusCode)
	}

	// Add logging to track handler usage
	if n.logger != nil {
		n.logger.Debug("processHealthCheckResponse called",
			zap.String("network", n.Name),
			zap.String("type", n.Type),
			zap.Bool("has_handler", n.handler != nil),
			zap.String("response_snippet", string(resBytes[:min(len(resBytes), 100)])))
	}

	// Use handler method if available
	if n.handler != nil {
		if n.logger != nil {
			n.logger.Debug("Using handler to parse health check response",
				zap.String("network", n.Name),
				zap.String("handler_type", n.handler.GetType()))
		}
		blockInfo, err := n.handler.ParseHealthCheckResponse(resBytes)
		if err != nil {
			if n.logger != nil {
				n.logger.Error("Handler failed to parse health check response",
					zap.String("network", n.Name),
					zap.String("handler_type", n.handler.GetType()),
					zap.Error(err))
			}
			return 0, Unhealthy, errors.Wrap(err, "Handler failed to parse health check response")
		}
		if n.logger != nil {
			n.logger.Debug("Handler successfully parsed health check response",
				zap.String("network", n.Name),
				zap.Int64("block_number", blockInfo.Number))
		}
		return blockInfo.Number, Healthy, nil
	}

	return 0, Unhealthy, fmt.Errorf("no handler available for network %s", n.Name)
}

// archiveModeCheck performs archive mode check using handler
func (n *network) archiveModeCheck(httpUrl string, headers map[string]string, ac auth.IAuthClient, quarterBlockHeight string) error {
	// Use handler if available
	if n.handler == nil {
		if n.logger != nil {
			n.logger.Error("No handler available for archive mode check",
				zap.String("network", n.Name),
				zap.String("type", n.Type))
		}
		return fmt.Errorf("no handler available for archive mode check on network %s", n.Name)
	}

	// Get archive method from handler
	method := n.handler.GetArchiveMethod()
	if method == "" {
		return fmt.Errorf("handler does not provide archive method for network %s", n.Name)
	}

	var lastErr error

	// Create synthetic request context for consistent logging
	providerHost := httpUrl // Use the full URL as provider identifier for archive mode checks
	repl, jsonRPCReq, _ := createHealthCheckRequestContext(n.Name, providerHost, method)

	// Layer 1: Handle attempts
	for attempt := 0; attempt < n.RequestAttemptCount; attempt++ {
		// Use handler to create archive payload
		payload, err := n.handler.CreateArchivePayload(method, quarterBlockHeight)
		if err != nil {
			lastErr = errors.Wrap(err, "Failed to create archive payload")

			// Log the failed attempt with detailed information
			logFailedAttempt(&LogFailedAttemptParams{
				Reason:              "Health check payload creation failed",
				Logger:              n.logger,
				NetworkPath:         n.Name,
				FailedAttemptNumber: attempt + 1,
				MaxAttempts:         n.RequestAttemptCount,
				StatusCodeOfFailure: 0, // No status code for payload creation errors
				Error:               lastErr,
				Replacer:            repl,
				ParsedReqBody:       jsonRPCReq,
				RawResponseBody:     nil, // no response body for payload creation errors
			})
			continue
		}

		// Update the request body in the replacer with the actual payload
		repl.Set(RequestBodyKey, payload)

		// Send the POST request
		resBytes, statusCode, err := n.HttpClient.Post(httpUrl, headers, payload, ac)
		if err != nil {
			lastErr = errors.Wrap(err, "Error sending POST request")

			// Log the failed attempt with detailed information
			logFailedAttempt(&LogFailedAttemptParams{
				Reason:              "Health check HTTP request failed",
				Logger:              n.logger,
				NetworkPath:         n.Name,
				FailedAttemptNumber: attempt + 1,
				MaxAttempts:         n.RequestAttemptCount,
				StatusCodeOfFailure: 0, // No status code for HTTP errors
				Error:               lastErr,
				Replacer:            repl,
				ParsedReqBody:       jsonRPCReq,
				RawResponseBody:     nil, // no response body for HTTP errors
			})
			continue
		}

		if *statusCode == http.StatusServiceUnavailable || *statusCode == StatusOriginUnreachable {
			lastErr = errors.New("Network Unavailable")

			// Log the failed attempt with detailed information
			logFailedAttempt(&LogFailedAttemptParams{
				Reason:              "Health check HTTP request failed",
				Logger:              n.logger,
				NetworkPath:         n.Name,
				FailedAttemptNumber: attempt + 1,
				MaxAttempts:         n.RequestAttemptCount,
				StatusCodeOfFailure: *statusCode,
				Error:               lastErr,
				Replacer:            repl,
				ParsedReqBody:       jsonRPCReq,
				RawResponseBody:     nil, // Check for JSON-RPC error below
			})
			continue
		}

		// Use handler to parse archive response
		err = n.handler.ParseArchiveResponse(resBytes)
		if err != nil {
			lastErr = errors.Wrap(err, "Archive mode check failed")

			// Log the failed attempt with detailed information
			logFailedAttempt(&LogFailedAttemptParams{
				Reason:              "Health check response processing failed",
				Logger:              n.logger,
				NetworkPath:         n.Name,
				FailedAttemptNumber: attempt + 1,
				MaxAttempts:         n.RequestAttemptCount,
				StatusCodeOfFailure: *statusCode,
				Error:               lastErr,
				Replacer:            repl,
				ParsedReqBody:       jsonRPCReq,
				RawResponseBody:     resBytes,
			})
			continue
		}

		// Success case - handler validated the response
		return nil
	}

	return errors.Wrap(lastErr, fmt.Sprintf("Failed after %d attempts", n.RequestAttemptCount))
}

// supportsArchiveMode checks if the network supports archive mode using handler
func (n *network) supportsArchiveMode() bool {
	// Use handler method if available
	if n.handler != nil {
		return n.handler.SupportsArchiveMode()
	}

	return false
}

// formatBlockHeight formats block height using handler
func (n *network) formatBlockHeight(blockNum int64) string {
	// Use handler method if available
	if n.handler != nil {
		return n.handler.FormatBlockHeight(blockNum)
	}

	return ""
}

func (n *network) close() {
	close(n.quit)
}

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
	url := fmt.Sprintf("http://127.0.0.1:%s/%s", n.CaddyPort, n.Name)

	// Create synthetic request context for consistent logging
	providerHost := fmt.Sprintf("127.0.0.1:%s", n.CaddyPort) // Use loopback address as provider identifier
	repl, jsonRPCReq, payload := createHealthCheckRequestContext(n.Name, providerHost, n.getHealthCheckMethod())

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	resBytes, statusCode, err := n.HttpClient.Post(url, headers, payload, nil)
	currentResponseStatus := 0 // Default status code if statusCode is nil
	if statusCode != nil {
		currentResponseStatus = *statusCode
	}

	if err != nil {
		// Log the failed loopback attempt with detailed information
		logFailedAttempt(&LogFailedAttemptParams{
			Reason:              "Health check HTTP request failed",
			Logger:              n.logger,
			NetworkPath:         n.Name,
			FailedAttemptNumber: 1, // Single attempt for loopback
			MaxAttempts:         1, // Total attempts is always 1 for loopback
			StatusCodeOfFailure: currentResponseStatus,
			Error:               err,
			Replacer:            repl,
			ParsedReqBody:       jsonRPCReq,
			RawResponseBody:     nil, // no response body for HTTP errors
		})

		return &getLatestBlockNumberResult{
			blockNumber:    0,
			healthStatus:   Unhealthy,
			responseStatus: currentResponseStatus, // Use the safe value
		}, errors.Wrap(err, "Self loopback health check failed")
	}

	// statusCode is known to be non-nil here if err was nil, because processBlockNumberResponse requires non-nil statusCode
	blockNumber, health, err := n.processBlockNumberResponse(resBytes, statusCode)
	if err != nil {
		// Log the failed loopback attempt with detailed information
		logFailedAttempt(&LogFailedAttemptParams{
			Reason:              "Health check response processing failed",
			Logger:              n.logger,
			NetworkPath:         n.Name,
			FailedAttemptNumber: 1, // Single attempt for loopback
			MaxAttempts:         1, // Total attempts is always 1 for loopback
			StatusCodeOfFailure: currentResponseStatus,
			Error:               err,
			Replacer:            repl,
			ParsedReqBody:       jsonRPCReq,
			RawResponseBody:     resBytes,
		})

		// It's possible processBlockNumberResponse gets an error but statusCode was valid (e.g. 200 OK with bad JSON)
		// So, we still use currentResponseStatus (which would be *statusCode from the successful Post)
		return &getLatestBlockNumberResult{
			blockNumber:    0,
			healthStatus:   health, // Health status from processBlockNumberResponse
			responseStatus: currentResponseStatus,
		}, errors.Wrap(err, "Self loopback health check response error")
	}

	return &getLatestBlockNumberResult{
		blockNumber:    blockNumber,
		healthStatus:   health,
		responseStatus: currentResponseStatus, // Should be *statusCode from successful Post
	}, nil
}

// getBlock returns the block for a given block or hash
func (n *network) getBlockByNumber(blockNumber int64) (interface{}, error) {
	n.logger.Debug("getBlockByNumber called", zap.Int64("blockNumber", blockNumber), zap.String("networkName", n.Name))

	if n.CaddyPort == "" {
		return nil, errors.New("Caddy port is not set")
	}

	url := fmt.Sprintf("http://127.0.0.1:%s/%s", n.CaddyPort, n.Name)
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Use handler method instead of string checks
	if !n.supportsGetBlockByNumber() {
		n.logger.Debug("Network doesn't support getBlockByNumber", zap.String("networkName", n.Name))
		return nil, nil
	}

	// Use handler method to create block request
	supportedMethods := n.getSupportedMethods()
	if len(supportedMethods) == 0 {
		return nil, errors.New("no supported block methods available")
	}

	getBlockMethod := n.getBlockByNumberMethod()
	payload, err := n.createBlockRequest(getBlockMethod, blockNumber, false)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create block request")
	}

	n.logger.Debug("Sending getBlockByNumber request", zap.String("url", url), zap.Any("headers", headers), zap.String("payload", string(payload)))

	resBytes, statusCode, err := n.HttpClient.Post(url, headers, payload, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Error sending POST request")
	}

	n.logger.Debug("Received getBlockByNumber response", zap.Int("statusCode", *statusCode), zap.String("responseBodySnippet", string(resBytes[:min(len(resBytes), 200)])))

	if *statusCode != http.StatusOK {
		n.logger.Warn("Error getting block from response, non-OK status", zap.Int("statusCode", *statusCode), zap.String("networkName", n.Name))
		return nil, errors.New("Error getting block from response")
	}

	// Use handler method to parse block response
	blockData, err := n.parseBlockResponse(resBytes)
	if err != nil {
		return nil, errors.Wrap(err, "Error parsing block response")
	}

	n.logger.Debug("Successfully parsed block response", zap.String("networkName", n.Name))
	return blockData, nil
}

// supportsGetBlockByNumber checks if the network supports getBlockByNumber using handler
func (n *network) supportsGetBlockByNumber() bool {
	// Use handler method if available
	if n.handler != nil {
		return n.handler.SupportsGetBlockByNumber()
	}

	return false
}

// getSupportedMethods gets supported methods using handler
func (n *network) getSupportedMethods() []string {
	// Use handler method if available
	if n.handler != nil {
		return n.handler.GetSupportedMethods()
	}

	return []string{}
}

// getBlockByNumberMethod gets the block by number method using handler
func (n *network) getBlockByNumberMethod() string {
	// Use handler method if available
	if n.handler != nil {
		supportedMethods := n.handler.GetSupportedMethods()

		// First, look for specific getBlockByNumber methods
		for _, method := range supportedMethods {
			methodLower := strings.ToLower(method)
			if strings.Contains(methodLower, "getblockbynumber") ||
				strings.Contains(methodLower, "get_block_by_number") {
				return method
			}
		}

		// Fallback: look for any block-related method (but this shouldn't happen with proper handlers)
		for _, method := range supportedMethods {
			if strings.Contains(strings.ToLower(method), "block") &&
				!strings.Contains(strings.ToLower(method), "blocknumber") {
				return method
			}
		}
	}

	return ""
}

// createBlockRequest creates block request using handler
func (n *network) createBlockRequest(method string, blockNumber int64, includeTransactions bool) ([]byte, error) {
	// Use handler method if available
	if n.handler != nil {
		return n.handler.CreateBlockRequest(method, blockNumber, includeTransactions)
	}

	return nil, fmt.Errorf("no handler available for network %s", n.Name)
}

// parseBlockResponse parses block response using handler
func (n *network) parseBlockResponse(resBytes []byte) (interface{}, error) {
	// Use handler method if available
	if n.handler != nil {
		return n.handler.ParseBlockResponse(resBytes)
	}

	return nil, fmt.Errorf("no handler available for network %s", n.Name)
}
