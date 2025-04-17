package reputationscore

const (
	MetricDefaultInterval                 = "5min"
	HighLatencyInMillis                   = 1000.0 // 1 second
	LowLatencyInMillis                    = 50.0   // 50ms
	MaxAcceptableRequestSuccessPercentage = 99.5   // If less than [threshold] of requests are successful, Metric is zero
	BlockNumberConsistencyMetricID        = "blockNumberConsistency"
	BlockNonStateConsistencyMetricID      = "blockNonStateConsistency"
	LatencyMetricID                       = "latency"
	ScoreSmoothingFactor                  = 0.5
	BlockNumberConsistencyWeight          = 0.5
	BlockNonStateConsistencyWeight        = 0.3
	LatencyWeight                         = 0.2
)
