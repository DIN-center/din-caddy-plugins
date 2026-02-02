package network

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
	prom "github.com/DIN-center/din-caddy-plugins/lib/prometheus"
)

// StartHealthcheck starts the health check goroutine for the network
func (n *Network) StartHealthcheck() {
	// Start dynamic block lag limit calculation (runs once asynchronously)
	go n.CalculateDynamicBlockLagLimit()

	n.healthCheck()
	ticker := time.NewTicker(time.Second * time.Duration(n.HCInterval))
	go func() {
		// Keep an index for RPC request IDs
		for i := 0; ; i++ {
			select {
			// Cleanup if the quit channel gets closed. Right now nothing closes this channel, but
			// once we integrate the authentication work there's code that should.
			case <-n.Quit:
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
func (n *Network) healthCheck() {
	// Add handler status logging at the start of health check

	// Self loopback health check (run asynchronously)
	go n.LoopbackHealthCheck()

	// Get latest network block for comparison
	latestNetworkBlock := n.GetLatestHealthyBlock()

	for _, provider := range n.Providers {

		// Get latest block and initial health status
		var healthStatus = Healthy
		latestBlockResult, err := n.GetLatestBlockNumber(provider.HttpUrl, provider.Headers, provider.AuthClient(), provider.Host)
		if err != nil {
			n.logProviderWarning("Health check failed after all attempts for provider", provider,
				zap.Int64("block_number", latestBlockResult.BlockNumber),
				zap.String("provider", provider.Host),
				zap.String("providerName", provider.Name),
				zap.Int("response_status", latestBlockResult.ResponseStatus),
				zap.String("health_status", HealthStatusString(latestBlockResult.HealthStatus)),
				zap.Int("total_attempts", n.RequestAttemptCount),
				zap.Error(err))
			// Handle error cases with grace period logic
			healthStatus := n.HandleErrorWithGracePeriod(provider, latestBlockResult.HealthStatus, latestBlockResult.BlockNumber)

			if healthStatus == Unhealthy {
				// Add the block entry and send metric
				provider.AddBlockEntry(latestBlockResult.BlockNumber, Unhealthy, n.ProviderBlockHistorySize)
				n.SendHealthCheckMetric(provider.Host, provider.Name, latestBlockResult.ResponseStatus, HealthStatusString(latestBlockResult.HealthStatus), latestBlockResult.BlockNumber, provider.Priority, string(n.Environment))

				continue // Skip further checks for confirmed unhealthy providers
			}
		}
		// Evaluate final health status
		newStatus := n.EvaluateProviderHealth(provider, latestBlockResult.BlockNumber, healthStatus, latestNetworkBlock)

		// Update metrics and history
		provider.AddBlockEntry(latestBlockResult.BlockNumber, newStatus, n.ProviderBlockHistorySize)
		n.SendHealthCheckMetric(provider.Host, provider.Name, latestBlockResult.ResponseStatus, HealthStatusString(newStatus), latestBlockResult.BlockNumber, provider.Priority, string(n.Environment))
	}
}

// LoopbackHealthCheck performs a self loopback health check and logs/metrics the result.
func (n *Network) LoopbackHealthCheck() {
	startTime := time.Now()
	selfResult, selfErr := n.CheckSelfLoopbackHealth()
	duration := time.Since(startTime)

	// Prepare metric data regardless of error, as we want to capture response status and duration
	metricData := &prom.PromNetworkHealthCheckMetricData{
		Network:        n.Name, // Use n.Name directly for the network label
		ResponseStatus: 0,      // Default to 0 if selfResult is nil
		Duration:       duration,
		Environment:    string(n.Environment),
	}

	if selfResult != nil {
		metricData.ResponseStatus = selfResult.ResponseStatus
	}

	n.PrometheusClient.HandleNetworkHealthCheckMetric(metricData)

	if selfErr != nil {
		n.Logger.Warn("Self loopback health check failed",
			zap.Error(selfErr),
			zap.String("network", n.Name),
			zap.Int("response_status", metricData.ResponseStatus),
			zap.Duration("duration", duration),
		)
	}
}

// handleErrorWithGracePeriod implements the grace period logic for unhealthy providers
func (n *Network) HandleErrorWithGracePeriod(provider *Provider, healthStatus HealthStatus, blockNum int64) HealthStatus {
	if healthStatus != Unhealthy {
		// For Warning status, reset counter and continue with checks
		provider.ConsecutiveUnhealthyChecks = 0
		return healthStatus
	}

	// Handle Unhealthy status with grace period
	provider.ConsecutiveUnhealthyChecks++
	if provider.ConsecutiveUnhealthyChecks < n.HCThreshold {
		// First unhealthy response - give grace period by converting to warning
		return Warning
	}

	// Provider has exceeded grace period - mark as unhealthy
	n.logProviderWarning("Provider has exceeded grace period, marking as unhealthy", provider,
		zap.Int64("block_number", blockNum),
		zap.Int("consecutive_unhealthy_checks", provider.ConsecutiveUnhealthyChecks),
		zap.Int("healthcheck_threshold", n.HCThreshold),
		zap.String("health_status", HealthStatusString(Unhealthy)))
	return Unhealthy
}

// logProviderWarning logs a warning message with standard provider context
func (n *Network) logProviderWarning(msg string, provider *Provider, fields ...zap.Field) {
	baseFields := []zap.Field{
		zap.String("provider", provider.Host),
		zap.String("providerName", provider.Name),
		zap.String("network", n.Name),
		zap.Int("priority", provider.Priority),
	}

	n.Logger.Warn(msg, append(baseFields, fields...)...)
}

// evaluateProviderHealth performs both levels of health checks
func (n *Network) EvaluateProviderHealth(provider *Provider, currentBlock int64, healthStatus HealthStatus, latestNetworkBlock int64) HealthStatus {
	// Track the worst status we find
	worstStatus := healthStatus

	// if provider has no block history and current health check failed, set it to unhealthy
	// Allow providers with no history to be healthy if the current check succeeded
	if len(provider.GetBlockHistory()) == 0 && healthStatus != Healthy {
		n.logProviderWarning("Provider has no block history and current check failed, marking as unhealthy", provider,
			zap.Int64("current_block", currentBlock),
			zap.Int64("latest_network_block", latestNetworkBlock),
			zap.String("health_status", HealthStatusString(Unhealthy)))
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
			if n.HasOtherHealthyProviders(provider) {
				n.logProviderWarning("Provider is too far ahead of network", provider,
					zap.Int64("block_jump_limit", n.BlockJumpLimit),
					zap.Int64("block_jump", blockJump),
					zap.Int64("provider_block", currentBlock),
					zap.Int64("network_block", latestNetworkBlock),
					zap.String("health_status", HealthStatusString(Unhealthy)))
				return Unhealthy
			} else {
				// If there are no other healthy providers, assume this one is correct
				// and healthy because it might be the first to recover from a network outage
				n.logProviderWarning("Provider is far ahead but keeping as healthy (no other healthy providers)", provider,
					zap.Int64("block_jump_limit", n.BlockJumpLimit),
					zap.Int64("block_jump", blockJump),
					zap.Int64("provider_block", currentBlock),
					zap.Int64("network_block", latestNetworkBlock),
					zap.String("health_status", HealthStatusString(Healthy)))
				return Healthy
			}
		}
	}

	isStalled := n.IsStalled(provider)

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
				zap.String("health_status", HealthStatusString(Unhealthy)))
			return Unhealthy
		}
	} else if isStalled && !n.AllProvidersStalled() {
		n.logProviderWarning("Provider is stalled while others are progressing", provider,
			zap.Int64("provider_block", currentBlock),
			zap.Int64("network_block", latestNetworkBlock),
			zap.String("health_status", HealthStatusString(Warning)))
		return Warning
	}

	// chainId check health check
	// Only perform if handler supports chain ID validation
	if chainIdentifier, ok := n.Handler.(networklib.ChainIdentifier); ok {
		chainId, err := chainIdentifier.GetChainID(provider.HttpUrl, provider.Headers, n.HttpClient, provider.AuthClient(), n.RequestAttemptCount)
		if err != nil {
			n.logProviderWarning("Error getting chain ID", provider,
				zap.String("expected_chain_id", n.ChainId),
				zap.String("health_status", HealthStatusString(Unhealthy)),
				zap.Error(err))
			return Unhealthy
		}

		if err := chainIdentifier.ValidateChainID(chainId); err != nil {
			n.logProviderWarning("Provider has incorrect chain ID", provider,
				zap.String("chain_id", chainId),
				zap.String("expected_chain_id", n.ChainId),
				zap.String("validation_error", err.Error()),
				zap.String("health_status", HealthStatusString(Unhealthy)))
			return Unhealthy
		}
	}

	// Archive Health Check - Use handler method directly
	if err := n.performArchiveCheck(provider, currentBlock); err != nil {
		n.logProviderWarning("Archive mode check failed", provider,
			zap.Int64("current_block", currentBlock),
			zap.Error(err),
			zap.String("health_status", HealthStatusString(Warning)))
		return Warning
	}

	return worstStatus
}

