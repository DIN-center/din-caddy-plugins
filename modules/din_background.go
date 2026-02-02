package modules

import (
	"time"

	"go.uber.org/zap"

	"github.com/DIN-center/din-caddy-plugins/lib/web3"
)

// startHealthChecks starts a background goroutine to monitor all of the networks' overall health and the health of its providers
func (d *DinMiddleware) startHealthChecks() error {
	d.logger.Info("Starting healthchecks")
	for _, network := range d.Networks {
		d.logger.Info("Starting healthcheck for network", zap.String("network", network.Name))
		network.StartHealthcheck()
	}
	return nil
}

// startRegistrySync initiates a periodic synchronization process with the registry. It retrieves data from the
// registry and processes it immediately. A ticker is started to poll the latest block number from the
// Linea network at regular intervals (default 60 seconds). If the latest block number has moved beyond
// the defined block epoch, it retrieves new registry data and processes it. The function runs in a separate
// goroutine and will terminate when a quit signal is received.
func (d *DinMiddleware) startRegistrySync() {
	// Get the initial registry data with retry
	registryData, err := d.getRegistryData()
	if err != nil {
		d.logger.Error("Failed to initialize registry sync after retries",
			zap.Error(err),
			zap.Int("max_retries", d.Registry.RetryMaxAttempts))
	}
	d.processRegistryData(registryData)

	// Start a ticker to check the linea network latest block number on a time interval of 60 seconds by default.
	ticker := time.NewTicker(time.Second * time.Duration(d.Registry.BlockCheckIntervalSec))
	go func() {
		// CRITICAL: Panic recovery to prevent application crash
		// If registry sync panics, log the error and continue running
		defer func() {
			if r := recover(); r != nil {
				d.logger.Error("CRITICAL: Registry sync goroutine panicked and recovered. Application continues running.",
					zap.Any("panic", r),
					zap.Stack("stacktrace"))
				// Clean up the ticker
				ticker.Stop()

				// Restart the sync after a configurable delay to recover from transient issues
				// This prevents the sync from being permanently dead after a panic
				time.Sleep(d.Registry.PanicRecoveryDelay)
				d.logger.Info("Attempting to restart registry sync after panic recovery",
					zap.Duration("recovery_delay", d.Registry.PanicRecoveryDelay))
				d.startRegistrySync()
			}
		}()

		for {
			select {
			case <-d.quit:
				ticker.Stop()
				d.logger.Info("Registry sync goroutine shutting down gracefully")
				return
			case <-ticker.C:
				// Wrap the sync call in a function that can recover from panics
				func() {
					defer func() {
						if r := recover(); r != nil {
							d.logger.Error("Registry sync operation panicked during sync attempt",
								zap.Any("panic", r))
						}
					}()
					d.syncRegistryWithLatestBlock(web3.NewEVMClient(d.DingoClient.GetEthereumRpcClient()))
				}()
			}
		}
	}()
}

// startWatcherScoreSync starts a background goroutine to sync the watcher scores to the middleware
func (d *DinMiddleware) startWatcherScoreSync() chan struct{} {
	syncQuit := make(chan struct{})

	// Do immediate initial sync
	// Note that syncing watcher scores immediately here may be a bit early if the score computation is not yet complete,
	// but it's ok because the watcher score manager will return empty scores until the computation is complete
	// and the middleware will not use these empty scores for load balancing
	d.SyncMiddlewareWithLatestScores()

	go func() {
		ticker := time.NewTicker(time.Duration(d.DynamicLoadBalancing.WatcherScoreSyncIntervalSec) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-syncQuit:
				d.logger.Info("[DYNAMIC_LB] Watcher score sync goroutine shutting down gracefully")
				return
			case <-ticker.C:
				d.SyncMiddlewareWithLatestScores()
			}
		}
	}()
	return syncQuit
}
