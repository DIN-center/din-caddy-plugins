package reputationscore

import (
	"math"
	"time"

	"github.com/DIN-center/din-sc/apps/din-go/lib/watcher"
	"go.uber.org/zap"

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
	// The highest score is when P95 is less or equal than 50ms, rank is 1
	// The lowest score is when P95 is greater or equal than 1000ms, rank is 0
	// Using LowLatencyInMillis as the baseline for normalization
	var rank float64
	if latencyStats.P95 <= LowLatencyInMillis {
		rank = 1.0
	} else if latencyStats.P95 >= HighLatencyInMillis {
		rank = 0.0
	} else {
		// Otherwise, logarithmic interpolation between 50ms and 1000ms
		// This makes the score decrease faster as latency approaches 1000ms
		// The value 9.0 is chosen to make the score decrease faster as latency approaches 1000ms
		// and also for convenience to have a nice range of values:
		// When normalizedLatency = 0 (best case): score = 1.0 - Log10(1) = 1.0
		// When normalizedLatency = 1 (worst case): score = 1.0 - Log10(10) = 0.0
		normalizedLatency := (latencyStats.P95 - LowLatencyInMillis) / (HighLatencyInMillis - LowLatencyInMillis)
		rank = 1.0 - math.Log10(1.0+9.0*normalizedLatency)
	}
	// Round to 4 decimal places
	return math.Max(0, math.Min(1, math.Round(rank*10000)/10000))

}

func buildMetricsForCheckQuery(client watcher.IWatcherAPIClient, params watcher.CheckQueryParams, metricID string, logger *zap.Logger) ([]*ProviderMetric, error) {
	// Query watcher API for check
	logger.Debug("[REPUTATION_SCORE] Querying watcher API for check", zap.Any("params", params))

	response := client.GetCheck(params)
	if response.IsErr() {
		logger.Error("[REPUTATION_SCORE] Error getting check data", zap.Error(response.UnwrapErr()))
		return nil, errors.Wrapf(response.UnwrapErr(), "Error while querying check %s for network %s", params.CheckID, params.Network)
	}

	// Convert check response to metrics
	checkResponse := response.Unwrap()
	metrics := []*ProviderMetric{}
	for _, provider := range checkResponse.Providers {
		logger.Info("[REPUTATION_SCORE] Watcher check response",
			zap.String("cycleStart", checkResponse.CycleStart),
			zap.String("cycleEnd", checkResponse.CycleEnd),
			zap.String("checkID", params.CheckID),
			zap.String("network", params.Network),
			zap.String("provider", provider.Provider),
			zap.Any("responseStatus", provider.ResponseStatus),
			zap.Any("checkSummary", provider.CheckSummary))
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
			params.Network,
			provider.Provider,
			provider.EndpointURL,
			metricValue,
			lastUpdated,
		)
		logger.Debug("[REPUTATION_SCORE] Check metric built", zap.Any("metric", metric))
		metrics = append(metrics, metric)
	}
	return metrics, nil
}

func buildMetricsForLatencyQuery(client watcher.IWatcherAPIClient, params watcher.LatencyQueryParams, metricID string, logger *zap.Logger) ([]*ProviderMetric, error) {
	// Query watcher API for latency
	logger.Debug("[REPUTATION_SCORE] Querying watcher API for latency", zap.Any("params", params))
	response := client.GetLatency(params)
	if response.IsErr() {
		logger.Error("[REPUTATION_SCORE] Error getting latency data", zap.Error(response.UnwrapErr()))
		return nil, errors.Wrapf(response.UnwrapErr(), "Error getting latency data for network %s", params.Network)
	}
	latencyResponse := response.Unwrap()

	// Transform latency response to metrics
	metrics := []*ProviderMetric{}
	for _, provider := range latencyResponse.Providers {
		logger.Info("[REPUTATION_SCORE] Watcher latency response",
			zap.String("cycleStart", latencyResponse.CycleStart),
			zap.String("cycleEnd", latencyResponse.CycleEnd),
			zap.String("network", params.Network),
			zap.String("provider", provider.Provider),
			zap.Any("responseStatus", provider.ResponseStatus),
			zap.Any("latencySummary", provider.Latency))
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
			params.Network,
			provider.Provider,
			provider.EndpointURL,
			metricValue,
			lastUpdated,
		)
		logger.Debug("[REPUTATION_SCORE] Latency metric built", zap.Any("metric", metric))
		metrics = append(metrics, metric)
	}

	return metrics, nil
}