// performArchiveCheck performs archive mode check for a provider if archive mode is enabled
func (n *Network) performArchiveCheck(provider *Provider, currentBlock int64) error {
	// Check if archive mode is enabled
	if !n.ArchiveEnabled {
		return nil // Archive mode disabled, skip check
	}

	// Check if handler supports archive mode using capability interface
	archiveChecker, ok := n.Handler.(networklib.ArchiveChecker)
	if !ok || !archiveChecker.SupportsArchiveMode() {
		return nil // Handler doesn't support archive mode, skip check
	}

	// Check if provider has enough block history
	if len(provider.GetBlockHistory()) <= 1 {
		return nil // Not enough block history, skip check
	}

	quarterBlockHeight := currentBlock / 4

	// Use BlockFetcher capability to format block height
	var quarterBlockHeightString string
	if blockFetcher, ok := n.Handler.(networklib.BlockFetcher); ok {
		quarterBlockHeightString = blockFetcher.FormatBlockHeight(quarterBlockHeight)
	} else {
		// Fallback to string format if BlockFetcher not supported
		quarterBlockHeightString = fmt.Sprintf("%d", quarterBlockHeight)
	}

	return archiveChecker.PerformArchiveCheck(provider.HttpUrl, provider.Headers, n.HttpClient, provider.AuthClient(), n.RequestAttemptCount, quarterBlockHeightString)
}

