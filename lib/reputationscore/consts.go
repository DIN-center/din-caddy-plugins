package reputationscore

const (
	MetricDefaultInterval                 = "5min"
	HighLatencyInMillis                   = 1000.0 // 1 second
	LowLatencyInMillis                    = 50.0   // 50ms
	MaxAcceptableRequestSuccessPercentage = 99.5   // If less than [threshold] of requests are successful, Metric is zero
	BlockNumberConsistencyMetricID        = "block_number_consistency"
	BlockNonStateConsistencyMetricID      = "block_non_state_consistency"
	LatencyMetricID                       = "latency"
	ScoreSmoothingFactor                  = 0.5
	BlockNumberConsistencyWeight          = 0.3
	BlockNonStateConsistencyWeight        = 0.2
	LatencyWeight                         = 0.5
)
