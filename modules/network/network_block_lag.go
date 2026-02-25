package network

import (
	"math"
	"sync/atomic"

	"go.uber.org/zap"
)

// Block lag calculation constants
const (
	// BlockLagCalculationLookback is the number of blocks to look back when calculating block time
	// Using 1024 blocks provides accurate measurement and makes reorgs/delays a rounding error
	BlockLagCalculationLookback = 1024

	// MinBlockLagLimit is the minimum block lag limit
	MinBlockLagLimit = 5
)

// calculateDynamicBlockLagLimit calculates and sets the optimal block lag limit
// based on immutable blockchain data (block timestamps).
// This produces deterministic results across all server instances since it uses
// historical block timestamps that are baked into the blockchain.
func (n *network) calculateDynamicBlockLagLimit() {
	n.logger.Debug("Starting dynamic block lag limit calculation",
		zap.String("network", n.Name),
		zap.Int("provider_count", len(n.Providers)))

	// Check if handler supports dynamic block lag
	if n.handler == nil || !n.handler.SupportsDynamicBlockLag() {
		n.logger.Debug("Handler does not support dynamic block lag, keeping default",
			zap.String("network", n.Name),
			zap.Int64("default_limit", atomic.LoadInt64(&n.BlockLagLimit)))
		return
	}

	// Check if we have providers
	if len(n.Providers) == 0 {
		n.logger.Warn("No providers available for dynamic block lag calculation, keeping default",
			zap.String("network", n.Name),
			zap.Int64("default_limit", atomic.LoadInt64(&n.BlockLagLimit)))
		return
	}

	// Get first available provider - if timestamps disagree across providers, we have bigger problems
	var firstProvider *provider
	var providerName string
	for name, p := range n.Providers {
		firstProvider = p
		providerName = name
		break
	}

	if firstProvider == nil {
		n.logger.Warn("No providers available for block time measurement",
			zap.String("network", n.Name),
			zap.Int64("default_limit", atomic.LoadInt64(&n.BlockLagLimit)))
		return
	}

	avgBlockTimeMs, err := n.measureBlockTimeFromTimestamps(firstProvider)
	if err != nil {
		n.logger.Warn("Failed to measure block time, keeping default",
			zap.String("network", n.Name),
			zap.String("provider", providerName),
			zap.Int64("default_limit", atomic.LoadInt64(&n.BlockLagLimit)),
			zap.Error(err))
		return
	}

	n.logger.Debug("Successfully measured block time",
		zap.String("network", n.Name),
		zap.String("provider", providerName),
		zap.Float64("avg_block_time_ms", avgBlockTimeMs))

	// Calculate new limit: how many blocks fit in the default lag period
	newLimit := int64(math.Ceil(float64(DefaultBlockLagPeriodMs) / avgBlockTimeMs))

	// Ensure minimum limit
	if newLimit < MinBlockLagLimit {
		newLimit = MinBlockLagLimit
	}

	// Round up to nearest interval of 5
	newLimit = roundUpToInterval(newLimit, 5)

	// Update the block lag limit atomically
	oldLimit := atomic.SwapInt64(&n.BlockLagLimit, newLimit)

	n.logger.Info("Dynamic block lag limit calculated and applied",
		zap.String("network", n.Name),
		zap.Int64("old_limit", oldLimit),
		zap.Int64("new_limit", newLimit),
		zap.String("provider", providerName),
		zap.Float64("avg_block_time_ms", avgBlockTimeMs),
		zap.Int("lookback_blocks", BlockLagCalculationLookback))
}

// measureBlockTimeFromTimestamps calculates the average block time in milliseconds
// using immutable blockchain timestamps from two blocks (N and N-lookback).
// This is deterministic - all servers querying the same blocks get the same result.
func (n *network) measureBlockTimeFromTimestamps(p *provider) (float64, error) {
	// Get the latest block number
	latestResult, err := n.getLatestBlockNumber(
		p.HttpUrl,
		p.Headers,
		p.AuthClient(),
		p.host,
	)
	if err != nil {
		return 0, err
	}
	latestBlock := latestResult.blockNumber

	// Ensure we have enough blocks for the lookback
	if latestBlock < BlockLagCalculationLookback {
		n.logger.Debug("Chain too young for block lag calculation",
			zap.String("network", n.Name),
			zap.Int64("latest_block", latestBlock),
			zap.Int("required_lookback", BlockLagCalculationLookback))
		return 0, nil
	}

	oldBlock := latestBlock - BlockLagCalculationLookback

	// Get timestamp for the latest block
	latestTimestamp, err := n.handler.GetBlockTimestamp(
		p.HttpUrl,
		p.Headers,
		n.HttpClient,
		p.AuthClient(),
		n.RequestAttemptCount,
		latestBlock,
	)
	if err != nil {
		return 0, err
	}

	// Get timestamp for the older block
	oldTimestamp, err := n.handler.GetBlockTimestamp(
		p.HttpUrl,
		p.Headers,
		n.HttpClient,
		p.AuthClient(),
		n.RequestAttemptCount,
		oldBlock,
	)
	if err != nil {
		return 0, err
	}

	// Calculate time difference in seconds
	timeDiffSeconds := latestTimestamp - oldTimestamp
	if timeDiffSeconds <= 0 {
		n.logger.Warn("Invalid timestamp difference",
			zap.String("network", n.Name),
			zap.Int64("latest_block", latestBlock),
			zap.Int64("old_block", oldBlock),
			zap.Int64("latest_timestamp", latestTimestamp),
			zap.Int64("old_timestamp", oldTimestamp))
		return 0, nil
	}

	// Calculate average block time in milliseconds
	avgBlockTimeMs := (float64(timeDiffSeconds) * 1000) / float64(BlockLagCalculationLookback)

	n.logger.Debug("Block time calculation details",
		zap.String("network", n.Name),
		zap.Int64("latest_block", latestBlock),
		zap.Int64("old_block", oldBlock),
		zap.Int64("time_diff_seconds", timeDiffSeconds),
		zap.Float64("avg_block_time_ms", avgBlockTimeMs))

	return avgBlockTimeMs, nil
}

// roundUpToInterval rounds a value up to the nearest multiple of the interval
func roundUpToInterval(value int64, interval int64) int64 {
	if value%interval == 0 {
		return value
	}
	return ((value / interval) + 1) * interval
}
