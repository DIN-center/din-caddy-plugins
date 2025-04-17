package reputationscore

import (
	"github.com/DIN-center/din-sc/apps/din-go/lib/watcher"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// Implements the logic to generate block number consistency metric
// Block number consistency is a metric that checks if the block number monotonically increases over time
type WatcherBlockNumberConsistency struct {
	WatcherClient watcher.IWatcherAPIClient
	Logger        *zap.Logger
}

func (g *WatcherBlockNumberConsistency) MetricID() string {
	return BlockNumberConsistencyMetricID
}

func (g *WatcherBlockNumberConsistency) GenerateMetrics(network string) ([]*ProviderMetric, error) {

	// Query for block number consistency check
	queryParams := watcher.CheckQueryParams{
		CheckID:  BlockNumberConsistencyMetricID,
		Network:  network,
		Interval: MetricDefaultInterval,
	}

	g.Logger.Debug("[REPUTATION_SCORE] Generating metrics for block number consistency...")

	metrics, err := buildMetricsForCheckQuery(g.WatcherClient, queryParams, g.MetricID(), g.Logger)
	if err != nil {
		return nil, errors.Wrap(err, "Error while building metrics from check blockNumberConsistency")
	}
	return metrics, nil
}

// Implements the logic to generate block non-state consistency metric
// Block non-state consistency is a metric that checks if non-state data (e.g., logs, transactions)
// associated with a specific block may change over time due to chain reorganizations, node inconsistencies, or data corruption.
type WatcherBlockNonStateConsistency struct {
	WatcherClient watcher.IWatcherAPIClient
	Logger        *zap.Logger
}

func (g *WatcherBlockNonStateConsistency) MetricID() string {
	return BlockNonStateConsistencyMetricID
}

// Generate blockNonStateConsistency metrics for a given network
func (g *WatcherBlockNonStateConsistency) GenerateMetrics(network string) ([]*ProviderMetric, error) {
	// Query for block number consistency check
	queryParams := watcher.CheckQueryParams{
		CheckID:  BlockNonStateConsistencyMetricID,
		Network:  network,
		Interval: MetricDefaultInterval,
	}

	g.Logger.Debug("[REPUTATION_SCORE] Generating metrics for block non-state consistency...")

	metrics, err := buildMetricsForCheckQuery(g.WatcherClient, queryParams, g.MetricID(), g.Logger)
	if err != nil {
		return nil, errors.Wrap(err, "Error while building metrics from check blockNonStateConsistency")
	}

	return metrics, nil
}

// Implements the logic to generate latency metric
// Latency is a metric that checks the overall response time that a provider takes to respond to a JSON RPC request
type WatcherLatency struct {
	WatcherClient watcher.IWatcherAPIClient
	Logger        *zap.Logger
}

func (g *WatcherLatency) MetricID() string {
	return LatencyMetricID
}

func (g *WatcherLatency) GenerateMetrics(network string) ([]*ProviderMetric, error) {

	// Get latency data
	latencyQuery := watcher.LatencyQueryParams{
		Network:  network,
		Interval: MetricDefaultInterval,
	}

	g.Logger.Debug("[REPUTATION_SCORE] Generating metrics for latency...")

	metrics, err := buildMetricsForLatencyQuery(g.WatcherClient, latencyQuery, g.MetricID(), g.Logger)
	if err != nil {
		return nil, errors.Wrap(err, "Error while building metrics from latency")
	}

	return metrics, nil
}
