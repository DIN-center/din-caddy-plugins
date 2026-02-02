package modules

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
)

// startHealthcheck starts the health check goroutine for the network
func (n *network) startHealthcheck() {
	// Start dynamic block lag limit calculation (runs once asynchronously)
	go n.calculateDynamicBlockLagLimit()

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
		var healthStatus = Healthy
		latestBlockResult, err := n.getLatestBlockNumber(provider.HttpUrl, provider.Headers, provider.AuthClient(), provider.host)
		if err != nil {
			n.logProviderWarning("Health check failed after all attempts for provider", provider,
				zap.Int64("block_number", latestBlockResult.blockNumber),
				zap.String("provider", provider.host),
				zap.String("providerName", provider.Name),
				zap.Int("response_status", latestBlockResult.responseStatus),
				zap.String("health_status", latestBlockResult.healthStatus.String()),
				zap.Int("total_attempts", n.RequestAttemptCount),
				zap.Error(err))
			// Handle error cases with grace period logic
			healthStatus := n.handleErrorWithGracePeriod(provider, latestBlockResult.healthStatus, latestBlockResult.blockNumber)

			if healthStatus == Unhealthy {
				// Add the block entry and send metric
				provider.AddBlockEntry(latestBlockResult.blockNumber, Unhealthy, n.ProviderBlockHistorySize)
				n.sendHealthCheckMetric(provider.host, provider.Name, latestBlockResult.responseStatus, latestBlockResult.healthStatus.String(), latestBlockResult.blockNumber, provider.Priority, string(n.Environment))

				continue // Skip further checks for confirmed unhealthy providers
			}
		}
		// Evaluate final health status
		newStatus := n.evaluateProviderHealth(provider, latestBlockResult.blockNumber, healthStatus, latestNetworkBlock)

		// Update metrics and history
		provider.AddBlockEntry(latestBlockResult.blockNumber, newStatus, n.ProviderBlockHistorySize)
		n.sendHealthCheckMetric(provider.host, provider.Name, latestBlockResult.responseStatus, newStatus.String(), latestBlockResult.blockNumber, provider.Priority, string(n.Environment))
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
		zap.String("providerName", provider.Name),
		zap.String("network", n.Name),
		zap.Int("priority", provider.Priority),
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
	isLagged, blockLag, blockLagLimit := n.checkBlockLag(provider, currentBlock, latestNetworkBlock)
	if isLagged {
		if Warning > worstStatus {
			worstStatus = Warning
		}
	}

	// Check if block is too far ahead (block jump)
	if latestNetworkBlock > 0 {
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
				zap.Int64("block_lag_limit", blockLagLimit),
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

// checkSelfLoopbackHealth performs a health check on the router's own endpoint (loopback)
func (n *network) checkSelfLoopbackHealth() (*getLatestBlockNumberResult, error) {
	if n.CaddyPort == "" {
		return nil, fmt.Errorf("Caddy port is not set")
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
		var method = "unknown"
		if genericContext != nil {
			method = genericContext.Method
		}

		// Set default values for when result is nil
		var statusCode = 0
		if result != nil {
			statusCode = result.ResponseStatus
		}

		// Maintain the detailed logFailedAttempt logging that's imperative
		logFailedAttempt(LogFailedAttemptParams{
			Reason:              "Self loopback health check failed",
			Logger:              n.logger,
			NetworkPath:         n.Name,
			FailedAttemptNumber: 1, // Single attempt for loopback
			MaxAttempts:         1, // Total attempts is always 1 for loopback
			StatusCodeOfFailure: statusCode,
			Error:               err,
			Replacer:            repl,
			RequestMethod:       method,
			RequestParams:       nil, // No params for health checks - completely generic
			RawResponseBody:     nil, // Handler abstracted the response processing
		})

		// Return safe defaults when request failed (result may be nil)
		return &getLatestBlockNumberResult{
			blockNumber:    0,          // Unknown block number since request failed
			healthStatus:   Unhealthy,  // Mark as unhealthy since the request failed
			responseStatus: statusCode, // Use safe status code (0 if result is nil)
		}, err
	}

	// Success!
	return &getLatestBlockNumberResult{
		blockNumber:    result.BlockNumber,
		healthStatus:   HealthStatus(result.HealthStatus), // Convert networklib.HealthStatus to modules.HealthStatus
		responseStatus: result.ResponseStatus,
	}, nil
}

// sendHealthCheckMetric sends health check metrics to Prometheus
func (n *network) sendHealthCheckMetric(provider string, providerName string, responseStatus int, healthStatus string, blockNumber int64, priority int, environment string) {
	n.PrometheusClient.HandleHealthCheckMetric(&prom.PromHealthCheckMetricData{
		Network:        n.Name,
		Provider:       provider,
		ProviderName:   providerName,
		ResponseStatus: responseStatus,
		HealthStatus:   healthStatus,
		BlockNumber:    blockNumber,
		Priority:       priority,
		Environment:    environment,
	})
}

// getLatestBlockNumber fetches the latest block number from a provider
func (n *network) getLatestBlockNumber(httpUrl string, headers map[string]string, ac auth.IAuthClient, providerHost string) (*getLatestBlockNumberResult, error) {
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
