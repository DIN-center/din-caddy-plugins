package modules

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// calculateDynamicBlockLagLimit calculates and sets the optimal block lag limit
// based on the actual block production rate of the fastest provider on the network.
// This function runs once asynchronously when the network starts health checking.
// It measures all providers in parallel over the specified duration for accurate comparison.
func (n *network) calculateDynamicBlockLagLimit(measurementSeconds int) {
	n.logger.Debug("Starting dynamic block lag limit calculation",
		zap.String("network", n.Name),
		zap.Int("provider_count", len(n.Providers)),
		zap.Int("measurement_seconds", measurementSeconds))

	// Check if we have providers
	if len(n.Providers) == 0 {
		n.logger.Warn("No providers available for dynamic block lag calculation, keeping default",
			zap.String("network", n.Name),
			zap.Int64("default_limit", n.BlockLagLimit))
		return
	}

	// Structure to hold provider measurement results
	type providerMeasurement struct {
		name         string
		initialBlock int64
		finalBlock   int64
		msPerBlock   float64
		err          error
	}

	// Channel to collect results and WaitGroup for synchronization
	results := make(chan providerMeasurement, len(n.Providers))
	var wg sync.WaitGroup

	// Launch parallel measurement for each provider
	for providerName, providerPtr := range n.Providers {
		wg.Add(1)
		go func(name string, p *provider, measureSecs int) {
			defer wg.Done()
			
			measurement := providerMeasurement{name: name}
			
			// Get initial block number
			startTime := time.Now()
			initialResult, err := n.getLatestBlockNumber(
				p.HttpUrl,
				p.Headers,
				p.AuthClient(),
				p.host,
			)
			if err != nil {
				measurement.err = err
				results <- measurement
				n.logger.Debug("Failed to get initial block for provider",
					zap.String("network", n.Name),
					zap.String("provider", name),
					zap.Error(err))
				return
			}
			measurement.initialBlock = initialResult.blockNumber
			n.logger.Debug("Initial block recorded",
				zap.String("network", n.Name),
				zap.String("provider", name),
				zap.Int64("block", measurement.initialBlock))
			
			// Calculate exact sleep time to ensure measurement duration total
			elapsed := time.Since(startTime)
			measurementDuration := time.Duration(measureSecs) * time.Second
			sleepDuration := measurementDuration - elapsed
			if sleepDuration > 0 {
				time.Sleep(sleepDuration)
			}
			
			// Get final block number (exactly after measurement duration)
			finalResult, err := n.getLatestBlockNumber(
				p.HttpUrl,
				p.Headers,
				p.AuthClient(),
				p.host,
			)
			if err != nil {
				measurement.err = err
				results <- measurement
				n.logger.Debug("Failed to get final block for provider",
					zap.String("network", n.Name),
					zap.String("provider", name),
					zap.Error(err))
				return
			}
			measurement.finalBlock = finalResult.blockNumber
			
			// Calculate block difference
			blockDiff := measurement.finalBlock - measurement.initialBlock
			if blockDiff <= 0 {
				measurement.err = fmt.Errorf("no new blocks produced")
				n.logger.Debug("Provider produced no new blocks",
					zap.String("network", n.Name),
					zap.String("provider", name),
					zap.Int64("initial_block", measurement.initialBlock),
					zap.Int64("final_block", measurement.finalBlock))
			} else {
				// Calculate milliseconds per block
				measurementMs := float64(measureSecs * 1000)
				measurement.msPerBlock = measurementMs / float64(blockDiff)
				n.logger.Debug("Provider block rate calculated",
					zap.String("network", n.Name),
					zap.String("provider", name),
					zap.Int64("blocks_produced", blockDiff),
					zap.Float64("ms_per_block", measurement.msPerBlock),
					zap.Int("measurement_seconds", measureSecs))
			}
			
			results <- measurement
		}(providerName, providerPtr, measurementSeconds)
	}

	// Wait for all measurements to complete
	wg.Wait()
	close(results)

	// Process results to find the fastest provider
	var fastestRateMs float64 = math.MaxFloat64
	var fastestProvider string
	successfulMeasurements := 0
	providerRates := make(map[string]float64)

	for result := range results {
		if result.err == nil && result.msPerBlock > 0 {
			successfulMeasurements++
			providerRates[result.name] = result.msPerBlock
			if result.msPerBlock < fastestRateMs {
				fastestRateMs = result.msPerBlock
				fastestProvider = result.name
			}
		}
	}

	// Step 4: Calculate and apply the new block lag limit
	if fastestRateMs < math.MaxFloat64 && fastestRateMs > 0 {
		// Calculate how many blocks fit in the default period
		newLimit := int64(math.Ceil(float64(DefaultBlockLagPeriodMs) / fastestRateMs))
		
		// Round up to nearest interval of 5
		newLimit = roundUpToInterval(newLimit, 5)

		// Update the block lag limit atomically
		oldLimit := atomic.SwapInt64(&n.BlockLagLimit, newLimit)

		n.logger.Info("Dynamic block lag limit calculated and applied",
			zap.String("network", n.Name),
			zap.Int64("old_limit", oldLimit),
			zap.Int64("new_limit", newLimit),
			zap.String("fastest_provider", fastestProvider),
			zap.Float64("fastest_rate_ms_per_block", fastestRateMs),
			zap.Int("providers_measured", len(providerRates)))
	} else {
		n.logger.Warn("Unable to calculate dynamic block lag limit, keeping default",
			zap.String("network", n.Name),
			zap.Int64("default_limit", atomic.LoadInt64(&n.BlockLagLimit)),
			zap.Int("providers_checked", len(n.Providers)),
			zap.Int("successful_measurements", successfulMeasurements))
	}
}

// roundUpToInterval rounds a value up to the nearest multiple of the interval
func roundUpToInterval(value int64, interval int64) int64 {
	if value%interval == 0 {
		return value
	}
	return ((value / interval) + 1) * interval
}