// checkSelfLoopbackHealth performs a health check on the router's own endpoint (loopback)
func (n *Network) CheckSelfLoopbackHealth() (*GetLatestBlockNumberResult, error) {
	if n.CaddyPort == "" {
		return nil, fmt.Errorf("Caddy port is not set")
	}

	// Create synthetic request context for consistent logging (this is critical for logFailedAttempt)
	providerHost := fmt.Sprintf("127.0.0.1:%s", n.CaddyPort) // Use loopback address as provider identifier
	healthCheckMethod := n.Handler.GetHealthCheckMethod()

	repl, genericContext, _ := CreateHealthCheckRequestContext(n.Name, providerHost, healthCheckMethod, n)

	// Check if handler was able to create context
	if repl == nil || genericContext == nil {
		return &GetLatestBlockNumberResult{
			BlockNumber:    0,
			HealthStatus:   Unhealthy,
			ResponseStatus: 0,
		}, fmt.Errorf("handler unavailable or failed to create health check context for network %s", n.Name)
	}

	// Use the loopback URL as the target
	loopbackURL := fmt.Sprintf("http://127.0.0.1:%s/%s", n.CaddyPort, n.Name)

	// Create headers for the request
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Use handler's GetLatestBlockNumber method but maintain detailed logging
	result, err := n.Handler.GetLatestBlockNumber(loopbackURL, headers, n.HttpClient, nil, 1)
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
		LogFailedAttempt(LogFailedAttemptParams{
			Reason:              "Self loopback health check failed",
			Logger:              n.Logger,
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
		return &GetLatestBlockNumberResult{
			BlockNumber:    0,          // Unknown block number since request failed
			HealthStatus:   Unhealthy,  // Mark as unhealthy since the request failed
			ResponseStatus: statusCode, // Use safe status code (0 if result is nil)
		}, err
	}

	// Success!
	return &GetLatestBlockNumberResult{
		BlockNumber:    result.BlockNumber,
		HealthStatus:   HealthStatus(result.HealthStatus), // Convert networklib.HealthStatus to HealthStatus
		ResponseStatus: result.ResponseStatus,
	}, nil
}

// sendHealthCheckMetric sends health check metrics to Prometheus
func (n *Network) SendHealthCheckMetric(provider string, providerName string, responseStatus int, healthStatus string, blockNumber int64, priority int, environment string) {
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
func (n *Network) GetLatestBlockNumber(httpUrl string, headers map[string]string, ac auth.IAuthClient, providerHost string) (*GetLatestBlockNumberResult, error) {
	// Delegate to handler's GetLatestBlockNumber method
	result, err := n.Handler.GetLatestBlockNumber(httpUrl, headers, n.HttpClient, ac, n.RequestAttemptCount)
	if err != nil {
		n.Logger.Debug("Handler GetLatestBlockNumber failed",
			zap.Error(err),
			zap.String("network", n.Name),
			zap.String("provider", providerHost))

		// If there's an error, result might be nil or partially populated
		if result != nil {
			return &GetLatestBlockNumberResult{
				BlockNumber:    result.BlockNumber,
				HealthStatus:   HealthStatus(result.HealthStatus), // Convert networklib.HealthStatus to HealthStatus
				ResponseStatus: result.ResponseStatus,
			}, err
		} else {
			// Return a default unhealthy result when result is nil
			return &GetLatestBlockNumberResult{
				BlockNumber:    0,
				HealthStatus:   Unhealthy,
				ResponseStatus: 0,
			}, err
		}
	}

	// Success!
	n.Logger.Debug("Handler GetLatestBlockNumber succeeded",
		zap.Int64("block_number", result.BlockNumber),
		zap.String("health_status", result.HealthStatus.String()),
		zap.String("network", n.Name),
		zap.String("provider", providerHost))

	return &GetLatestBlockNumberResult{
		BlockNumber:    result.BlockNumber,
		HealthStatus:   HealthStatus(result.HealthStatus), // Convert networklib.HealthStatus to HealthStatus
		ResponseStatus: result.ResponseStatus,
	}, nil
}
