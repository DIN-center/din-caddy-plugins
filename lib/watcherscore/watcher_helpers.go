package watcherscore

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
	var rank float64
	if latencyStats.P95 <= LowLatencyInMillis {
		// Perfect rank for latencies under 50ms
		rank = 1.0
	} else if latencyStats.P95 >= HighLatencyInMillis {
		// Zero rank for latencies over 1000ms
		rank = 0.0
	} else {

		normalizedLatency := (latencyStats.P95 - LowLatencyInMillis) / (HighLatencyInMillis - LowLatencyInMillis)
		// We invert it since we want higher latencies to have lower scores
		rank = 1.0 - NormalizedExponentialFunction(normalizedLatency, 1.0)
		// Visualization of the latency ranking function (k=1):
		// Rank
		// 1.0 |  ****
		//     |      ****
		//     |          ***
		// 0.5 |             ***
		//     |                **
		//     |                  **
		//     |                    **
		// 0.0 |                      ****
		//     +--------------------------------
		//     0   200  400  600  800  1000 1200ms
		//     |    Low                High   |
		//     |  (50ms)             (1000ms) |
	}
	// Round to 4 decimal places
	return math.Max(0, math.Min(1, math.Round(rank*10000)/10000))
}

func buildMetricsForCheckQuery(client watcher.IWatcherAPIClient, params watcher.CheckQueryParams, metricID string, logger *zap.Logger) ([]*ProviderMetric, error) {
	// Query watcher API for check
	logger.Debug("[WATCHER_SCORE] Querying watcher API for check", zap.Any("params", params))

	response := client.GetCheck(params)
	if response.IsErr() {
		logger.Error("[WATCHER_SCORE] Error getting check data", zap.Error(response.UnwrapErr()))
		return nil, errors.Wrapf(response.UnwrapErr(), "Error while querying check %s for network %s", params.CheckID, params.Network)
	}

	// Convert check response to metrics
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
			params.Network,
			provider.Provider,
			provider.ProviderID,
			provider.EndpointURL,
			metricValue,
			lastUpdated,
		)
		logger.Info("[WATCHER_SCORE] Watcher check response",
			zap.String("cycleStart", checkResponse.CycleStart),
			zap.String("cycleEnd", checkResponse.CycleEnd),
			zap.String("metricID", metricID),
			zap.String("network", metric.Network()),
			zap.String("provider", metric.ProviderName()),
			zap.String("providerID", metric.ProviderID()),
			zap.Float64("metricValue", metric.Value()),
			zap.Time("lastUpdated", metric.LastUpdated()),
			zap.Any("responseStatus", provider.ResponseStatus),
			zap.Any("checkSummary", provider.CheckSummary))
		metrics = append(metrics, metric)
	}
	return metrics, nil
}

func buildMetricsForLatencyQuery(client watcher.IWatcherAPIClient, params watcher.LatencyQueryParams, metricID string, logger *zap.Logger) ([]*ProviderMetric, error) {
	// Query watcher API for latency
	logger.Debug("[WATCHER_SCORE] Querying watcher API for latency", zap.Any("params", params))
	response := client.GetLatency(params)
	if response.IsErr() {
		logger.Error("[WATCHER_SCORE] Error getting latency data", zap.Error(response.UnwrapErr()))
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
			params.Network,
			provider.Provider,
			provider.ProviderID,
			provider.EndpointURL,
			metricValue,
			lastUpdated,
		)
		logger.Info("[WATCHER_SCORE] Watcher latency response",
			zap.String("cycleStart", latencyResponse.CycleStart),
			zap.String("cycleEnd", latencyResponse.CycleEnd),
			zap.String("metricID", metricID),
			zap.String("network", metric.Network()),
			zap.String("provider", metric.ProviderName()),
			zap.String("providerID", metric.ProviderID()),
			zap.Float64("metricValue", metric.Value()),
			zap.Time("lastUpdated", metric.LastUpdated()),
			zap.Any("responseStatus", provider.ResponseStatus),
			zap.Any("latencySummary", provider.Latency))
		metrics = append(metrics, metric)
	}

	return metrics, nil
}
