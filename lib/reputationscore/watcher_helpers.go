package reputationscore

import (
	"math"
	"time"

	"github.com/DIN-center/din-sc/apps/din-go/lib/watcher"

	"github.com/pkg/errors"
)

// Calculate check metric to a metric scaled between 0 and 1
// Higher pass percentage = higher rank
func calculateCheckMetric(responseStatus watcher.Status, checkSummary watcher.Summary) float64 {
	// If less than [MaxAcceptableRequestSuccessPercentage] of requests are successful, Metric is zero
	if responseStatus.SuccessPercentage < MaxAcceptableRequestSuccessPercentage {
		return 0
	}
	rank := checkSummary.PassPercentage / 100.0
	return math.Max(0, math.Min(1, rank))
}

// Calculate latency metric scaled between 0 and 1
// Lower latency = higher rank
func calculateLatencyMetric(responseStatus watcher.Status, latencyStats watcher.LatencyStats) float64 {
	// If less than [MaxAcceptableRequestSuccessPercentage] of requests are successful, Metric is zero
	if responseStatus.SuccessPercentage < MaxAcceptableRequestSuccessPercentage {
		return 0
	}

	// Convert P95 to a rank between 0 and 1
	// Lower latency = higher rank
	// Using maxAcceptableLatency as the baseline for normalization
	rank := 1 - (latencyStats.P95 / MaxAcceptableLatencyInMilliseconds)
	return math.Max(0, math.Min(1, rank))

}

func buildMetricsForCheckQuery(client watcher.IWatcherAPIClient, params watcher.CheckQueryParams, metricID string) ([]*ProviderMetric, error) {
	// Query watcher API for check
	response := client.GetCheck(params)
	if response.IsErr() {
		return nil, errors.Wrapf(response.UnwrapErr(), "Error while querying check %s for network %s", params.CheckID, params.Network)
	}

	// Transform check response to metrics
	checkResponse := response.Unwrap()
	metrics := []*ProviderMetric{}
	for _, provider := range checkResponse.Providers {
		metricValue := calculateCheckMetric(provider.ResponseStatus, provider.CheckSummary)

		// Parse timestamp, but verify that it's not empty
		if provider.LatestCheckTimestamp == "" {
			return nil, errors.New("Field 'latest_check_ts' is empty")
		}
		lastUpdated, err := time.Parse(time.RFC3339Nano, provider.LatestCheckTimestamp)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse LastCheckTimestamp")
		}
		metric, _ := NewProviderMetric(
			metricID,
			provider.Provider,
			metricValue,
			lastUpdated,
		)

		metrics = append(metrics, metric)
	}
	return metrics, nil
}

func buildMetricsForLatencyQuery(client watcher.IWatcherAPIClient, params watcher.LatencyQueryParams, metricID string) ([]*ProviderMetric, error) {
	// Query watcher API for latency
	response := client.GetLatency(params)
	if response.IsErr() {
		return nil, errors.Wrapf(response.UnwrapErr(), "Error getting latency data for network %s", params.Network)
	}
	latencyResponse := response.Unwrap()

	// Transform latency response to metrics
	metrics := []*ProviderMetric{}
	for _, provider := range latencyResponse.Providers {
		metricValue := calculateLatencyMetric(provider.ResponseStatus, provider.Latency)

		// Parse timestamp, but verify that it's not empty
		if provider.LastRequestTimestamp == "" {
			return nil, errors.New("Field 'last_request_ts' is empty")
		}
		lastUpdated, err := time.Parse(time.RFC3339Nano, provider.LastRequestTimestamp)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse LastRequestTimestamp")
		}
		metric, _ := NewProviderMetric(
			metricID,
			provider.Provider,
			metricValue,
			lastUpdated,
		)
		metrics = append(metrics, metric)
	}

	return metrics, nil
}
