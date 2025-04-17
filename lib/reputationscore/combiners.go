package reputationscore

import (
	"fmt"
	"math"
	"time"

	"go.uber.org/zap"
)

// A weighted combiner is a base implementation of combiner that uses weights to aggregate metrics.
type WeightedCombiner struct {
	weights map[string]float64
	logger  *zap.Logger
}

func NewWeightedCombiner(weights map[string]float64, logger *zap.Logger) (*WeightedCombiner, error) {
	if len(weights) == 0 {
		return nil, fmt.Errorf("weights map cannot be empty")
	}

	totalWeight := 0.0
	for _, weight := range weights {
		totalWeight += weight
	}

	if totalWeight != 1.0 {
		return nil, fmt.Errorf("weights must sum to 1.0, got %f", totalWeight)
	}

	return &WeightedCombiner{weights: weights, logger: logger}, nil
}

func (c *WeightedCombiner) CombineMetrics(metrics []*ProviderMetric) (*Score, error) {
	if len(metrics) == 0 {
		return NewEmptyScore(), fmt.Errorf("no metrics to combine")
	}

	if len(c.weights) != len(metrics) {
		return NewEmptyScore(), fmt.Errorf("weights map must have the same length as the number of metrics")
	}

	weightedValue := 0.0
	for _, metric := range metrics {
		if weight, exists := c.weights[metric.MetricID()]; exists {

			metricWeightedValue := metric.Value() * weight
			c.logger.Info("[REPUTATION_SCORE] Combining metric (weighted)",
				zap.String("metricID", metric.MetricID()),
				zap.String("network", metric.Network()),
				zap.String("provider", metric.ProviderID()),
				zap.Float64("weight", weight),
				zap.Float64("value", metric.Value()),
				zap.Float64("weightedValue", metricWeightedValue),
				zap.Float64("fullnessRatio", func() float64 {
					if weight == 0 {
						return 0
					}
					return math.Round((metricWeightedValue/weight)*10000) / 10000
				}()))
			weightedValue += metricWeightedValue
		} else {
			return NewEmptyScore(), fmt.Errorf("no weight for metric %s", metric.MetricID())
		}
	}

	// Find the most recent metric timestamp
	maxLastUpdated := time.Time{}
	for _, metric := range metrics {
		if metric.LastUpdated().After(maxLastUpdated) {
			maxLastUpdated = metric.LastUpdated()
		}
	}

	return NewScore(weightedValue, maxLastUpdated)
}
