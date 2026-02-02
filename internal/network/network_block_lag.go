package network

import (
	"container/list"
	"errors"
	"math"
	"net/http"
	"reflect"
	"sync/atomic"
	"time"

	pkgerrors "github.com/pkg/errors"
	"go.uber.org/zap"

	networklib "github.com/DIN-center/din-caddy-plugins/lib/network"
)

// GetLatestBlockNumberResult holds the result of a block number query
type GetLatestBlockNumberResult struct {
	BlockNumber    int64
	HealthStatus   HealthStatus
	ResponseStatus int
}

// Block lag calculation constants
const (
	// BlockLagCalculationLookback is the number of blocks to look back when calculating block time
	// Using 1024 blocks provides accurate measurement and makes reorgs/delays a rounding error
	BlockLagCalculationLookback = 1024

	// MinBlockLagLimit is the minimum block lag limit
	MinBlockLagLimit = 5
)

// CalculateDynamicBlockLagLimit calculates and sets the optimal block lag limit
// based on immutable blockchain data (block timestamps).
// This produces deterministic results across all server instances since it uses
// historical block timestamps that are baked into the blockchain.
func (n *Network) CalculateDynamicBlockLagLimit() {
	n.Logger.Debug("Starting dynamic block lag limit calculation",
		zap.String("network", n.Name),
		zap.Int("provider_count", len(n.Providers)))

	// Check if handler supports dynamic block lag using capability interface
	if n.Handler == nil || !networklib.SupportsDynamicBlockLag(n.Handler) {
		n.Logger.Debug("Handler does not support dynamic block lag, keeping default",
			zap.String("network", n.Name),
			zap.Int64("default_limit", atomic.LoadInt64(&n.BlockLagLimit)))
		return
	}

	// Check if we have providers
	if len(n.Providers) == 0 {
		n.Logger.Warn("No providers available for dynamic block lag calculation, keeping default",
			zap.String("network", n.Name),
			zap.Int64("default_limit", atomic.LoadInt64(&n.BlockLagLimit)))
		return
	}

	// Get first available provider - if timestamps disagree across providers, we have bigger problems
	var firstProvider *Provider
	var providerName string
	for name, p := range n.Providers {
		firstProvider = p
		providerName = name
		break
	}

	if firstProvider == nil {
		n.Logger.Warn("No providers available for block time measurement",
			zap.String("network", n.Name),
			zap.Int64("default_limit", atomic.LoadInt64(&n.BlockLagLimit)))
		return
	}

	avgBlockTimeMs, err := n.MeasureBlockTimeFromTimestamps(firstProvider)
	if err != nil {
		n.Logger.Warn("Failed to measure block time, keeping default",
			zap.String("network", n.Name),
			zap.String("provider", providerName),
			zap.Int64("default_limit", atomic.LoadInt64(&n.BlockLagLimit)),
			zap.Error(err))
		return
	}

	n.Logger.Debug("Successfully measured block time",
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
	newLimit = RoundUpToInterval(newLimit, 5)

	// Update the block lag limit atomically
	oldLimit := atomic.SwapInt64(&n.BlockLagLimit, newLimit)

	n.Logger.Info("Dynamic block lag limit calculated and applied",
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
func (n *Network) MeasureBlockTimeFromTimestamps(p *Provider) (float64, error) {
	// Get the dynamic block lag support capability
	// This function is only called after verifying the handler supports dynamic block lag
	dblSupport, ok := n.Handler.(networklib.DynamicBlockLagSupport)
	if !ok {
		return 0, errors.New("handler does not support dynamic block lag")
	}

	// Get the latest block number
	latestResult, err := n.GetLatestBlockNumber(
		p.HttpUrl,
		p.Headers,
		p.AuthClient(),
		p.Host,
	)
	if err != nil {
		return 0, err
	}
	latestBlock := latestResult.BlockNumber

	// Ensure we have enough blocks for the lookback
	if latestBlock < BlockLagCalculationLookback {
		n.Logger.Debug("Chain too young for block lag calculation",
			zap.String("network", n.Name),
			zap.Int64("latest_block", latestBlock),
			zap.Int("required_lookback", BlockLagCalculationLookback))
		return 0, nil
	}

	oldBlock := latestBlock - BlockLagCalculationLookback

	// Get timestamp for the latest block using capability interface
	latestTimestamp, err := dblSupport.GetBlockTimestamp(
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
	oldTimestamp, err := dblSupport.GetBlockTimestamp(
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
		n.Logger.Warn("Invalid timestamp difference",
			zap.String("network", n.Name),
			zap.Int64("latest_block", latestBlock),
			zap.Int64("old_block", oldBlock),
			zap.Int64("latest_timestamp", latestTimestamp),
			zap.Int64("old_timestamp", oldTimestamp))
		return 0, nil
	}

	// Calculate average block time in milliseconds
	avgBlockTimeMs := (float64(timeDiffSeconds) * 1000) / float64(BlockLagCalculationLookback)

	n.Logger.Debug("Block time calculation details",
		zap.String("network", n.Name),
		zap.Int64("latest_block", latestBlock),
		zap.Int64("old_block", oldBlock),
		zap.Int64("time_diff_seconds", timeDiffSeconds),
		zap.Float64("avg_block_time_ms", avgBlockTimeMs))

	return avgBlockTimeMs, nil
}

// RoundUpToInterval rounds a value up to the nearest multiple of the interval
func RoundUpToInterval(value int64, interval int64) int64 {
	if value%interval == 0 {
		return value
	}
	return ((value / interval) + 1) * interval
}

// checkBlockLag checks if the provider is lagging behind the network
// Returns: isLagged, blockLag, blockLagLimit
func (n *Network) checkBlockLag(provider *Provider, currentBlock int64, latestNetworkBlock int64) (bool, int64, int64) {
	if latestNetworkBlock <= 0 {
		return false, 0, 0
	}

	// Get block lag limit atomically
	blockLagLimit := atomic.LoadInt64(&n.BlockLagLimit)
	blockLag := int64(latestNetworkBlock) - currentBlock

	// If block lag is greater than limit, mark as warning and set isLagged flag
	if blockLag > blockLagLimit {
		n.logProviderWarning("Provider is lagging behind network", provider,
			zap.Int64("block_lag_limit", blockLagLimit),
			zap.Int64("block_lag", blockLag),
			zap.Int64("provider_block", currentBlock),
			zap.Int64("network_block", latestNetworkBlock),
			zap.String("health_status", HealthStatusString(Warning)))
		return true, blockLag, blockLagLimit
	}

	return false, blockLag, blockLagLimit
}

// isStalled checks if provider's block numbers haven't changed
func (n *Network) IsStalled(provider *Provider) bool {
	history := provider.GetBlockHistory()
	// if history is less than the block history size, return false
	if len(history) < n.ProviderBlockHistorySize {
		return false
	}

	// if all blocks in history are the same, return true
	firstBlock := history[0].BlockNumber
	for _, entry := range history[1:] {
		if entry.BlockNumber != firstBlock {
			return false
		}
	}
	return true
}

// allProvidersStalled checks if all providers are showing no progress
func (n *Network) AllProvidersStalled() bool {
	for _, p := range n.Providers {
		if !n.IsStalled(p) {
			return false
		}
	}
	return true
}

// getLatestHealthyBlock returns the highest block number among healthy providers
// Falls back to warning providers if no healthy providers are available
func (n *Network) GetLatestHealthyBlock() int64 {
	var latestBlockFromHealthy int64
	var latestBlockFromWarning int64
	var latestBlockFromUnhealthy int64

	// Single pass through providers to track latest blocks by status
	for _, provider := range n.Providers {
		history := provider.GetBlockHistory()
		if len(history) == 0 {
			continue
		}

		latestBlock := provider.GetLatestBlockEntry()
		if latestBlock == nil {
			continue
		}
		switch latestBlock.HealthStatus {
		case Healthy:
			if latestBlock.BlockNumber > latestBlockFromHealthy {
				latestBlockFromHealthy = latestBlock.BlockNumber
			}
		case Warning:
			if latestBlock.BlockNumber > latestBlockFromWarning {
				latestBlockFromWarning = latestBlock.BlockNumber
			}
		case Unhealthy:
			if latestBlock.BlockNumber > latestBlockFromUnhealthy {
				latestBlockFromUnhealthy = latestBlock.BlockNumber
			}
		}
	}

	// Return highest block number, prioritizing by health status
	if latestBlockFromHealthy > 0 {
		return latestBlockFromHealthy
	}
	return latestBlockFromWarning
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
func (n *Network) HasOtherHealthyProviders(provider *Provider) bool {
	for _, p := range n.Providers {
		if p != provider {
			entry := p.GetLatestHealthyBlockEntry()
			if entry != nil {
				return true
			}
		}
	}
	return false
}

// processBlockNumberResponse processes the response from a block number request
func (n *Network) ProcessBlockNumberResponse(resBytes []byte, statusCode *int) (int64, HealthStatus, error) {
	// Validate input
	if statusCode == nil {
		return 0, Unhealthy, errors.New("received nil statusCode in processBlockNumberResponse")
	}

	// Use BlockNumberParser capability interface
	parser, ok := n.Handler.(networklib.BlockNumberParser)
	if !ok {
		return 0, Unhealthy, errors.New("handler does not implement BlockNumberParser")
	}

	blockNumber, err := parser.ParseBlockNumberResponse(resBytes, *statusCode)
	if err != nil {
		// Determine health status based on error type
		if *statusCode == 429 {
			return 0, Warning, err
		}
		return 0, Unhealthy, err
	}

	return blockNumber, Healthy, nil
}

// AddNetworkBlockEntry adds a new block entry to the network's history,
// maintaining the configured history size. It is concurrency-safe.
func (n *Network) AddNetworkBlockEntry(blockNumber int64, blockData interface{}) {
	if n == nil { // Guard against nil network pointer
		// Cannot use n.Logger here. Consider a global logger for such rare cases if necessary.
		return
	}

	// blockData is allowed to be nil. The check for blockNumber is important.
	if blockNumber <= 0 {
		n.Logger.Error("Invalid block number in AddNetworkBlockEntry", zap.Int64("blockNumber", blockNumber), zap.Any("blockData", blockData))
		return
	}

	n.BlockHistoryMu.Lock()
	defer n.BlockHistoryMu.Unlock()

	if n.BlockHistory == nil {
		n.BlockHistory = list.New()
		if n.BlockHistory == nil { // Should not happen with list.New(), but defensive
			n.Logger.Error("Failed to initialize network block history list")
			return
		}
	}

	// Check if the new block number is greater than the latest entry
	if n.BlockHistory.Len() > 0 {
		latestEntryValue := n.BlockHistory.Back().Value
		latestEntry, ok := latestEntryValue.(BlockHistoryEntry)
		if !ok {
			n.Logger.Error("Invalid type in network block history during add check, skipping addition")
			return // Or handle error appropriately, e.g., clear history if corrupted
		}
		if blockNumber <= latestEntry.BlockNumber {
			n.Logger.Debug("New block number is not greater than the latest, skipping addition",
				zap.Int64("new_block", blockNumber),
				zap.Int64("latest_block", latestEntry.BlockNumber),
				zap.String("network", n.Name),
			)
			return
		}
	}

	// Extract block hash from blockData using BlockFetcher capability
	blockHash := ""
	if blockData != nil {
		switch data := blockData.(type) {
		case string:
			blockHash = data
		default:
			// Use BlockFetcher capability to extract block hash if available
			if blockFetcher, ok := n.Handler.(networklib.BlockFetcher); ok {
				blockHash = blockFetcher.ExtractBlockHash(blockData)
			} else {
				// If BlockFetcher not supported, log warning but continue
				n.Logger.Warn("Handler does not support BlockFetcher capability",
					zap.String("dataType", reflect.TypeOf(blockData).String()),
					zap.String("network", n.Name))
			}
		}
	}

	now := time.Now()
	entry := BlockHistoryEntry{
		BlockNumber: blockNumber,
		BlockHash:   blockHash,
		Timestamp:   &now,
	}

	n.BlockHistory.PushBack(entry)

	// Trim the list if it exceeds the history size
	for n.BlockHistory.Len() > n.NetworkBlockHistorySize {
		if n.BlockHistory.Front() != nil {
			n.BlockHistory.Remove(n.BlockHistory.Front())
		} else {
			break
		}
	}
}

// GetLatestNetworkBlockEntry returns the most recent block history entry for the network.
// It is concurrency-safe.
func (n *Network) GetLatestNetworkBlockEntry() *BlockHistoryEntry {
	if n == nil {
		return nil
	}
	n.BlockHistoryMu.RLock()
	defer n.BlockHistoryMu.RUnlock()

	// Keep critical nil checks
	if n.BlockHistory == nil {
		return nil
	}

	if n.BlockHistory.Len() == 0 {
		return nil
	}

	entry, ok := n.BlockHistory.Back().Value.(BlockHistoryEntry)
	if !ok {
		// This should ideally not happen if entries are always BlockHistoryEntry
		n.Logger.Error("Invalid type in network block history")
		return nil
	}
	return &entry
}

// GetBlockByNumber returns the block for a given block number
func (n *Network) GetBlockByNumber(blockNumber int64) (interface{}, error) {
	n.Logger.Debug("GetBlockByNumber called", zap.Int64("blockNumber", blockNumber), zap.String("networkName", n.Name))

	if n.CaddyPort == "" {
		return nil, errors.New("Caddy port is not set")
	}

	// Check if handler supports BlockFetcher capability
	blockFetcher, ok := n.Handler.(networklib.BlockFetcher)
	if !ok || !blockFetcher.SupportsGetBlockByNumber() {
		n.Logger.Debug("Network doesn't support getBlockByNumber", zap.String("networkName", n.Name))
		return nil, nil
	}

	// Check if HttpClient is available
	if n.HttpClient == nil {
		return nil, errors.New("HTTP client is not set")
	}

	// Use loopback URL to make request through the Din middleware
	url := "http://127.0.0.1:" + n.CaddyPort + "/" + n.Name
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	// Get the block by number method directly from the BlockFetcher capability
	getBlockMethod := blockFetcher.GetBlockByNumberMethod()
	if getBlockMethod == "" {
		return nil, errors.New("handler does not provide a getBlockByNumber method")
	}

	// Use BlockFetcher to create the block request payload
	payload, err := blockFetcher.CreateBlockRequest(getBlockMethod, blockNumber, false)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "Failed to create block request")
	}

	n.Logger.Debug("Sending getBlockByNumber request", zap.String("url", url), zap.Any("headers", headers), zap.String("payload", string(payload)))

	resBytes, statusCode, err := n.HttpClient.Post(url, headers, payload, nil)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "Error sending POST request")
	}

	responseSnippet := string(resBytes)
	if len(resBytes) > 200 {
		responseSnippet = string(resBytes[:200])
	}
	n.Logger.Debug("Received getBlockByNumber response", zap.Int("statusCode", *statusCode), zap.String("responseBodySnippet", responseSnippet))

	if *statusCode != http.StatusOK {
		n.Logger.Warn("Error getting block from response, non-OK status", zap.Int("statusCode", *statusCode), zap.String("networkName", n.Name))
		return nil, errors.New("Error getting block from response")
	}

	// Use BlockFetcher to parse the block response
	blockData, err := blockFetcher.ParseBlockResponse(resBytes)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "Error parsing block response")
	}

	n.Logger.Debug("Successfully parsed block response", zap.String("networkName", n.Name))
	return blockData, nil
}
